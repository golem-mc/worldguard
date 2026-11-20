package worldguardplugin

import (
	"sync"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/internal/chat"
)

// sessionLifetime is how long a session outlives its player's last
// access, so god mode and the like survive a quick reconnect.
// WorldGuard: AbstractSessionManager.SESSION_LIFETIME
const sessionLifetime = 10 * time.Minute

// sessionTickPeriod is how often, in ticks, the handlers' tick runs (heal,
// feed). WorldGuard: AbstractSessionManager.RUN_DELAY
const sessionTickPeriod = 20

// moveType is how a player moved. WorldGuard: MoveType
type moveType struct {
	cancellable, teleport bool
}

var (
	moveRespawn             = moveType{teleport: true}
	moveEmbark              = moveType{cancellable: true}
	moveMove                = moveType{cancellable: true}
	moveTeleport            = moveType{cancellable: true, teleport: true}
	moveOtherNonCancellable = moveType{}
	moveOtherCancellable    = moveType{cancellable: true}
)

// regionSet is a set of regions, for the regions a move entered or left.
type regionSet map[*worldguard.Region]bool

func setOf(regions []*worldguard.Region) regionSet {
	out := make(regionSet, len(regions))
	for _, r := range regions {
		out[r] = true
	}
	return out
}

// minus answers the regions of s not in o.
func (s regionSet) minus(o regionSet) regionSet {
	out := regionSet{}
	for r := range s {
		if !o[r] {
			out[r] = true
		}
	}
	return out
}

// handler reacts to a player's session: their moves across region
// boundaries and a periodic tick. WorldGuard: Handler
type handler interface {
	initialize(p *localPlayer, current plugin.Location, set worldguard.ApplicableRegionSet)
	uninitialize(p *localPlayer, current plugin.Location, set worldguard.ApplicableRegionSet)
	// testMoveTo and onCrossBoundary answer false to refuse a move.
	testMoveTo(p *localPlayer, from, to plugin.Location, toSet worldguard.ApplicableRegionSet, mt moveType) bool
	onCrossBoundary(p *localPlayer, from, to plugin.Location, toSet worldguard.ApplicableRegionSet,
		entered, exited regionSet, mt moveType) bool
	tick(p *localPlayer, set worldguard.ApplicableRegionSet)
	// invincibility answers StateAllow or StateDeny to decide it, StateNone
	// to leave it to the other handlers.
	invincibility(p *localPlayer) worldguard.State
}

// baseHandler is a handler that does nothing, to embed.
type baseHandler struct{ s *session }

func (baseHandler) initialize(*localPlayer, plugin.Location, worldguard.ApplicableRegionSet)   {}
func (baseHandler) uninitialize(*localPlayer, plugin.Location, worldguard.ApplicableRegionSet) {}
func (baseHandler) testMoveTo(*localPlayer, plugin.Location, plugin.Location, worldguard.ApplicableRegionSet, moveType) bool {
	return true
}
func (baseHandler) onCrossBoundary(*localPlayer, plugin.Location, plugin.Location, worldguard.ApplicableRegionSet,
	regionSet, regionSet, moveType) bool {
	return true
}
func (baseHandler) tick(*localPlayer, worldguard.ApplicableRegionSet) {}
func (baseHandler) invincibility(*localPlayer) worldguard.State       { return worldguard.StateNone }

// session is what WorldGuard keeps about a player. WorldGuard: Session
type session struct {
	m *sessionManager
	// bypassDisabled turns the player's region bypass off (/rg bypass).
	bypassDisabled bool

	handlers      []handler
	godMode       *godMode
	waterBreath   *waterBreathing
	gameMode      *gameModeFlag
	lastValid     *plugin.Location
	lastRegionSet regionSet
	needRefresh   bool
	initialized   bool
	lastAccess    time.Time
}

// initialize records where the player is and lets the handlers apply the
// flags there. WorldGuard: Session.initialize
func (s *session) initialize(p *localPlayer) {
	wg := s.m.wg
	loc := p.Location()
	set := wg.Query().ApplicableRegions(loc)
	s.lastValid = &loc
	s.lastRegionSet = setOf(set.Regions())
	s.bypassDisabled = wg.Config.DisableDefaultBypass
	if wg.Config.AnnounceBypassStatus && wg.HasPermission(p.Player, "worldguard.region.toggle-bypass") {
		not := ""
		if s.bypassDisabled {
			not = "not "
		}
		p.SendMessage(plugin.Plain("You are " + not + "bypassing region protection. " +
			"You can toggle this with /rg bypass").Color("dark_purple"))
	}
	for _, h := range s.handlers {
		h.initialize(p, loc, set)
	}
}

// uninitialize undoes what the handlers applied. WorldGuard:
// Session.uninitialize
func (s *session) uninitialize(p *localPlayer) {
	loc := p.Location()
	set := s.m.wg.Query().ApplicableRegions(loc)
	for _, h := range s.handlers {
		h.uninitialize(p, loc, set)
	}
	s.lastValid = nil
	s.lastRegionSet = nil
}

func (s *session) tick(p *localPlayer) {
	set := s.m.wg.Query().ApplicableRegions(p.Location())
	for _, h := range s.handlers {
		h.tick(p, set)
	}
}

// resetState applies the flags afresh. WorldGuard: Session.resetState
func (s *session) resetState(p *localPlayer) {
	s.uninitialize(p)
	s.initialize(p)
	s.needRefresh = true
}

// invincible reports whether the player takes no damage: any handler's
// deny wins over the others' allow. WorldGuard: Session.isInvincible
func (s *session) invincible(p *localPlayer) bool {
	invincible := false
	for _, h := range s.handlers {
		switch h.invincibility(p) {
		case worldguard.StateDeny:
			return false
		case worldguard.StateAllow:
			invincible = true
		}
	}
	return invincible
}

// testMoveTo answers where to send the player back to when the handlers
// refuse the move to to, nil to let it happen. A forced test runs even
// within the same block. WorldGuard: Session.testMoveTo
func (s *session) testMoveTo(p *localPlayer, to plugin.Location, mt moveType, forced bool) *plugin.Location {
	if s.lastValid == nil {
		return nil
	}
	if !forced && s.needRefresh {
		forced = true
	}
	s.needRefresh = false
	from := *s.lastValid
	if !forced && from.World == to.World && from.Pos.Block() == to.Pos.Block() {
		return nil
	}
	toSet := s.m.wg.Query().ApplicableRegions(to)
	for _, h := range s.handlers {
		if !h.testMoveTo(p, from, to, toSet, mt) && mt.cancellable {
			return s.lastValid
		}
	}
	toRegions := setOf(toSet.Regions())
	entered, exited := toRegions.minus(s.lastRegionSet), s.lastRegionSet.minus(toRegions)
	for _, h := range s.handlers {
		if !h.onCrossBoundary(p, from, to, toSet, entered, exited, mt) && mt.cancellable {
			return s.lastValid
		}
	}
	s.lastValid = &to
	s.lastRegionSet = toRegions
	return nil
}

// sessionManager holds the players' sessions. WorldGuard:
// AbstractSessionManager, BukkitSessionManager
type sessionManager struct {
	wg *WorldGuard

	mu       sync.Mutex
	sessions map[plugin.UUID]*session
}

func newSessionManager(wg *WorldGuard) *sessionManager {
	return &sessionManager{wg: wg, sessions: map[plugin.UUID]*session{}}
}

func (m *sessionManager) register() {
	plugin.Listen(m.wg.Server, func(e *plugin.PlayerJoinEvent) { m.wg.onJoin(e.Player) })
	plugin.Listen(m.wg.Server, func(e *plugin.PlayerQuitEvent) { m.quit(e.Player) })
	m.wg.Server.Scheduler().RunTimer(sessionTickPeriod, sessionTickPeriod, m.tickAll)
	m.registerMovement()
	m.registerInvincibility()
	for _, p := range m.wg.Server.Players() {
		m.process(p)
	}
}

// newSession makes a session with every handler, in upstream's order.
func (m *sessionManager) newSession() *session {
	s := &session{m: m}
	s.godMode = &godMode{}
	s.waterBreath = &waterBreathing{}
	s.gameMode = &gameModeFlag{}
	s.handlers = []handler{
		&healFlag{baseHandler: baseHandler{s}},
		&feedFlag{baseHandler: baseHandler{s}},
		newValueChange[bool](s, worldguard.FlagNotifyEnter, &notifyEntry{}),
		newValueChange[bool](s, worldguard.FlagNotifyLeave, &notifyExit{}),
		&entryFlag{baseHandler: baseHandler{s}},
		newValueChange[worldguard.State](s, worldguard.FlagExit, &exitFlag{}),
		&farewellFlag{baseHandler: baseHandler{s}},
		&greetingFlag{baseHandler: baseHandler{s}},
		newValueChange[plugin.GameMode](s, worldguard.FlagGameMode, s.gameMode),
		newValueChange[worldguard.State](s, worldguard.FlagInvincible, &invincibilityFlag{}),
		newValueChange[string](s, worldguard.FlagTimeLock, &timeLockFlag{}),
		newValueChange[plugin.Weather](s, worldguard.FlagWeatherLock, &weatherLockFlag{}),
		s.godMode,
		s.waterBreath,
	}
	return s
}

// get answers p's session, making and initializing it on first use.
// WorldGuard: AbstractSessionManager.get
func (m *sessionManager) get(p plugin.Player) *session {
	m.mu.Lock()
	s, ok := m.sessions[p.UUID()]
	if !ok {
		s = m.newSession()
		m.sessions[p.UUID()] = s
	}
	s.lastAccess = time.Now()
	m.mu.Unlock()
	if !s.initialized {
		s.initialized = true
		s.initialize(m.wg.local(p))
	}
	return s
}

// getIfPresent answers p's session, nil when they have none.
func (m *sessionManager) getIfPresent(p plugin.Player) *session {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[p.UUID()]
	if s != nil {
		s.lastAccess = time.Now()
	}
	return s
}

// process starts a joining player's session, or refreshes the one kept
// from their last visit. WorldGuard: BukkitSessionManager.onPlayerProcess
func (m *sessionManager) process(p plugin.Player) {
	if s := m.getIfPresent(p); s != nil && s.initialized {
		s.resetState(m.wg.local(p))
	} else {
		m.get(p)
	}
	m.wg.applyPlayerModes(p)
}

// quit tests the player's last move once more, sending them back if it is
// refused, and undoes the flags. WorldGuard: PlayerMoveListener.onPlayerQuit
func (m *sessionManager) quit(p plugin.Player) {
	lp := m.wg.local(p)
	s := m.get(p)
	if loc := s.testMoveTo(lp, p.Location(), moveOtherCancellable, false); loc != nil {
		p.Teleport(*loc, plugin.TeleportPlugin)
	}
	s.uninitialize(lp)
	s.initialized = false
}

// tickAll runs the handlers' tick for every player and forgets sessions
// unused for sessionLifetime. WorldGuard: BukkitSessionManager.run
func (m *sessionManager) tickAll() {
	online := map[plugin.UUID]bool{}
	for _, p := range m.wg.Server.Players() {
		online[p.UUID()] = true
		m.get(p).tick(m.wg.local(p))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		if !online[id] && time.Since(s.lastAccess) > sessionLifetime {
			delete(m.sessions, id)
		}
	}
}

// resetAll applies every online player's flags afresh. WorldGuard:
// BukkitSessionManager.resetAllStates
func (m *sessionManager) resetAll() {
	for _, p := range m.wg.Server.Players() {
		if s := m.getIfPresent(p); s != nil {
			s.resetState(m.wg.local(p))
		}
	}
}

// shutdown undoes the flags of every online player. WorldGuard:
// BukkitSessionManager.shutdown
func (m *sessionManager) shutdown() {
	for _, p := range m.wg.Server.Players() {
		if s := m.getIfPresent(p); s != nil && s.initialized {
			s.uninitialize(m.wg.local(p))
		}
	}
}

// local answers p as WorldGuard's player.
func (wg *WorldGuard) local(p plugin.Player) *localPlayer {
	if lp, ok := p.(*localPlayer); ok {
		return lp
	}
	return &localPlayer{Player: p, wg: wg}
}

// notifyAll sends a notice to everyone with worldguard.notify and logs it.
// WorldGuard: BukkitWorldGuardPlatform.broadcastNotification
func (wg *WorldGuard) notifyAll(msg plugin.Text) {
	for _, p := range wg.Server.Players() {
		if wg.HasPermission(p, "worldguard.notify") {
			p.SendMessage(msg)
		}
	}
	wg.Log.Info(msg.PlainString())
}

// sendRaw sends a message with legacy formatting, a line per newline.
// WorldGuard: LocalPlayer.printRaw
func sendRaw(to plugin.CommandSender, msg string) {
	to.SendMessage(chat.Legacy(msg))
}
