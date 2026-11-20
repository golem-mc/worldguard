package worldguardplugin

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/internal/chat"
)

// messageThreshold is the least time between two entry or exit refusals'
// messages. WorldGuard: EntryFlag.MESSAGE_THRESHOLD
const messageThreshold = 2 * time.Second

// valueHooks is what a flag value change handler does as the flag's value
// changes. WorldGuard: FlagValueChangeHandler
type valueHooks[T comparable] interface {
	onInitial(s *session, p *localPlayer, set worldguard.ApplicableRegionSet, v T, ok bool)
	onSet(s *session, p *localPlayer, from, to plugin.Location, toSet worldguard.ApplicableRegionSet,
		cur, last T, hadLast bool, mt moveType) bool
	onAbsent(s *session, p *localPlayer, from, to plugin.Location, toSet worldguard.ApplicableRegionSet,
		last T, mt moveType) bool
}

// keepOnClear marks hooks whose last value stays applied when the session
// ends. WorldGuard: NotifyExitFlag.onClearValue
type keepOnClear interface{ keepOnClear() }

// valueChange calls its hooks when a flag's value differs between the
// regions a player leaves and enters. WorldGuard: FlagValueChangeHandler
type valueChange[T comparable] struct {
	baseHandler
	flag    worldguard.Flag
	hooks   valueHooks[T]
	last    T
	hasLast bool
}

func newValueChange[T comparable](s *session, f worldguard.Flag, hooks valueHooks[T]) *valueChange[T] {
	return &valueChange[T]{baseHandler: baseHandler{s}, flag: f, hooks: hooks}
}

func (h *valueChange[T]) initialize(p *localPlayer, _ plugin.Location, set worldguard.ApplicableRegionSet) {
	h.last, h.hasLast = worldguard.Value[T](set, p, h.flag)
	h.hooks.onInitial(h.s, p, set, h.last, h.hasLast)
}

func (h *valueChange[T]) uninitialize(p *localPlayer, _ plugin.Location, set worldguard.ApplicableRegionSet) {
	if _, keep := h.hooks.(keepOnClear); !keep && h.hasLast {
		cur := p.Location()
		h.hooks.onAbsent(h.s, p, cur, cur, set, h.last, moveOtherNonCancellable)
	}
	var zero T
	h.last, h.hasLast = zero, false
}

func (h *valueChange[T]) onCrossBoundary(p *localPlayer, from, to plugin.Location, toSet worldguard.ApplicableRegionSet,
	entered, exited regionSet, mt moveType) bool {
	// The sets leave out the global region, so a change of world counts.
	if len(entered) == 0 && len(exited) == 0 && from.World == to.World {
		return true
	}
	cur, ok := worldguard.Value[T](toSet, p, h.flag)
	allowed := true
	switch {
	case !ok && h.hasLast:
		allowed = h.hooks.onAbsent(h.s, p, from, to, toSet, h.last, mt)
	case ok && (!h.hasLast || cur != h.last):
		allowed = h.hooks.onSet(h.s, p, from, to, toSet, cur, h.last, h.hasLast, mt)
	}
	if allowed {
		h.last, h.hasLast = cur, ok
	}
	return allowed
}

// invincibilityHook is hooks that decide invincibility.
type invincibilityHook interface {
	invincibility(wg *WorldGuard, p *localPlayer) worldguard.State
}

func (h *valueChange[T]) invincibility(p *localPlayer) worldguard.State {
	if i, ok := h.hooks.(invincibilityHook); ok {
		return i.invincibility(h.s.m.wg, p)
	}
	return worldguard.StateNone
}

// entryFlag refuses moves into regions whose entry flag denies the
// player. WorldGuard: EntryFlag
type entryFlag struct {
	baseHandler
	lastMessage time.Time
}

func (h *entryFlag) onCrossBoundary(p *localPlayer, _, to plugin.Location, toSet worldguard.ApplicableRegionSet,
	_, _ regionSet, mt moveType) bool {
	if toSet.TestState(p, worldguard.FlagEntry) || !mt.cancellable || h.s.m.wg.HasBypass(p.Player, to.World) {
		return true
	}
	msg, _ := worldguard.Value[string](toSet, p, worldguard.FlagEntryDenyMessage)
	if now := time.Now(); now.Sub(h.lastMessage) > messageThreshold && msg != "" {
		sendRaw(p, worldguard.ReplaceColorMacros(msg))
		h.lastMessage = now
	}
	return false
}

// exitFlag refuses moves out of regions whose exit flag denies the
// player. WorldGuard: ExitFlag
type exitFlag struct {
	storedMessage   string
	exitViaTeleport bool
	lastMessage     time.Time
}

func (h *exitFlag) update(p *localPlayer, set worldguard.ApplicableRegionSet, allowed bool) {
	if !allowed {
		h.storedMessage, _ = worldguard.Value[string](set, p, worldguard.FlagExitDenyMessage)
		h.exitViaTeleport = set.TestState(p, worldguard.FlagExitViaTeleport)
	}
}

func (h *exitFlag) sendMessage(p *localPlayer) {
	if now := time.Now(); now.Sub(h.lastMessage) > messageThreshold && h.storedMessage != "" {
		sendRaw(p, worldguard.ReplaceColorMacros(h.storedMessage))
		h.lastMessage = now
	}
}

// overridden reports whether exit-override lets the player out.
func overridden(p *localPlayer, toSet worldguard.ApplicableRegionSet) bool {
	v, _ := worldguard.Value[bool](toSet, p, worldguard.FlagExitOverride)
	return v
}

func (h *exitFlag) onInitial(_ *session, p *localPlayer, set worldguard.ApplicableRegionSet, v worldguard.State, ok bool) {
	h.update(p, set, !ok || v != worldguard.StateDeny)
}

func (h *exitFlag) onSet(s *session, p *localPlayer, from, _ plugin.Location, toSet worldguard.ApplicableRegionSet,
	cur, last worldguard.State, hadLast bool, mt moveType) bool {
	if s.m.wg.HasBypass(p.Player, from.World) {
		return true
	}
	lastAllowed := !hadLast || last == worldguard.StateAllow
	allowed := cur == worldguard.StateAllow
	if allowed && !lastAllowed && (!mt.teleport || !h.exitViaTeleport) && mt.cancellable && !overridden(p, toSet) {
		h.sendMessage(p)
		return false
	}
	h.update(p, toSet, allowed)
	return true
}

func (h *exitFlag) onAbsent(s *session, p *localPlayer, from, _ plugin.Location, toSet worldguard.ApplicableRegionSet,
	last worldguard.State, mt moveType) bool {
	if s.m.wg.HasBypass(p.Player, from.World) {
		return true
	}
	if last != worldguard.StateAllow && mt.cancellable && !overridden(p, toSet) {
		h.sendMessage(p)
		return false
	}
	return true
}

// messages answers a string flag's values at a place, in order, without
// repeats.
func messages(p *localPlayer, set worldguard.ApplicableRegionSet, f worldguard.Flag) []string {
	var out []string
	seen := map[string]bool{}
	for _, v := range set.QueryAllValues(p, f) {
		if s, ok := v.(string); ok && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// withRegionValues adds the flag's value of every region in set to stack.
func withRegionValues(stack map[string]bool, set worldguard.ApplicableRegionSet, f worldguard.Flag) {
	for _, r := range set.Regions() {
		if s, ok := r.Flag(f).(string); ok {
			stack[s] = true
		}
	}
}

// greetingFlag shows the greeting and greeting-title of regions a player
// enters. WorldGuard: GreetingFlag
type greetingFlag struct {
	baseHandler
	lastMessages, lastTitles map[string]bool
}

func (h *greetingFlag) onCrossBoundary(p *localPlayer, _, _ plugin.Location, toSet worldguard.ApplicableRegionSet,
	_, _ regionSet, _ moveType) bool {
	h.lastMessages = h.sendAndCollect(p, toSet, worldguard.FlagGreeting, h.lastMessages, h.s.m.wg.sendChatFlag)
	h.lastTitles = h.sendAndCollect(p, toSet, worldguard.FlagGreetingTitle, h.lastTitles, h.s.m.wg.sendTitleFlag)
	return true
}

func (h *greetingFlag) sendAndCollect(p *localPlayer, toSet worldguard.ApplicableRegionSet, f worldguard.Flag,
	stack map[string]bool, send func(*localPlayer, string)) map[string]bool {
	msgs := messages(p, toSet, f)
	for _, m := range msgs {
		if !stack[m] {
			send(p, m)
			break
		}
	}
	stack = map[string]bool{}
	for _, m := range msgs {
		stack[m] = true
	}
	if len(stack) > 0 {
		withRegionValues(stack, toSet, f)
	}
	return stack
}

// farewellFlag shows the farewell and farewell-title of regions a player
// leaves. WorldGuard: FarewellFlag
type farewellFlag struct {
	baseHandler
	lastMessages, lastTitles map[string]bool
}

func (h *farewellFlag) initialize(p *localPlayer, _ plugin.Location, set worldguard.ApplicableRegionSet) {
	h.lastMessages = toSet(messages(p, set, worldguard.FlagFarewell))
	h.lastTitles = toSet(messages(p, set, worldguard.FlagFarewellTitle))
}

func toSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, v := range values {
		out[v] = true
	}
	return out
}

func (h *farewellFlag) onCrossBoundary(p *localPlayer, _, _ plugin.Location, toSet worldguard.ApplicableRegionSet,
	_, _ regionSet, _ moveType) bool {
	h.lastMessages = h.collectAndSend(p, toSet, worldguard.FlagFarewell, h.lastMessages, h.s.m.wg.sendChatFlag)
	h.lastTitles = h.collectAndSend(p, toSet, worldguard.FlagFarewellTitle, h.lastTitles, h.s.m.wg.sendTitleFlag)
	return true
}

func (h *farewellFlag) collectAndSend(p *localPlayer, set worldguard.ApplicableRegionSet, f worldguard.Flag,
	stack map[string]bool, send func(*localPlayer, string)) map[string]bool {
	msgs := toSet(messages(p, set, f))
	if len(msgs) > 0 {
		withRegionValues(msgs, set, f)
	}
	// The stack is a Java hash set, whose order this does not keep; with
	// one region left at a time there is one message to choose from.
	for _, m := range sortedKeys(stack) {
		if !msgs[m] {
			send(p, m)
			break
		}
	}
	return msgs
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// replaceMacros fills in %name%, %id%, %online%, %world% and %health%.
// WorldGuard: BukkitStringMatcher.replaceMacros
func (wg *WorldGuard) replaceMacros(p *localPlayer, s string) string {
	s = strings.ReplaceAll(s, "%name%", p.Name())
	s = strings.ReplaceAll(s, "%id%", p.UUID().String())
	s = strings.ReplaceAll(s, "%online%", strconv.Itoa(len(wg.Server.Players())))
	s = strings.ReplaceAll(s, "%world%", wg.WorldName(p.World()))
	return strings.ReplaceAll(s, "%health%", javaFloat(float64(p.Health())))
}

// javaFloat prints a number as Java's String.valueOf(double) does for
// ordinary values: 20.0, 7.5.
func javaFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// sendChatFlag sends a greeting or farewell, a line per newline.
// WorldGuard: MessagingUtil.sendStringToChat
func (wg *WorldGuard) sendChatFlag(p *localPlayer, msg string) {
	msg = wg.replaceMacros(p, worldguard.ReplaceColorMacros(msg))
	for line := range strings.SplitSeq(strings.ReplaceAll(msg, `\n`, "\n"), "\n") {
		sendRaw(p, line)
	}
}

// sendTitleFlag shows a greeting or farewell title; a newline starts the
// subtitle. WorldGuard: MessagingUtil.sendStringToTitle
func (wg *WorldGuard) sendTitleFlag(p *localPlayer, msg string) {
	title, subtitle, _ := strings.Cut(strings.ReplaceAll(msg, `\n`, "\n"), "\n")
	title = wg.replaceMacros(p, worldguard.ReplaceColorMacros(title))
	subtitle = wg.replaceMacros(p, worldguard.ReplaceColorMacros(subtitle))
	// Bukkit leaves the client's title times when the config says not to
	// force the defaults; golem always sends times, so the defaults go.
	p.SendTitle(chat.Legacy(title), chat.Legacy(subtitle), 10, 70, 20)
}

// notifyEntry tells everyone with worldguard.notify that a player entered
// a notify-enter region. WorldGuard: NotifyEntryFlag
type notifyEntry struct{}

func (notifyEntry) onInitial(*session, *localPlayer, worldguard.ApplicableRegionSet, bool, bool) {}
func (notifyEntry) onSet(s *session, p *localPlayer, _, _ plugin.Location, toSet worldguard.ApplicableRegionSet,
	cur, _ bool, _ bool, _ moveType) bool {
	if !cur {
		return true
	}
	var ids []string
	for _, r := range toSet.Regions() {
		ids = append(ids, r.ID())
	}
	s.m.wg.notifyAll(chat.Notify(p.Name(), " entered NOTIFY region: "+strings.Join(ids, ", ")))
	return true
}
func (notifyEntry) onAbsent(*session, *localPlayer, plugin.Location, plugin.Location, worldguard.ApplicableRegionSet,
	bool, moveType) bool {
	return true
}

// notifyExit tells everyone with worldguard.notify that a player left a
// notify-leave region. WorldGuard: NotifyExitFlag
type notifyExit struct{}

func (notifyExit) keepOnClear()                                                                 {}
func (notifyExit) onInitial(*session, *localPlayer, worldguard.ApplicableRegionSet, bool, bool) {}
func (notifyExit) onSet(*session, *localPlayer, plugin.Location, plugin.Location, worldguard.ApplicableRegionSet,
	bool, bool, bool, moveType) bool {
	return true
}
func (notifyExit) onAbsent(s *session, p *localPlayer, _, _ plugin.Location, _ worldguard.ApplicableRegionSet,
	last bool, _ moveType) bool {
	if last {
		s.m.wg.notifyAll(chat.Notify(p.Name(), " left NOTIFY region"))
	}
	return true
}

// survivalLike reports a game mode in which a player can be hurt.
func survivalLike(m plugin.GameMode) bool { return m == plugin.Survival || m == plugin.Adventure }

// healFlag heals or hurts players as the heal flags say. WorldGuard: HealFlag
type healFlag struct {
	baseHandler
	lastHeal time.Time
}

func (h *healFlag) tick(p *localPlayer, set worldguard.ApplicableRegionSet) {
	if p.Health() <= 0 {
		return
	}
	amount, okA := worldguard.Value[int](set, p, worldguard.FlagHealAmount)
	delay, okD := worldguard.Value[int](set, p, worldguard.FlagHealDelay)
	if !okA || !okD || amount == 0 || delay < 0 {
		return
	}
	if amount < 0 && (h.s.invincible(p) || !survivalLike(p.GameMode())) {
		return
	}
	maxHealth := float64(p.MaxHealth())
	minH, _ := worldguard.Value[float64](set, p, worldguard.FlagHealMinHealth)
	maxH, ok := worldguard.Value[float64](set, p, worldguard.FlagHealMaxHealth)
	if !ok {
		maxH = maxHealth
	}
	minH, maxH = math.Min(maxHealth, minH), math.Min(maxHealth, maxH)
	health := float64(p.Health())
	if health >= maxH && amount > 0 {
		return
	}
	now := time.Now()
	switch {
	case delay <= 0:
		if amount > 0 {
			p.SetHealth(float32(maxH))
		} else {
			p.SetHealth(float32(minH)) // kills when heal-min-health is unset
		}
		h.lastHeal = now
	case now.Sub(h.lastHeal) > time.Duration(delay)*time.Second:
		p.SetHealth(float32(math.Min(maxH, math.Max(minH, health+float64(amount)))))
		h.lastHeal = now
	}
}

// feedFlag feeds or starves players as the feed flags say. WorldGuard:
// FeedFlag
type feedFlag struct {
	baseHandler
	lastFeed time.Time
}

func (h *feedFlag) tick(p *localPlayer, set worldguard.ApplicableRegionSet) {
	amount, okA := worldguard.Value[int](set, p, worldguard.FlagFeedAmount)
	delay, okD := worldguard.Value[int](set, p, worldguard.FlagFeedDelay)
	if !okA || !okD || amount == 0 || delay < 0 {
		return
	}
	if amount < 0 && (h.s.invincible(p) || !survivalLike(p.GameMode())) {
		return
	}
	minF, _ := worldguard.Value[int](set, p, worldguard.FlagFeedMinHunger)
	maxF, ok := worldguard.Value[int](set, p, worldguard.FlagFeedMaxHunger)
	if !ok {
		maxF = 20
	}
	minF, maxF = min(20, minF), min(20, maxF)
	if p.Food() >= maxF && amount > 0 {
		return
	}
	now := time.Now()
	switch {
	case delay <= 0:
		if amount > 0 {
			p.SetFood(maxF)
		} else {
			p.SetFood(minF)
		}
	case now.Sub(h.lastFeed) > time.Duration(delay)*time.Second:
		p.SetFood(min(maxF, max(minF, p.Food()+amount)))
	default:
		return
	}
	p.SetSaturation(float32(p.Food()))
	h.lastFeed = now
}

// gameModeFlag puts players in the game-mode flag's mode and back.
// WorldGuard: GameModeFlag
type gameModeFlag struct {
	original    plugin.GameMode
	hasOriginal bool
}

func (h *gameModeFlag) update(s *session, p *localPlayer, v plugin.GameMode, ok bool, w plugin.World) {
	if ok && !s.m.wg.HasBypass(p.Player, w) {
		if p.GameMode() != v {
			if !h.hasOriginal {
				h.original, h.hasOriginal = p.GameMode(), true
			}
			p.SetGameMode(v)
		} else if !h.hasOriginal {
			h.original, h.hasOriginal = s.m.wg.Server.DefaultGameMode(), true
		}
		return
	}
	if h.hasOriginal {
		h.hasOriginal = false
		p.SetGameMode(h.original)
	}
}

func (h *gameModeFlag) onInitial(s *session, p *localPlayer, _ worldguard.ApplicableRegionSet, v plugin.GameMode, ok bool) {
	h.update(s, p, v, ok, p.World())
}

func (h *gameModeFlag) onSet(s *session, p *localPlayer, _, to plugin.Location, _ worldguard.ApplicableRegionSet,
	cur, _ plugin.GameMode, _ bool, _ moveType) bool {
	h.update(s, p, cur, true, to.World)
	return true
}

func (h *gameModeFlag) onAbsent(s *session, p *localPlayer, _, _ plugin.Location, _ worldguard.ApplicableRegionSet,
	_ plugin.GameMode, _ moveType) bool {
	h.update(s, p, 0, false, p.World())
	return true
}

// invincibilityFlag makes players invincible, or not, as the invincible
// flag says. WorldGuard: InvincibilityFlag
type invincibilityFlag struct {
	state worldguard.State
}

func (h *invincibilityFlag) onInitial(_ *session, _ *localPlayer, _ worldguard.ApplicableRegionSet, v worldguard.State, ok bool) {
	h.state = worldguard.StateNone
	if ok {
		h.state = v
	}
}

func (h *invincibilityFlag) onSet(_ *session, _ *localPlayer, _, _ plugin.Location, _ worldguard.ApplicableRegionSet,
	cur, _ worldguard.State, _ bool, _ moveType) bool {
	h.state = cur
	return true
}

func (h *invincibilityFlag) onAbsent(*session, *localPlayer, plugin.Location, plugin.Location, worldguard.ApplicableRegionSet,
	worldguard.State, moveType) bool {
	h.state = worldguard.StateNone
	return true
}

func (h *invincibilityFlag) invincibility(wg *WorldGuard, p *localPlayer) worldguard.State {
	if h.state == worldguard.StateDeny && wg.HasPermission(p.Player, "worldguard.god.override-regions") {
		return worldguard.StateNone
	}
	return h.state
}

// timePattern is what the time-lock flag accepts. WorldGuard:
// TimeLockFlag.timePattern
var timePattern = regexp.MustCompile(`^([+\-])?\d+$`)

// timeLockFlag shows players the time-lock flag's time. WorldGuard:
// TimeLockFlag
type timeLockFlag struct {
	initialTime     int64
	initialRelative bool
	// initialSet is false when the player saw the world's clock.
	initialSet bool
}

// remember records the clock the player sees, to restore it.
func (h *timeLockFlag) remember(p *localPlayer) {
	ticks, fixed, overridden := p.PlayerTime()
	h.initialSet = overridden
	h.initialRelative = !fixed
	h.initialTime = ticks
	if overridden && !fixed {
		h.initialTime = ticks - p.World().DayTime()
	}
}

func setPlayerTime(p *localPlayer, v string) {
	if !timePattern.MatchString(v) {
		return
	}
	t, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return
	}
	p.SetPlayerTime(t, strings.HasPrefix(v, "+") || strings.HasPrefix(v, "-"))
}

func (h *timeLockFlag) onInitial(_ *session, p *localPlayer, _ worldguard.ApplicableRegionSet, v string, ok bool) {
	h.remember(p)
	if ok {
		setPlayerTime(p, v)
	}
}

func (h *timeLockFlag) onSet(_ *session, p *localPlayer, _, _ plugin.Location, _ worldguard.ApplicableRegionSet,
	cur, _ string, hadLast bool, _ moveType) bool {
	if !hadLast {
		h.remember(p)
	}
	setPlayerTime(p, cur)
	return true
}

func (h *timeLockFlag) onAbsent(_ *session, p *localPlayer, _, _ plugin.Location, _ worldguard.ApplicableRegionSet,
	_ string, _ moveType) bool {
	if h.initialSet {
		p.SetPlayerTime(h.initialTime, h.initialRelative)
	} else {
		p.ResetPlayerTime()
	}
	h.initialTime, h.initialRelative, h.initialSet = 0, true, false
	return true
}

// weatherLockFlag shows players the weather-lock flag's weather.
// WorldGuard: WeatherLockFlag
type weatherLockFlag struct {
	initialRaining, initialSet bool
}

func (h *weatherLockFlag) remember(p *localPlayer) {
	h.initialRaining, h.initialSet = p.PlayerWeather()
}

func (h *weatherLockFlag) onInitial(_ *session, p *localPlayer, _ worldguard.ApplicableRegionSet, v plugin.Weather, ok bool) {
	h.remember(p)
	if ok {
		p.SetPlayerWeather(v != plugin.WeatherClear)
	}
}

func (h *weatherLockFlag) onSet(_ *session, p *localPlayer, _, _ plugin.Location, _ worldguard.ApplicableRegionSet,
	cur, _ plugin.Weather, hadLast bool, _ moveType) bool {
	if !hadLast {
		h.remember(p)
	}
	p.SetPlayerWeather(cur != plugin.WeatherClear)
	return true
}

func (h *weatherLockFlag) onAbsent(_ *session, p *localPlayer, _, _ plugin.Location, _ worldguard.ApplicableRegionSet,
	_ plugin.Weather, _ moveType) bool {
	if h.initialSet {
		p.SetPlayerWeather(h.initialRaining)
	} else {
		p.ResetPlayerWeather()
	}
	h.initialSet = false
	return true
}

// godMode is /god's switch. WorldGuard: GodMode
type godMode struct {
	baseHandler
	on bool
}

func (h *godMode) invincibility(*localPlayer) worldguard.State {
	if h.on {
		return worldguard.StateAllow
	}
	return worldguard.StateNone
}

// waterBreathing lets players in the wg-amphibious group breathe under
// water. WorldGuard: WaterBreathing
type waterBreathing struct {
	baseHandler
	on bool
}
