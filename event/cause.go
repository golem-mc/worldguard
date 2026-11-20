// Package event holds WorldGuard's own events: what players, mobs and
// blocks do to the world, abstracted from the server's many events into
// placing, breaking and using blocks, spawning, destroying, using and
// damaging entities and using items. WorldGuard's protection listens to
// them, and so may other plugins. WorldGuard: com.sk89q.worldguard.bukkit.event
package event

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// Cause is the chain of objects behind an event, the root first: a
// plugin.Player, a plugin.Entity or a plugin.Block. WorldGuard: Cause
type Cause struct {
	causes   []any
	indirect bool
}

// Unknown is a cause nobody is behind.
func Unknown() Cause { return Cause{} }

// NewCause builds the cause of objects, expanding each into what is behind
// it: a projectile's shooter, a primed TNT's lighter, a vehicle's riders, a
// tamed animal's online owner, a mob's target. nil objects are skipped.
// WorldGuard: Cause.create
func NewCause(s plugin.Server, objects ...any) Cause {
	b := builder{server: s, seen: map[any]bool{}}
	b.addAll(objects...)
	return Cause{causes: b.causes, indirect: b.indirect}
}

// Indirect reports whether the root did not act itself (a tamed wolf's
// owner, a mob's target), so it is not told when it is denied.
func (c Cause) Indirect() bool { return c.indirect }

// Root answers the first cause, nil for none.
func (c Cause) Root() any {
	if len(c.causes) == 0 {
		return nil
	}
	return c.causes[0]
}

// Causes answers the chain, the root first.
func (c Cause) Causes() []any { return c.causes }

// Known reports whether something is behind the event that regions can
// judge: not nothing, a tamed animal, or a TNT or vehicle whose origin is
// lost. WorldGuard: Cause.isKnown
func (c Cause) Known() bool {
	root := c.Root()
	switch r := root.(type) {
	case nil:
		return false
	case plugin.Tameable:
		return !r.Tamed()
	case plugin.TNT, plugin.Vehicle:
		return false
	}
	return true
}

// FirstPlayer answers the first player in the chain.
func (c Cause) FirstPlayer() plugin.Player {
	for _, o := range c.causes {
		if p, ok := o.(plugin.Player); ok {
			return p
		}
	}
	return nil
}

// FirstEntity answers the first entity in the chain.
func (c Cause) FirstEntity() plugin.Entity {
	for _, o := range c.causes {
		if e, ok := o.(plugin.Entity); ok {
			return e
		}
	}
	return nil
}

// FirstNonPlayerEntity answers the first entity that is no player.
func (c Cause) FirstNonPlayerEntity() plugin.Entity {
	for _, o := range c.causes {
		if _, ok := o.(plugin.Player); ok {
			continue
		}
		if e, ok := o.(plugin.Entity); ok {
			return e
		}
	}
	return nil
}

// FirstBlock answers the first block in the chain.
func (c Cause) FirstBlock() (plugin.Block, bool) {
	for _, o := range c.causes {
		if b, ok := o.(plugin.Block); ok {
			return b, true
		}
	}
	return plugin.Block{}, false
}

// Find answers the first of types among the chain's entities.
func (c Cause) Find(types ...registry.EntityType) (registry.EntityType, bool) {
	for _, o := range c.causes {
		if e, ok := o.(plugin.Entity); ok {
			for _, t := range types {
				if e.Type() == t {
					return t, true
				}
			}
		}
	}
	return 0, false
}

func (c Cause) String() string {
	parts := make([]string, len(c.causes))
	for i, o := range c.causes {
		switch x := o.(type) {
		case plugin.Player:
			parts[i] = x.Name()
		case plugin.Entity:
			parts[i] = x.Type().Name()
		case plugin.Block:
			parts[i] = fmt.Sprintf("%s@%d,%d,%d", x.State().Block().Name(), x.Pos.X, x.Pos.Y, x.Pos.Z)
		default:
			parts[i] = fmt.Sprint(o)
		}
	}
	return strings.Join(parts, " | ")
}

// parents holds the cause tracked for entities, by UUID: the player who
// angered a creeper, for one.
var parents sync.Map

// TrackParent records parent as what is behind entity, until untracked.
// WorldGuard: Cause.trackParentCause
func TrackParent(entity plugin.Entity, parent any) {
	if entity != nil && parent != nil {
		parents.Store(entity.UUID(), parent)
	}
}

// UntrackParent forgets what is behind entity.
func UntrackParent(entity plugin.Entity) {
	if entity != nil {
		parents.Delete(entity.UUID())
	}
}

type builder struct {
	server   plugin.Server
	causes   []any
	seen     map[any]bool
	indirect bool
}

// key identifies an object for the seen set: entities by UUID.
func key(o any) any {
	if e, ok := o.(plugin.Entity); ok {
		return e.UUID()
	}
	if b, ok := o.(plugin.Block); ok {
		return b
	}
	return o
}

func present(o any) bool {
	switch x := o.(type) {
	case nil:
		return false
	case plugin.Player:
		return x != nil
	case plugin.Entity:
		return x != nil
	case *plugin.Block:
		return x != nil
	}
	return true
}

func (b *builder) addAll(objects ...any) {
	for _, o := range objects {
		if p, ok := o.(*plugin.Block); ok && p != nil {
			o = *p
		}
		if !present(o) || b.seen[key(o)] {
			continue
		}
		b.seen[key(o)] = true
		b.expand(o)
		// Tracked parents go before o, the furthest first.
		index := len(b.causes)
		for src := o; ; {
			e, ok := src.(plugin.Entity)
			if !ok {
				break
			}
			v, found := parents.Load(e.UUID())
			if !found || b.seen[key(v)] {
				break
			}
			b.causes = slices.Insert(b.causes, index, v)
			b.seen[key(v)] = true
			src = v
		}
		b.causes = append(b.causes, o)
	}
}

// expand adds what is behind o before o itself.
func (b *builder) expand(o any) {
	switch x := o.(type) {
	case plugin.Player:
	case plugin.TNT:
		if src, ok := x.Source(); ok {
			b.addAll(src)
		}
	case plugin.Projectile:
		if shooter, ok := x.Shooter(); ok {
			b.addAll(shooter)
		}
	case plugin.Vehicle:
		for _, p := range x.Passengers() {
			b.addAll(p)
		}
	case plugin.Tameable:
		b.indirect = true
		if owner, ok := x.Owner(); ok && b.server != nil {
			if p, ok := b.server.Player(owner); ok {
				b.addAll(p)
			}
		}
	case plugin.Mob:
		if x.Type() == registry.EntityTypeAreaEffectCloud {
			b.indirect = true
			return
		}
		if !targetsAsCreature(x) {
			return
		}
		b.indirect = true
		if t, ok := x.Target(); ok {
			b.addAll(t)
		}
	case plugin.Entity:
		if x.Type() == registry.EntityTypeAreaEffectCloud {
			b.indirect = true
		}
	}
}

// targetsAsCreature reports whether a mob's target stands behind what it
// does: a creeper, or a pathfinding mob (Bukkit's Creature), not a slime,
// ghast, phantom or the dragon.
func targetsAsCreature(m plugin.Mob) bool {
	switch m.Type() {
	case registry.EntityTypeSlime, registry.EntityTypeMagmaCube, registry.EntityTypeGhast,
		registry.EntityTypePhantom, registry.EntityTypeEnderDragon, registry.EntityTypeBat:
		return false
	}
	return true
}
