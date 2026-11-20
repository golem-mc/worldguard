package event

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard"
)

// Result is a listener's verdict on an event. WorldGuard: Event.Result
type Result int8

const (
	// Default leaves the event to the next listener.
	Default Result = iota
	// Allow lets the event through; protection does not check it.
	Allow
	// Deny stops the event.
	Deny
)

// Delegate is what every WorldGuard event carries. WorldGuard: DelegateEvent
type Delegate struct {
	plugin.Base
	// Original is the server event this one stands for; nil when fired
	// for itself.
	Original plugin.Event
	Cause    Cause
	// Flags are further flags the protection check must also pass.
	Flags  []*worldguard.StateFlag
	Result Result
	// Silent events tell the player nothing when denied.
	Silent bool
}

// Cancelled reports whether the event is denied.
func (d *Delegate) Cancelled() bool { return d.Result == Deny }

// SetCancelled denies the event; false does nothing, as upstream.
func (d *Delegate) SetCancelled(v bool) {
	if v {
		d.Result = Deny
	}
}

// SetAllowed marks the event allowed when v is set.
func (d *Delegate) SetAllowed(v bool) {
	if v {
		d.Result = Allow
	}
}

// AddFlags adds flags the protection check must also pass.
func (d *Delegate) AddFlags(flags ...*worldguard.StateFlag) { d.Flags = append(d.Flags, flags...) }

// Blocks is the part of a block event naming the blocks. An event whose
// blocks are all filtered away is denied. WorldGuard: AbstractBlockEvent
type Blocks struct {
	Delegate
	World plugin.World
	// Positions are the blocks affected; listeners filter them.
	Positions []plugin.BlockPos
	// Type is the block the event is about: the one placed, or the one
	// broken.
	Type registry.Block
}

// Cancelled reports whether the event is denied or no block is left.
func (b *Blocks) Cancelled() bool { return len(b.Positions) == 0 || b.Delegate.Cancelled() }

// Part answers the block part of the event, whichever event it is.
func (b *Blocks) Part() *Blocks { return b }

// ExplicitlyDenied reports whether a listener denied the event outright,
// rather than filtering some blocks.
func (b *Blocks) ExplicitlyDenied() bool { return b.Delegate.Cancelled() }

// Filter drops the blocks keep refuses; with cancelOnFalse, one refusal
// drops all and denies. It answers whether any block was refused.
// WorldGuard: AbstractBlockEvent.filter
func (b *Blocks) Filter(keep func(plugin.Location) bool, cancelOnFalse bool) bool {
	removed := false
	out := make([]plugin.BlockPos, 0, len(b.Positions))
	for _, p := range b.Positions {
		if keep(plugin.Location{World: b.World, Pos: plugin.Vec3{X: float64(p.X), Y: float64(p.Y), Z: float64(p.Z)}}) {
			out = append(out, p)
			continue
		}
		removed = true
		if cancelOnFalse {
			b.Positions = nil
			b.Result = Deny
			return true
		}
	}
	b.Positions = out
	return removed
}

// PlaceBlockEvent is a block placed or changed into another. WorldGuard:
// PlaceBlockEvent
type PlaceBlockEvent struct{ Blocks }

// BreakBlockEvent is a block broken or removed. WorldGuard: BreakBlockEvent
type BreakBlockEvent struct{ Blocks }

// UseBlockEvent is a block used without being broken: a door opened, a
// chest opened, a button pressed. WorldGuard: UseBlockEvent
type UseBlockEvent struct{ Blocks }

// Entity is the part of an entity event naming the entity. WorldGuard:
// AbstractEntityEvent
type Entity struct {
	Delegate
	World plugin.World
	// Entity is the entity; nil for one about to spawn.
	Entity plugin.Entity
	// Target is where the entity is or will be.
	Target plugin.Location
}

// SpawnEntityEvent is an entity spawned: placed, dropped, thrown.
// WorldGuard: SpawnEntityEvent
type SpawnEntityEvent struct {
	Entity
	// Type is the entity's type.
	Type registry.EntityType
}

// DestroyEntityEvent is an entity broken or picked up. WorldGuard:
// DestroyEntityEvent
type DestroyEntityEvent struct{ Entity }

// UseEntityEvent is an entity used: ridden, leashed, traded with.
// WorldGuard: UseEntityEvent
type UseEntityEvent struct{ Entity }

// DamageEntityEvent is an entity hurt. WorldGuard: DamageEntityEvent
type DamageEntityEvent struct{ Entity }

// UseItemEvent is an item used. WorldGuard: UseItemEvent
type UseItemEvent struct {
	Delegate
	World plugin.World
	Item  plugin.ItemStack
}

// DisallowedPVPEvent fires when regions deny a player hurting another;
// cancelling it lets the hurt through. WorldGuard: DisallowedPVPEvent
type DisallowedPVPEvent struct {
	plugin.Cancellable
	Attacker, Defender plugin.Player
	// Original is the server event of the hurt.
	Original plugin.Event
}

// Define makes the events known to the server, so they can be fired and
// listened to.
func Define(s plugin.Server) {
	plugin.DefineEvent[PlaceBlockEvent](s)
	plugin.DefineEvent[BreakBlockEvent](s)
	plugin.DefineEvent[UseBlockEvent](s)
	plugin.DefineEvent[SpawnEntityEvent](s)
	plugin.DefineEvent[DestroyEntityEvent](s)
	plugin.DefineEvent[UseEntityEvent](s)
	plugin.DefineEvent[DamageEntityEvent](s)
	plugin.DefineEvent[UseItemEvent](s)
	plugin.DefineEvent[DisallowedPVPEvent](s)
}

// NewBlocks answers the block part of an event at positions.
func NewBlocks(original plugin.Event, cause Cause, w plugin.World, t registry.Block, positions ...plugin.BlockPos) Blocks {
	return Blocks{Original: original, Cause: cause, World: w, Positions: positions, Type: t}
}

// NewEntity answers the entity part of an event about e.
func NewEntity(original plugin.Event, cause Cause, e plugin.Entity) Entity {
	return Entity{Original: original, Cause: cause, World: e.World(), Entity: e, Target: e.Location()}
}
