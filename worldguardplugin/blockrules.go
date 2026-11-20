package worldguardplugin

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard"
)

// registerBlockRules applies the world settings and flags for what blocks
// do by themselves: fire, liquids, growth, melting. WorldGuard: WorldGuardBlockListener
func (p *protection) registerBlockRules() {
	s := p.wg.Server
	high := plugin.ListenOptions{Priority: plugin.High, IgnoreCancelled: true}
	normal := plugin.ListenOptions{Priority: plugin.Normal, IgnoreCancelled: true}
	plugin.Listen(s, p.keepDurability, high)
	plugin.Listen(s, p.ruleFromTo, normal)
	plugin.Listen(s, p.ruleIgnite, high)
	plugin.Listen(s, p.ruleBurn, high)
	plugin.Listen(s, p.rulePhysics, normal)
	plugin.Listen(s, p.ruleSpongePlace, high)
	plugin.Listen(s, p.ruleLeavesDecay, high)
	plugin.Listen(s, p.ruleForm, high)
	plugin.Listen(s, p.ruleSpread, high)
	plugin.Listen(s, func(e *plugin.BlockGrowEvent) { p.ruleGrow(e, e.Block, e.NewState.Block()) }, high)
	plugin.Listen(s, p.ruleFade, high)
	plugin.Listen(s, func(e *plugin.BlockExplodeEvent) {
		if p.wg.Config.ActivityHaltToggle || p.cfg(e.Block.World).BlockOtherExplosions {
			e.SetCancelled(true)
		}
	}, normal)
}

// keepDurability repairs the tool that broke a block when item-durability
// is off. WorldGuard: WorldGuardBlockListener.onBlockBreak
func (p *protection) keepDurability(e *plugin.BlockBreakEvent) {
	if p.cfg(e.Player.World()).ItemDurability {
		return
	}
	held := e.Player.Equipment().Get(plugin.SlotMainHand)
	if held.Empty() || held.Patch.Damage == nil || *held.Patch.Damage == 0 {
		return
	}
	held.Patch.Damage = new(0)
	e.Player.Equipment().Set(plugin.SlotMainHand, held)
}

// stateDenied reports whether regions are on in w and f's state at b
// does not allow.
func (p *protection) stateDenied(b plugin.Block, f *worldguard.StateFlag) bool {
	return p.cfg(b.World).UseRegions && !worldguard.TestStates(p.query().QueryState(blockLoc(b), nil, f))
}

func (p *protection) ruleFromTo(e *plugin.BlockFromToEvent) {
	if p.wg.Config.ActivityHaltToggle {
		e.SetCancelled(true)
		return
	}
	cfg := p.cfg(e.From.World)
	fs := e.From.State()
	from := fs.Block()
	water, lava, air := isWater(from), from == registry.BlockLava, isAir(from)
	if cfg.SimulateSponge && water && p.spongeNear(e.To, cfg.SpongeRadius, cfg.RedstoneSponges) {
		e.SetCancelled(true)
		return
	}
	if len(cfg.PreventWaterDamage) > 0 && (air || water) && cfg.PreventWaterDamage[e.To.State().Block()] {
		e.SetCancelled(true)
		return
	}
	if len(cfg.AllowedLavaSpreadOver) > 0 && lava && !cfg.AllowedLavaSpreadOver[e.To.Relative(plugin.Down, 1).State().Block()] {
		e.SetCancelled(true)
		return
	}
	if cfg.HighFreqFlags && (water || waterloggable(fs)) &&
		p.query().QueryState(blockLoc(e.From), nil, worldguard.FlagWaterFlow) == worldguard.StateDeny {
		e.SetCancelled(true)
		return
	}
	if cfg.HighFreqFlags && lava && !worldguard.TestStates(p.query().QueryState(blockLoc(e.From), nil, worldguard.FlagLavaFlow)) {
		e.SetCancelled(true)
	}
}

// spongeNear reports whether a sponge is within r blocks of b; with
// redstone, a powered sponge does not count.
// WorldGuard: WorldGuardBlockListener.onBlockFromTo
func (p *protection) spongeNear(b plugin.Block, r int, redstone bool) bool {
	for dx := -r; dx <= r; dx++ {
		for dy := -r; dy <= r; dy++ {
			for dz := -r; dz <= r; dz++ {
				pos := plugin.BlockPos{X: b.Pos.X + dx, Y: b.Pos.Y + dy, Z: b.Pos.Z + dz}
				if b.World.BlockState(pos).Block() == registry.BlockSponge && (!redstone || b.World.Power(pos) == 0) {
					return true
				}
			}
		}
	}
	return false
}

func (p *protection) ruleIgnite(e *plugin.BlockIgniteEvent) {
	if p.wg.Config.ActivityHaltToggle {
		e.SetCancelled(true)
		return
	}
	cfg := p.cfg(e.Block.World)
	spread := e.Cause == plugin.IgniteSpread
	switch {
	case cfg.PreventLightningFire && e.Cause == plugin.IgniteLightning,
		cfg.PreventLavaFire && e.Cause == plugin.IgniteLava,
		cfg.DisableFireSpread && spread,
		cfg.BlockLighter && (e.Cause == plugin.IgniteFlintAndSteel || e.Cause == plugin.IgniteFireball) &&
			e.Player != nil && !p.wg.HasPermission(e.Player, "worldguard.override.lighter"),
		cfg.FireSpreadDisableToggle && spread:
		e.SetCancelled(true)
		return
	}
	if len(cfg.DisableFireSpreadBlocks) > 0 && spread {
		for _, f := range []plugin.BlockFace{plugin.Down, plugin.East, plugin.West, plugin.North, plugin.South} {
			if cfg.DisableFireSpreadBlocks[e.Block.Relative(f, 1).State().Block()] {
				e.SetCancelled(true)
				return
			}
		}
	}
	if !cfg.UseRegions {
		return
	}
	set := p.query().ApplicableRegions(blockLoc(e.Block))
	switch {
	case cfg.HighFreqFlags && spread && !set.TestState(nil, worldguard.FlagFireSpread),
		cfg.HighFreqFlags && e.Cause == plugin.IgniteLava && !set.TestState(nil, worldguard.FlagLavaFire),
		e.Cause == plugin.IgniteFireball && e.Player == nil && !set.TestState(nil, worldguard.FlagGhastFireball),
		e.Cause == plugin.IgniteLightning && !set.TestState(nil, worldguard.FlagLightning):
		e.SetCancelled(true)
	}
}

func (p *protection) ruleBurn(e *plugin.BlockBurnEvent) {
	if p.wg.Config.ActivityHaltToggle {
		e.SetCancelled(true)
		return
	}
	cfg := p.cfg(e.Block.World)
	switch {
	case cfg.DisableFireSpread:
		e.SetCancelled(true)
	case cfg.FireSpreadDisableToggle,
		len(cfg.DisableFireSpreadBlocks) > 0 && cfg.DisableFireSpreadBlocks[e.Block.State().Block()]:
		e.SetCancelled(true)
		putOutFireAround(e.Block)
	case cfg.UseRegions && !p.query().ApplicableRegions(blockLoc(e.Block)).TestState(nil, worldguard.FlagFireSpread):
		putOutFireAround(e.Block)
		e.SetCancelled(true)
	}
}

// putOutFireAround removes the fire next to b. WorldGuard:
// checkAndDestroyFireAround
func putOutFireAround(b plugin.Block) {
	for _, f := range sixFaces {
		if n := b.Relative(f, 1); isFire(n.State().Block()) {
			n.Set(plugin.DefaultState(registry.BlockAir))
		}
	}
}

func (p *protection) rulePhysics(e *plugin.BlockPhysicsEvent) {
	if p.wg.Config.ActivityHaltToggle {
		e.SetCancelled(true)
		return
	}
	cfg := p.cfg(e.Block.World)
	switch id := e.State.Block(); {
	case id == registry.BlockGravel && cfg.NoPhysicsGravel,
		(id == registry.BlockSand || id == registry.BlockRedSand) && cfg.NoPhysicsSand,
		id == registry.BlockNetherPortal && cfg.AllowPortalAnywhere,
		id == registry.BlockLadder && cfg.RopeLadders && e.Block.Relative(plugin.Up, 1).State().Block() == registry.BlockLadder:
		e.SetCancelled(true)
	}
}

// ruleSpongePlace dries the water around a placed sponge, when
// simulate-sponge is on. WorldGuard: WorldGuardBlockListener.onBlockPlace
func (p *protection) ruleSpongePlace(e *plugin.BlockPlaceEvent) {
	cfg := p.cfg(e.Block.World)
	if !cfg.SimulateSponge || e.Placed.Block() != registry.BlockSponge {
		return
	}
	r := cfg.SpongeRadius
	for dx := -r; dx <= r; dx++ {
		for dy := -r; dy <= r; dy++ {
			for dz := -r; dz <= r; dz++ {
				pos := plugin.BlockPos{X: e.Block.Pos.X + dx, Y: e.Block.Pos.Y + dy, Z: e.Block.Pos.Z + dz}
				s := e.Block.World.BlockState(pos)
				switch s.Block() {
				case registry.BlockWater, registry.BlockSeagrass, registry.BlockTallSeagrass, registry.BlockKelpPlant, registry.BlockKelp:
					e.Block.World.SetBlockState(pos, plugin.DefaultState(registry.BlockAir), plugin.UpdateAll)
				default:
					if dry, ok := s.With(registry.PropWaterlogged, "false"); ok && dry != s {
						e.Block.World.SetBlockState(pos, dry, plugin.UpdateAll)
					}
				}
			}
		}
	}
}

func (p *protection) ruleLeavesDecay(e *plugin.LeavesDecayEvent) {
	if p.wg.Config.ActivityHaltToggle || p.cfg(e.Block.World).DisableLeafDecay || p.stateDenied(e.Block, worldguard.FlagLeafDecay) {
		e.SetCancelled(true)
	}
}

func (p *protection) ruleForm(e *plugin.BlockFormEvent) {
	if p.wg.Config.ActivityHaltToggle {
		e.SetCancelled(true)
		return
	}
	cfg := p.cfg(e.Block.World)
	if e.Entity != nil {
		if e.Entity.Type() == registry.EntityTypeSnowGolem && cfg.DisableSnowmanTrails {
			e.SetCancelled(true)
		}
		return
	}
	old, nu := e.Block.State().Block(), e.NewState.Block()
	deny := func(off bool, f *worldguard.StateFlag) bool {
		if off || p.stateDenied(e.Block, f) {
			e.SetCancelled(true)
			return true
		}
		return false
	}
	if nu == registry.BlockIce && deny(cfg.DisableIceFormation, worldguard.FlagIceForm) {
		return
	}
	if nu == registry.BlockSnow {
		if cfg.DisableSnowFormation || len(cfg.AllowedSnowFallOver) > 0 && !cfg.AllowedSnowFallOver[e.Block.Relative(plugin.Down, 1).State().Block()] {
			e.SetCancelled(true)
			return
		}
		if deny(false, worldguard.FlagSnowFall) {
			return
		}
	}
	if isUnwaxedCopper(old) && deny(cfg.DisableCopperBlockFade, worldguard.FlagCopperFade) {
		return
	}
	if old == registry.BlockLava && !isLiquid(nu) {
		deny(cfg.DisableLavaHarden, worldguard.FlagLavaHarden)
	}
}

func (p *protection) ruleSpread(e *plugin.BlockSpreadEvent) {
	if p.wg.Config.ActivityHaltToggle {
		e.SetCancelled(true)
		return
	}
	cfg := p.cfg(e.Block.World)
	nu := e.NewState.Block()
	for _, r := range []struct {
		match bool
		off   bool
		flag  *worldguard.StateFlag
	}{
		{isMushroom(nu), cfg.DisableMushroomSpread, worldguard.FlagMushroomGrowth},
		{nu == registry.BlockGrassBlock, cfg.DisableGrassGrowth, worldguard.FlagGrassGrowth},
		{nu == registry.BlockMycelium, cfg.DisableMyceliumSpread, worldguard.FlagMyceliumSpread},
		{isVine(nu), cfg.DisableVineGrowth, worldguard.FlagVineGrowth},
		{isAmethystGrowth(nu) || nu == registry.BlockPointedDripstone, cfg.DisableRockGrowth, worldguard.FlagRockGrowth},
		{isSculkGrowth(nu), cfg.DisableSculkGrowth, worldguard.FlagSculkGrowth},
	} {
		if r.match && (r.off || p.stateDenied(e.Block, r.flag)) {
			e.SetCancelled(true)
			return
		}
	}
	p.ruleGrow(e, e.Block, nu)
}

// ruleGrow stops crops growing where crop growth is off. WorldGuard: handleGrow
func (p *protection) ruleGrow(e interface{ SetCancelled(bool) }, b plugin.Block, t registry.Block) {
	if isCrop(t) && (p.cfg(b.World).DisableCropGrowth || p.stateDenied(b, worldguard.FlagCropGrowth)) {
		e.SetCancelled(true)
	}
}

func (p *protection) ruleFade(e *plugin.BlockFadeEvent) {
	cfg := p.cfg(e.Block.World)
	var off bool
	var flag *worldguard.StateFlag
	switch t := e.Block.State().Block(); {
	case t == registry.BlockIce:
		off, flag = cfg.DisableIceMelting, worldguard.FlagIceMelt
	case t == registry.BlockFrostedIce:
		flag = worldguard.FlagFrostedIceMelt
	case t == registry.BlockSnow:
		off, flag = cfg.DisableSnowMelting, worldguard.FlagSnowMelt
	case t == registry.BlockFarmland:
		if cfg.DisableSoilDehydration || p.stateDenied(e.Block, worldguard.FlagSoilDry) {
			e.SetCancelled(true)
		}
		return
	case isCoral(t):
		off, flag = cfg.DisableCoralBlockFade, worldguard.FlagCoralFade
	default:
		return
	}
	if off || p.stateDenied(e.Block, flag) {
		e.SetCancelled(true)
	}
}
