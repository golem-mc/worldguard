package worldguardplugin

import (
	"slices"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/event"
)

// registerRegionFlags applies the flags that limit what blocks and mobs do
// by themselves: pistons, creepers, endermen. WorldGuard: RegionFlagsListener
func (p *protection) registerRegionFlags() {
	s := p.wg.Server
	early := plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true}
	plugin.Listen(s, func(e *event.PlaceBlockEvent) {
		if !p.regionsOn(e.World) {
			return
		}
		if b, ok := e.Cause.FirstBlock(); ok && isPistonBlock(b.State().Block()) {
			e.Filter(p.testState(worldguard.FlagPistons), false)
		}
		if _, ok := e.Cause.Find(registry.EntityTypeSnowGolem); ok {
			e.Filter(p.testState(worldguard.FlagSnowmanTrails), false)
		}
		if _, ok := e.Cause.Find(registry.EntityTypeEnderman); ok {
			e.Filter(p.testState(worldguard.FlagEndermanGrief), false)
		}
	}, early)
	plugin.Listen(s, func(e *event.BreakBlockEvent) {
		if !p.regionsOn(e.World) {
			return
		}
		cancel := p.cfg(e.World).ExplosionFlagCancellation
		if b, ok := e.Cause.FirstBlock(); ok && isPistonBlock(b.State().Block()) {
			e.Filter(p.testState(worldguard.FlagPistons), false)
		}
		for _, f := range []struct {
			t      registry.EntityType
			flag   *worldguard.StateFlag
			cancel bool
		}{
			{registry.EntityTypeCreeper, worldguard.FlagCreeperExplosion, cancel},
			{registry.EntityTypeEnderDragon, worldguard.FlagEnderDragonBlockDamage, cancel},
			{registry.EntityTypeEndCrystal, worldguard.FlagOtherExplosion, cancel},
			{registry.EntityTypeEnderman, worldguard.FlagEndermanGrief, false},
			{registry.EntityTypeRavager, worldguard.FlagRavagerGrief, false},
		} {
			if _, ok := e.Cause.Find(f.t); ok {
				e.Filter(p.testState(f.flag), f.cancel)
			}
		}
	}, early)
	plugin.Listen(s, func(e *plugin.EntityDamageEvent) {
		pl, ok := e.Entity.(plugin.Player)
		if !ok || !p.regionsOn(pl.World()) {
			return
		}
		if isDamage(e.Source, "fall", "fly_into_wall") &&
			!p.query().TestState(pl.Location(), p.wg.WrapPlayer(pl), worldguard.FlagFallDamage) {
			e.SetCancelled(true)
		}
	}, early)
}

func (p *protection) testState(f *worldguard.StateFlag) func(plugin.Location) bool {
	return func(loc plugin.Location) bool { return p.query().TestState(loc, nil, f) }
}

// isDamage reports whether a damage source is of one of the named vanilla
// damage types.
func isDamage(src plugin.DamageSource, names ...string) bool {
	n := shortName(src.Type.Name())
	return slices.Contains(names, n)
}

// registerBuildPermissions requires worldguard.build.* nodes for what
// players do, when build-permission-nodes is on. WorldGuard: BuildPermissionListener
func (p *protection) registerBuildPermissions() {
	s := p.wg.Server
	skip := plugin.ListenOptions{Priority: plugin.Normal, IgnoreCancelled: true}
	check := func(d *event.Delegate, w plugin.World, kind, name, action string) {
		pl, ok := d.Cause.Root().(plugin.Player)
		if !ok || !p.cfg(w).BuildPermissions {
			return
		}
		if !p.buildPerm(pl, kind+"."+name+"."+action) && !p.buildPerm(pl, kind+"."+action+"."+name) {
			p.tellBuildDenied(pl, w)
			d.SetCancelled(true)
		}
	}
	plugin.Listen(s, func(e *event.PlaceBlockEvent) {
		check(&e.Delegate, e.World, "block", shortName(e.Type.Name()), "place")
	}, skip)
	plugin.Listen(s, func(e *event.BreakBlockEvent) {
		check(&e.Delegate, e.World, "block", shortName(e.Type.Name()), "remove")
	}, skip)
	plugin.Listen(s, func(e *event.UseBlockEvent) {
		pl, ok := e.Cause.Root().(plugin.Player)
		if !ok || !p.cfg(e.World).BuildPermissions {
			return
		}
		blocked := e.Filter(func(loc plugin.Location) bool {
			name := shortName(loc.World.BlockState(loc.Pos.Block()).Block().Name())
			return p.buildPerm(pl, "block."+name+".interact") || p.buildPerm(pl, "block.interact."+name)
		}, true)
		if blocked && !e.Silent {
			p.tellBuildDenied(pl, e.World)
		}
	}, skip)
	plugin.Listen(s, func(e *event.SpawnEntityEvent) {
		check(&e.Delegate, e.World, "entity", shortName(e.Type.Name()), "place")
	}, skip)
	plugin.Listen(s, func(e *event.DestroyEntityEvent) {
		check(&e.Delegate, e.World, "entity", shortName(e.Entity.Entity.Type().Name()), "remove")
	}, skip)
	plugin.Listen(s, func(e *event.UseEntityEvent) {
		check(&e.Delegate, e.World, "entity", shortName(e.Entity.Entity.Type().Name()), "interact")
	}, skip)
	plugin.Listen(s, func(e *event.DamageEntityEvent) {
		check(&e.Delegate, e.World, "entity", shortName(e.Entity.Entity.Type().Name()), "damage")
	}, skip)
	plugin.Listen(s, func(e *event.UseItemEvent) {
		if isBlockItem(e.Item.Item) {
			return
		}
		check(&e.Delegate, e.World, "item", shortName(e.Item.Item.Name()), "use")
	}, skip)
}

func (p *protection) buildPerm(pl plugin.Player, node string) bool {
	return p.wg.HasPermission(pl, "worldguard.build."+node)
}

func (p *protection) tellBuildDenied(pl plugin.Player, w plugin.World) {
	if msg := p.cfg(w).BuildPermissionDenyMessage; msg != "" {
		tell(pl, msg)
	}
}
