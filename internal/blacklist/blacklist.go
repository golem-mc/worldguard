// Package blacklist is WorldGuard's blacklist: blocks and items whose
// breaking, placing, use and so on is denied, logged or reported, read from
// a world's blacklist.txt. WorldGuard: com.sk89q.worldguard.blacklist
package blacklist

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldedit/block"
	"github.com/golem-mc/worldguard"
)

// EventType is what a player does to a block or item. WorldGuard: EventType
type EventType int

// The event types, in blacklist.txt's order.
const (
	Break EventType = iota
	Place
	Interact
	Dispense
	DestroyWith
	Acquire
	Equip
	Drop
	Use
	eventTypes
)

var (
	ruleNames    = [eventTypes]string{"on-break", "on-place", "on-interact", "on-dispense", "on-destroy-with", "on-acquire", "on-equip", "on-drop", "on-use"}
	descriptions = [eventTypes]string{"break", "place", "interact with", "dispense", "destroy with", "acquire", "equip", "drop", "use"}
)

// Description is the verb the event is told with: "break", "interact with".
func (t EventType) Description() string { return descriptions[t] }

// Player is the player behind an event.
type Player interface {
	Name() string
	// Groups answers the player's permission groups.
	Groups() []string
	HasPermission(node string) bool
	SendMessage(plugin.Text)
	Kick(reason plugin.Text)
	// Ban bans the player by name and kicks them with reason.
	Ban(reason plugin.Text)
}

// Target is the block or item an event is about.
type Target struct {
	// ID is the namespaced block or item id.
	ID string
	// Name is the translated name the client shows.
	Name plugin.Text
}

// plainName is the name as plain text, the translation key for a
// translated name. WorldGuard: PlainComponentSerializer of getRichName
func (t Target) plainName() string { return t.Name.PlainString() }

// BlockTarget is a block. WorldGuard: BlockTarget
func BlockTarget(b registry.Block) Target {
	return Target{ID: b.Name(), Name: plugin.Translate("block.minecraft." + strings.TrimPrefix(b.Name(), "minecraft:"))}
}

// ItemTarget is an item, or the block of the same name when there is one,
// as Bukkit's Material is. WorldGuard: BukkitUtil.createTarget
func ItemTarget(i registry.Item) Target {
	if b, ok := registry.BlockByName(i.Name()); ok {
		return BlockTarget(b)
	}
	key, ok := registry.ItemTranslationKey(int32(i))
	if !ok {
		key = "item.minecraft." + strings.TrimPrefix(i.Name(), "minecraft:")
	}
	return Target{ID: i.Name(), Name: plugin.Translate(key)}
}

// Event is one thing a player, or a dispenser, does. WorldGuard: BlacklistEvent
type Event struct {
	Type EventType
	// Player is nil for a dispenser.
	Player Player
	Pos    plugin.BlockPos
	Target Target
}

// causeName is who did it: the player, or where.
func (e Event) causeName() string {
	if e.Player != nil {
		return e.Player.Name()
	}
	return vector(e.Pos)
}

// vector prints a position as WorldEdit's BlockVector3.toString.
func vector(p plugin.BlockPos) string { return fmt.Sprintf("(%d, %d, %d)", p.X, p.Y, p.Z) }

// LoggerMessage is the console log's line. WorldGuard: getLoggerMessage
func (e Event) LoggerMessage() string {
	who := "(unknown)"
	if e.Player != nil {
		who = e.Player.Name()
	}
	if e.Type == Dispense {
		who = vector(e.Pos)
	}
	return who + " tried to " + e.Type.Description() + " " + e.Target.plainName()
}

// Logger records events. WorldGuard: LoggerHandler
type Logger interface {
	// Log records an event; comment is "" when the entry has none.
	Log(e Event, comment string, hasComment bool)
	Close()
}

// Blacklist is a world's loaded blacklist.txt. WorldGuard: Blacklist
type Blacklist struct {
	entries   map[string][]*entry
	count     int
	whitelist bool
	// Loggers record log actions; Notify reports notify actions.
	Loggers []Logger
	Notify  func(plugin.Text)

	mu        sync.Mutex
	repeating map[string]*tracked
}

// tracked is the last event of a cause, to keep repeats quiet.
// WorldGuard: TrackedEvent
type tracked struct {
	last     EventType
	has      bool
	time     time.Time // when last was told
	accessed time.Time
}

const (
	repeatWindow    = 3 * time.Second  // WorldGuard: TrackedEvent.matches
	repeatExpiry    = 30 * time.Second // WorldGuard: Blacklist.repeatingEventCache
	repeatCacheSize = 1000
)

// Len answers how many block and item headings the blacklist holds.
func (b *Blacklist) Len() int { return b.count }

// Whitelist reports whether entries allow rather than deny.
func (b *Blacklist) Whitelist() bool { return b.whitelist }

// Close closes the loggers.
func (b *Blacklist) Close() {
	for _, l := range b.Loggers {
		l.Close()
	}
}

var heading = regexp.MustCompile(`^\[.*\]$`)

// Load reads a blacklist file; warn gets each line it cannot use.
// WorldGuard: Blacklist.load
func Load(r io.Reader, fileName string, whitelist bool, warn func(string)) (*Blacklist, error) {
	b := &Blacklist{entries: map[string][]*entry{}, whitelist: whitelist, repeating: map[string]*tracked{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(nil, 1<<20)
	var current []*entry
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == ';' || line[0] == '#' {
			continue
		}
		if heading.MatchString(line) {
			current = nil
			for item := range strings.SplitSeq(line[1:len(line)-1], ",") {
				id, err := parseTarget(item)
				if err != nil {
					warn("Could not parse a block/item heading: " + err.Error())
					continue
				}
				e := &entry{b: b, actions: map[EventType][]action{}}
				b.entries[id] = append(b.entries[id], e)
				b.count++
				current = append(current, e)
			}
			continue
		}
		if current == nil {
			warn("Found option with no heading " + fileName + " for '" + line + "'")
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) == 1 || len(parts) == 2 && parts[1] == "" {
			warn("Found option with no value " + fileName + " for '" + line + "'")
			continue
		}
		key, value := parts[0], parts[1]
		unknown := false
		for _, e := range current {
			switch {
			case strings.EqualFold(key, "ignore-groups"):
				e.ignoreGroups = map[string]bool{}
				for g := range strings.SplitSeq(value, ",") {
					e.ignoreGroups[strings.ToLower(g)] = true
				}
			case strings.EqualFold(key, "ignore-perms"):
				e.ignorePerms = strings.Split(value, ",")
			case strings.EqualFold(key, "message"):
				e.message, e.hasMessage = worldguard.ReplaceColorMacros(strings.TrimSpace(value)), true
			case strings.EqualFold(key, "comment"):
				e.comment, e.hasComment = worldguard.ReplaceColorMacros(strings.TrimSpace(value)), true
			default:
				t, ok := ruleType(key)
				if !ok {
					unknown = true
					continue
				}
				e.actions[t] = append(e.actions[t], parseActions(value, warn)...)
			}
		}
		if unknown {
			warn("Unknown option '" + key + "' in " + fileName + " for '" + line + "'")
		}
	}
	return b, sc.Err()
}

func ruleType(key string) (EventType, bool) {
	for t, n := range ruleNames {
		if strings.EqualFold(n, key) {
			return EventType(t), true
		}
	}
	return 0, false
}

// parseTarget turns a heading item into a block or item id: a block or
// item name, namespaced or not, or a legacy numeric id ("35:14") of a block.
// WorldGuard: TargetMatcherParser.fromInput
func parseTarget(input string) (string, error) {
	input = strings.ToLower(strings.TrimSpace(input))
	name := input
	if !strings.Contains(name, ":") || isLegacyID(name) {
		if isLegacyID(name) {
			if st, ok := block.LegacyBlockState(name); ok {
				return st.Block().Name(), nil
			}
			return "", fmt.Errorf("Unknown block or item name: %s", input) //nolint:staticcheck // WorldGuard's message
		}
		name = "minecraft:" + name
	}
	if b, ok := registry.BlockByName(name); ok {
		return b.Name(), nil
	}
	if i, ok := registry.ItemByName(name); ok {
		return i.Name(), nil
	}
	return "", fmt.Errorf("Unknown block or item name: %s", input) //nolint:staticcheck // WorldGuard's message
}

func isLegacyID(s string) bool {
	id, data, _ := strings.Cut(s, ":")
	if _, err := strconv.Atoi(id); err != nil {
		return false
	}
	if data == "" {
		return !strings.Contains(s, ":")
	}
	_, err := strconv.Atoi(data)
	return err == nil
}

// Check runs the entries for the event's target and reports whether the
// event may go ahead. forceRepeat runs the repeat-guarded actions even for
// a repeat; silent runs no action that tells, logs, kicks or bans.
// WorldGuard: Blacklist.check
func (b *Blacklist) Check(e Event, forceRepeat, silent bool) bool {
	ok := true
	for _, en := range b.entries[e.Target.ID] {
		if !en.check(e, forceRepeat, silent) {
			ok = false
		}
	}
	return ok
}

// repeated reports whether e repeats its cause's last event within the
// window, else remembers it. WorldGuard: BlacklistEntry.check
func (b *Blacklist) repeated(e Event) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	key := e.causeName()
	t, ok := b.repeating[key]
	if ok && now.Sub(t.accessed) > repeatExpiry {
		ok = false
	}
	if !ok {
		if len(b.repeating) >= repeatCacheSize {
			b.evict(now)
		}
		t = &tracked{}
		b.repeating[key] = t
	}
	t.accessed = now
	if t.has && t.last == e.Type && now.Sub(t.time) < repeatWindow {
		return true
	}
	t.last, t.has, t.time = e.Type, true, now
	return false
}

// evict drops expired causes, or the least recently seen one.
func (b *Blacklist) evict(now time.Time) {
	var oldest string
	var at time.Time
	for k, t := range b.repeating {
		if now.Sub(t.accessed) > repeatExpiry {
			delete(b.repeating, k)
			continue
		}
		if oldest == "" || t.accessed.Before(at) {
			oldest, at = k, t.accessed
		}
	}
	if len(b.repeating) >= repeatCacheSize {
		delete(b.repeating, oldest)
	}
}
