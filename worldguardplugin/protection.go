package worldguardplugin

import (
	"strings"
	"sync"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/event"
	"github.com/golem-mc/worldguard/internal/config"
)

// protection is WorldGuard's listeners: the event abstraction, region
// protection, build permissions and the world settings.
type protection struct {
	wg *WorldGuard

	interact, pickup, entityBreak *debounce
	moveItem                      *debounce

	mu          sync.Mutex
	lastDeny    map[plugin.UUID]time.Time
	lastDisembk map[plugin.UUID]time.Time
}

// registerProtection starts the protection listeners. WorldGuard:
// WorldGuardPlugin.onEnable (the listeners' registration)
func (wg *WorldGuard) registerProtection() {
	p := &protection{wg: wg,
		interact: newDebounce(10 * time.Second), pickup: newDebounce(10 * time.Second),
		entityBreak: newDebounce(10 * time.Second), moveItem: newDebounce(30 * time.Second),
		lastDeny: map[plugin.UUID]time.Time{}, lastDisembk: map[plugin.UUID]time.Time{}}
	event.Define(wg.Server)
	p.registerAbstraction()
	p.registerRegionProtection()
	p.registerRegionFlags()
	p.registerBuildPermissions()
	p.registerBlockRules()
	p.registerEntityRules()
	p.registerPlayerRules()
	p.registerWorldRules()
}

func (p *protection) fire(e plugin.Event) { p.wg.Server.Events().Fire(e) }

type cancellable interface {
	plugin.Event
	Cancelled() bool
}

// fireToCancel fires e and cancels original when e ends cancelled.
// WorldGuard: Events.fireToCancel
func (p *protection) fireToCancel(original interface{ SetCancelled(bool) }, e cancellable) bool {
	p.fire(e)
	if e.Cancelled() {
		original.SetCancelled(true)
		return true
	}
	return false
}

// fireAndTest fires e and reports whether it ends cancelled.
func (p *protection) fireAndTest(e cancellable) bool {
	p.fire(e)
	return e.Cancelled()
}

type blockEvent interface {
	cancellable
	Part() *event.Blocks
}

// fireBulk fires a block event over many blocks: original is cancelled only
// when a listener denied it outright; otherwise the blocks left are the
// ones it may change. WorldGuard: Events.fireBulkEventToCancel
func (p *protection) fireBulk(original interface{ SetCancelled(bool) }, e blockEvent) bool {
	p.fire(e)
	if e.Part().ExplicitlyDenied() {
		original.SetCancelled(true)
		return true
	}
	return false
}

func (p *protection) cause(objects ...any) event.Cause {
	return event.NewCause(p.wg.Server, objects...)
}

func (p *protection) cfg(w plugin.World) *config.World { return p.wg.WorldConfig(w) }

func (p *protection) regionsOn(w plugin.World) bool { return w != nil && p.cfg(w).UseRegions }

func (p *protection) query() worldguard.RegionQuery { return p.wg.Query() }

func blockLoc(b plugin.Block) plugin.Location {
	return plugin.Location{World: b.World, Pos: plugin.Vec3{X: float64(b.Pos.X), Y: float64(b.Pos.Y), Z: float64(b.Pos.Z)}}
}

func posLoc(w plugin.World, pos plugin.BlockPos) plugin.Location {
	return blockLoc(plugin.Block{World: w, Pos: pos})
}

// associable answers who the cause is to regions: a player, the regions
// a mob or block stands in, or a non-member. WorldGuard:
// AbstractListener.createRegionAssociable
func (p *protection) associable(c event.Cause) worldguard.RegionAssociable {
	if !c.Known() {
		return worldguard.ConstantAssociation(worldguard.AssociationNonMember)
	}
	switch root := c.Root().(type) {
	case plugin.Player:
		return p.wg.WrapPlayer(root)
	case plugin.Entity:
		return p.overlap(root.Location())
	case plugin.Block:
		return p.overlap(blockLoc(root))
	}
	return worldguard.ConstantAssociation(worldguard.AssociationNonMember)
}

// overlap answers the association of something standing at loc: owner of
// the regions there. WorldGuard: DelayedRegionOverlapAssociation
func (p *protection) overlap(loc plugin.Location) *worldguard.RegionOverlapAssociation {
	return &worldguard.RegionOverlapAssociation{
		MaxPriority: loc.World != nil && p.cfg(loc.World).UseMaxPriorityAssociation,
		Lazy:        func() []*worldguard.Region { return p.query().ApplicableRegions(loc).Regions() },
	}
}

// whitelisted reports whether the cause is a player who bypasses region
// protection; pvp is never bypassed. WorldGuard: RegionProtectionListener.isWhitelisted
func (p *protection) whitelisted(c event.Cause, w plugin.World, pvp bool) bool {
	pl, ok := c.Root().(plugin.Player)
	return ok && !pvp && p.wg.HasBypass(pl, pl.World())
}

// tellDenied tells the player behind a denied event the deny-message
// flag's text, at most every half second. WorldGuard:
// RegionProtectionListener.tellErrorMessage
func (p *protection) tellDenied(d *event.Delegate, loc plugin.Location, what string) {
	if d.Silent || d.Cause.Indirect() {
		return
	}
	pl, ok := d.Cause.Root().(plugin.Player)
	if !ok {
		return
	}
	now := time.Now()
	p.mu.Lock()
	last, seen := p.lastDeny[pl.UUID()]
	if seen && now.Sub(last) < 500*time.Millisecond {
		p.mu.Unlock()
		return
	}
	p.lastDeny[pl.UUID()] = now
	p.mu.Unlock()
	lp := p.wg.WrapPlayer(pl)
	msg, _ := p.query().QueryValue(loc, lp, worldguard.FlagDenyMessage).(string)
	p.sendDeny(pl, what, msg)
}

// sendDeny formats a deny message: the player macros, %what%, colours.
// WorldGuard: RegionProtectionListener.formatAndSendDenyMessage
func (p *protection) sendDeny(pl plugin.Player, what, msg string) {
	if msg == "" {
		return
	}
	msg = p.wg.replaceMacros(&localPlayer{Player: pl, wg: p.wg}, msg)
	msg = worldguard.ReplaceColorMacros(msg)
	tell(pl, strings.ReplaceAll(msg, "%what%", what))
}

// combine appends the event's own flags to flags. WorldGuard:
// RegionProtectionListener.combine
func combine(d *event.Delegate, flags ...*worldguard.StateFlag) []*worldguard.StateFlag {
	return append(append([]*worldguard.StateFlag(nil), flags...), d.Flags...)
}

// denyEffect puffs smoke where a player was denied, when particle-effects
// is on. WorldGuard: EventAbstractionListener.playDenyEffect
func (p *protection) denyEffect(pl plugin.Player, w plugin.World, pos plugin.Vec3) {
	if !p.wg.Config.ParticleEffects || w == nil {
		return
	}
	smoke, ok := p.wg.Server.Registries().Particle(plugin.MinecraftKey("smoke"))
	if !ok {
		return
	}
	if pl != nil {
		pl.SpawnParticles(plugin.Particle{Type: smoke}, pos, 8, plugin.Vec3{X: .2, Y: .2, Z: .2}, 0)
		return
	}
	w.SpawnParticles(plugin.Particle{Type: smoke}, pos, 8, plugin.Vec3{X: .2, Y: .2, Z: .2}, 0)
}

// interactBypass reports whether allow-all-interactions names b.
func (p *protection) interactBypass(b plugin.Block) bool {
	return p.cfg(b.World).AllowAllInteract.HasBlock(b.State().Block())
}

func (p *protection) interactBypassItem(w plugin.World, i registry.Item) bool {
	t := p.cfg(w).AllowAllInteract
	if t.HasItem(i) {
		return true
	}
	b, ok := i.Block()
	return ok && t.HasBlock(b)
}

// debounce remembers an event's verdict for a while, so that an event the
// server fires every tick (a mob on farmland, a hopper) is judged once.
// WorldGuard: EventDebounce
type debounce struct {
	ttl time.Duration

	mu      sync.Mutex
	entries map[any]debounceEntry
}

type debounceEntry struct {
	expires   time.Time
	cancelled bool
}

func newDebounce(ttl time.Duration) *debounce {
	return &debounce{ttl: ttl, entries: map[any]debounceEntry{}}
}

// lookup answers the remembered verdict for key.
func (d *debounce) lookup(key any) (cancelled, ok bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	e, ok := d.entries[key]
	if !ok || time.Now().After(e.expires) {
		return false, false
	}
	return e.cancelled, true
}

func (d *debounce) store(key any, cancelled bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	if len(d.entries) >= 1000 {
		for k, e := range d.entries {
			if now.After(e.expires) {
				delete(d.entries, k)
			}
		}
		if len(d.entries) >= 1000 {
			clear(d.entries)
		}
	}
	d.entries[key] = debounceEntry{expires: now.Add(d.ttl), cancelled: cancelled}
}

// fire fires e unless key's verdict is remembered, and cancels original
// when the verdict is to cancel.
func (d *debounce) fire(p *protection, key any, original interface{ SetCancelled(bool) }, e cancellable) {
	cancelled, ok := d.lookup(key)
	if !ok {
		cancelled = p.fireAndTest(e)
		d.store(key, cancelled)
	}
	if cancelled {
		original.SetCancelled(true)
	}
}
