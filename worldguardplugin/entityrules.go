package worldguardplugin

import (
	"math"
	"slices"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/event"
)

// registerEntityRules applies the world settings and flags for what mobs,
// explosions and damage do. WorldGuard: WorldGuardEntityListener,
// WorldGuardHangingListener
func (p *protection) registerEntityRules() {
	s := p.wg.Server
	high := plugin.ListenOptions{Priority: plugin.High, IgnoreCancelled: true}
	plugin.Listen(s, func(e *plugin.EntityInteractEvent) {
		if e.Block.State().Block() == registry.BlockFarmland && p.cfg(e.Block.World).DisableCreatureCropTrampling {
			e.SetCancelled(true)
		}
	}, high)
	plugin.Listen(s, func(e *plugin.PlayerDeathEvent) {
		if p.cfg(e.Player.World()).DisableDeathMessages {
			e.Message = nil
		}
	}, plugin.ListenOptions{Priority: plugin.High})
	plugin.Listen(s, p.ruleDamage, high)
	plugin.Listen(s, p.ruleKnockback, high)
	plugin.Listen(s, p.ruleExplode, high)
	plugin.Listen(s, p.rulePrime, high)
	plugin.Listen(s, p.ruleCreatureSpawn, high)
	plugin.Listen(s, p.rulePortalCreate, high)
	plugin.Listen(s, func(e *plugin.EntityTransformEvent) {
		cfg := p.cfg(e.Entity.World())
		if e.Reason != plugin.TransformLightning {
			return
		}
		if e.Entity.Type() == registry.EntityTypeVillager && cfg.DisableVillagerZap ||
			e.Entity.Type() == registry.EntityTypePig && cfg.DisablePigZap {
			e.SetCancelled(true)
		}
	}, high)
	plugin.Listen(s, func(e *plugin.CreeperPowerEvent) {
		if p.cfg(e.Creeper.World()).DisableCreeperPower {
			e.SetCancelled(true)
		}
	}, high)
	plugin.Listen(s, p.ruleRegain, high)
	plugin.Listen(s, p.ruleFood, plugin.ListenOptions{Priority: plugin.Normal, IgnoreCancelled: true})
	plugin.Listen(s, p.ruleChangeBlock, high)
	plugin.Listen(s, func(e *plugin.EntityBreakDoorEvent) {
		if p.cfg(e.Block.World).BlockZombieDoorDestruction {
			e.SetCancelled(true)
		}
	}, high)
	plugin.Listen(s, func(e *plugin.VehicleEvent) {
		if e.Action != plugin.VehicleEnter || e.Entity == nil {
			return
		}
		if _, ok := e.Entity.(plugin.Player); !ok && p.cfg(e.Entity.World()).BlockEntityVehicleEntry {
			e.SetCancelled(true)
		}
	}, high)
	plugin.Listen(s, p.ruleHangingBreak, high)
}

// damageKind sorts a damage source into Bukkit's damage causes this
// listener tells apart.
func damageKind(src plugin.DamageSource) string {
	switch shortName(src.Type.Name()) {
	case "lava":
		return "lava"
	case "cactus", "sweet_berry_bush":
		return "contact"
	case "out_of_world":
		return "void"
	case "bad_respawn_point":
		return "block_explosion"
	case "explosion", "player_explosion":
		if src.Causing == nil && src.Direct == nil {
			return "block_explosion"
		}
		return "entity_explosion"
	case "fall":
		return "fall"
	case "in_fire", "on_fire", "campfire":
		return "fire"
	case "drown":
		return "drowning"
	case "in_wall":
		return "suffocation"
	case "lightning_bolt":
		return "lightning"
	case "wither":
		return "wither"
	}
	return ""
}

// ruleDamage applies the damage settings. WorldGuard:
// WorldGuardEntityListener.onEntityDamage
func (p *protection) ruleDamage(e *plugin.EntityDamageEvent) {
	switch {
	case e.Source.Direct != nil:
		if _, ok := e.Source.Direct.(plugin.Projectile); ok {
			p.damageByProjectile(e)
			return
		}
		p.damageByEntity(e, e.Source.Direct)
		return
	case e.Source.Causing != nil:
		p.damageByEntity(e, e.Source.Causing)
		return
	}
	kind := damageKind(e.Source)
	cfg := p.cfg(e.Entity.World())
	if e.Entity.Type() == registry.EntityTypeWolf && isTamed(e.Entity) {
		if cfg.AntiWolfDumbness && kind != "void" {
			e.SetCancelled(true)
		}
		return
	}
	pl, ok := e.Entity.(plugin.Player)
	if !ok {
		if kind == "block_explosion" && (cfg.BlockOtherExplosions ||
			(cfg.ExplosionFlagCancellation || buildingIfUsed(e.Entity)) &&
				!worldguard.TestStates(p.query().QueryState(e.Entity.Location(), nil, worldguard.FlagOtherExplosion))) {
			e.SetCancelled(true)
		}
		return
	}
	lp := p.wg.WrapPlayer(pl)
	switch kind {
	case "lava":
		if cfg.DisableLavaDamage {
			e.SetCancelled(true)
			pl.SetFireTicks(0)
			return
		}
	case "contact":
		if cfg.DisableContactDamage {
			e.SetCancelled(true)
			return
		}
	case "void":
		if cfg.TeleportOnVoid {
			p.findFreePosition(pl)
			if cfg.SafeFallOnVoid {
				pl.SetFallDistance(0)
			}
			e.SetCancelled(true)
			return
		}
		if cfg.DisableVoidDamage {
			e.SetCancelled(true)
			return
		}
	case "block_explosion":
		if cfg.DisableExplosionDamage || cfg.BlockOtherExplosions || cfg.ExplosionFlagCancellation &&
			!worldguard.TestStates(p.query().QueryState(pl.Location(), nil, worldguard.FlagOtherExplosion)) {
			e.SetCancelled(true)
			return
		}
	case "wither":
		if cfg.DisableMobDamage || cfg.UseRegions && !p.query().ApplicableRegions(pl.Location()).TestState(lp, worldguard.FlagMobDamage) {
			e.SetCancelled(true)
			return
		}
	}
	helmet := pl.Equipment().Get(plugin.SlotHead).Item
	switch {
	case kind == "drowning" && cfg.PumpkinScuba && (helmet == registry.ItemCarvedPumpkin || helmet == registry.ItemJackOLantern),
		kind == "drowning" && cfg.DisableDrowningDamage:
		pl.SetAir(maxAir)
		e.SetCancelled(true)
	case cfg.DisableFallDamage && kind == "fall",
		cfg.DisableFireDamage && kind == "fire",
		cfg.DisableSuffocationDamage && kind == "suffocation":
		e.SetCancelled(true)
	case cfg.TeleportOnSuffocation && kind == "suffocation":
		p.findFreePosition(pl)
		e.SetCancelled(true)
	}
}

// maxAir is a player's full breath, in ticks.
const maxAir = 300

// damageByEntity applies the settings for a hurt dealt by an entity's own
// blow or blast. WorldGuard: onEntityDamageByEntity
func (p *protection) damageByEntity(e *plugin.EntityDamageEvent, attacker plugin.Entity) {
	cfg := p.cfg(e.Entity.World())
	_, attackerIsPlayer := attacker.(plugin.Player)
	switch e.Entity.Type() {
	case registry.EntityTypeItemFrame, registry.EntityTypeGlowItemFrame:
		if p.itemFrameProtected(attacker, e.Entity) {
			e.SetCancelled(true)
			return
		}
	case registry.EntityTypeArmorStand:
		if !attackerIsPlayer && cfg.BlockEntityArmorStandDestroy {
			e.SetCancelled(true)
			return
		}
	}
	if attacker.Type() == registry.EntityTypeEndCrystal && cfg.UseRegions && cfg.ExplosionFlagCancellation &&
		!p.query().ApplicableRegions(e.Entity.Location()).TestState(nil, worldguard.FlagOtherExplosion) {
		e.SetCancelled(true)
		return
	}
	pl, ok := e.Entity.(plugin.Player)
	if !ok {
		return
	}
	kind := damageKind(e.Source)
	if cfg.DisableLightningDamage && kind == "lightning" ||
		cfg.DisableExplosionDamage && (kind == "block_explosion" || kind == "entity_explosion") ||
		isTNTBased(attacker) && cfg.BlockTNTExplosions {
		e.SetCancelled(true)
		return
	}
	if _, living := attacker.(plugin.LivingEntity); !living || attackerIsPlayer {
		return
	}
	if attacker.Type() == registry.EntityTypeCreeper && cfg.BlockCreeperExplosions || cfg.DisableMobDamage {
		e.SetCancelled(true)
		return
	}
	if _, tame := attacker.(plugin.Tameable); cfg.UseRegions && !tame &&
		!p.query().ApplicableRegions(pl.Location()).TestState(p.wg.WrapPlayer(pl), worldguard.FlagMobDamage) {
		e.SetCancelled(true)
	}
}

// damageByProjectile applies the settings for a hurt dealt by a mob's
// projectile. WorldGuard: onEntityDamageByProjectile
func (p *protection) damageByProjectile(e *plugin.EntityDamageEvent) {
	proj := e.Source.Direct
	shooter, ok := proj.(plugin.Projectile).Shooter()
	if !ok || shooter == nil {
		return
	}
	if _, living := shooter.(plugin.LivingEntity); !living {
		return
	}
	cfg := p.cfg(e.Entity.World())
	_, shooterIsPlayer := shooter.(plugin.Player)
	if pl, ok := e.Entity.(plugin.Player); ok {
		if shooterIsPlayer {
			return
		}
		lp := p.wg.WrapPlayer(pl)
		if cfg.DisableMobDamage || cfg.UseRegions && !p.query().ApplicableRegions(pl.Location()).TestState(lp, worldguard.FlagMobDamage) {
			e.SetCancelled(true)
			return
		}
		t := proj.Type()
		if !isFireballType(t) && t != registry.EntityTypeWitherSkull && t != registry.EntityTypeWindCharge &&
			t != registry.EntityTypeBreezeWindCharge && t != registry.EntityTypeDragonFireball {
			return
		}
		switch {
		case t == registry.EntityTypeWitherSkull && cfg.BlockWitherSkullExplosions,
			(t == registry.EntityTypeWindCharge || t == registry.EntityTypeBreezeWindCharge) && cfg.BlockWindChargeExplosions,
			isFireballType(t) && cfg.BlockFireballExplosions:
			e.SetCancelled(true)
			return
		}
		if cfg.UseRegions && cfg.ExplosionFlagCancellation && !p.query().TestState(pl.Location(), lp, explosionFlag(t)) {
			e.SetCancelled(true)
		}
		return
	}
	switch e.Entity.Type() {
	case registry.EntityTypeItemFrame, registry.EntityTypeGlowItemFrame:
		if p.itemFrameProtected(shooter, e.Entity) {
			e.SetCancelled(true)
		}
	case registry.EntityTypeArmorStand:
		if isNonPlayerCreature(shooter) && cfg.BlockEntityArmorStandDestroy {
			e.SetCancelled(true)
		}
	}
}

// itemFrameProtected reports whether a non-player may not break an item
// frame. WorldGuard: checkItemFrameProtection
func (p *protection) itemFrameProtected(attacker, frame plugin.Entity) bool {
	if _, ok := attacker.(plugin.Player); ok {
		return false
	}
	cfg := p.cfg(frame.World())
	if cfg.UseRegions && !worldguard.TestStates(p.query().QueryState(frame.Location(), nil, worldguard.FlagEntityItemFrameDestroy)) {
		return true
	}
	return cfg.BlockEntityItemFrameDestroy
}

// findFreePosition moves a player up to the first place they fit.
// WorldEdit: Player.findFreePosition
func (p *protection) findFreePosition(pl plugin.Player) {
	w := pl.World()
	pos := pl.Pos()
	x, z := int(math.Floor(pos.X)), int(math.Floor(pos.Z))
	y := max(int(math.Floor(pos.Y)), w.MinY())
	top := w.MinY() + w.Height()
	free := func(y int) bool {
		return w.BlockState(plugin.BlockPos{X: x, Y: y, Z: z}).Block().NoCollision() &&
			w.BlockState(plugin.BlockPos{X: x, Y: y + 1, Z: z}).Block().NoCollision()
	}
	for ; y < top; y++ {
		below := w.BlockState(plugin.BlockPos{X: x, Y: y - 1, Z: z}).Block()
		if free(y) && (y == w.MinY() || !below.NoCollision()) {
			loc := pl.Location()
			loc.Pos = plugin.Vec3{X: float64(x) + .5, Y: float64(y), Z: float64(z) + .5}
			pl.Teleport(loc, plugin.TeleportPlugin)
			return
		}
	}
}

// ruleExplode applies the explosion settings and flags. WorldGuard:
// WorldGuardEntityListener.onEntityExplode
func (p *protection) ruleExplode(e *plugin.EntityExplodeEvent) {
	if p.wg.Config.ActivityHaltToggle {
		if e.Entity != nil {
			e.Entity.Remove()
		}
		e.SetCancelled(true)
		return
	}
	w := e.Location.World
	if w == nil {
		return
	}
	cfg := p.cfg(w)
	clearOr := func(cancel, clearBlocks bool) bool {
		if cancel {
			e.SetCancelled(true)
			return true
		}
		if clearBlocks {
			e.Blocks = nil
			return true
		}
		return false
	}
	flagDenies := func(f *worldguard.StateFlag, cancel bool) bool {
		if !cfg.UseRegions {
			return false
		}
		for _, b := range e.Blocks {
			if !p.query().TestState(blockLoc(b), nil, f) {
				e.Blocks = nil
				if cancel {
					e.SetCancelled(true)
				}
				return true
			}
		}
		return false
	}
	switch t := e.Type; {
	case t == registry.EntityTypeCreeper:
		if clearOr(cfg.BlockCreeperExplosions, cfg.BlockCreeperBlockDamage) {
			return
		}
	case t == registry.EntityTypeEnderDragon:
		if clearOr(false, cfg.BlockEnderDragonBlockDamage) {
			return
		}
	case t == registry.EntityTypeTnt || t == registry.EntityTypeTntMinecart:
		if clearOr(cfg.BlockTNTExplosions, cfg.BlockTNTBlockDamage) {
			return
		}
	case t == registry.EntityTypeWitherSkull:
		if clearOr(cfg.BlockWitherSkullExplosions, cfg.BlockWitherSkullBlockDamage) ||
			flagDenies(explosionFlag(t), cfg.ExplosionFlagCancellation) {
			return
		}
	case t == registry.EntityTypeWindCharge || t == registry.EntityTypeBreezeWindCharge:
		if clearOr(cfg.BlockWindChargeExplosions, false) {
			return
		}
		if t == registry.EntityTypeBreezeWindCharge && flagDenies(explosionFlag(t), cfg.ExplosionFlagCancellation) {
			return
		}
	case isFireballType(t) || t == registry.EntityTypeDragonFireball:
		if clearOr(cfg.BlockFireballExplosions, cfg.BlockFireballBlockDamage) ||
			flagDenies(explosionFlag(t), cfg.ExplosionFlagCancellation) {
			return
		}
	case t == registry.EntityTypeWither:
		if clearOr(cfg.BlockWitherExplosions, cfg.BlockWitherBlockDamage) || flagDenies(worldguard.FlagWitherDamage, true) {
			return
		}
	default:
		if clearOr(cfg.BlockOtherExplosions, false) || flagDenies(worldguard.FlagOtherExplosion, cfg.ExplosionFlagCancellation) {
			return
		}
	}
}

// rulePrime stops explosions before they happen, and remembers the
// player a creeper was after. WorldGuard: onExplosionPrime
func (p *protection) rulePrime(e *plugin.ExplosionPrimeEvent) {
	if p.wg.Config.ActivityHaltToggle {
		if e.Entity != nil {
			e.Entity.Remove()
		}
		e.SetCancelled(true)
		return
	}
	if e.Location.World == nil {
		return
	}
	cfg := p.cfg(e.Location.World)
	switch t := e.Type; {
	case t == registry.EntityTypeWither && cfg.BlockWitherExplosions,
		t == registry.EntityTypeWitherSkull && cfg.BlockWitherSkullExplosions,
		t == registry.EntityTypeFireball && cfg.BlockFireballExplosions,
		t == registry.EntityTypeCreeper && cfg.BlockCreeperExplosions,
		(t == registry.EntityTypeTnt || t == registry.EntityTypeTntMinecart) && cfg.BlockTNTExplosions,
		(t == registry.EntityTypeWindCharge || t == registry.EntityTypeBreezeWindCharge) && cfg.BlockWindChargeExplosions:
		e.SetCancelled(true)
	case t == registry.EntityTypeCreeper:
		if m, ok := e.Entity.(plugin.Mob); ok {
			if target, ok := m.Target(); ok {
				if pl, ok := target.(plugin.Player); ok {
					event.TrackParent(m, pl)
				}
			}
		}
	}
}

// ruleCreatureSpawn applies the spawning settings and flags. WorldGuard:
// onCreatureSpawn, handleCreatureSpawn
func (p *protection) ruleCreatureSpawn(e *plugin.EntitySpawnEvent) {
	if _, ok := e.Entity.(plugin.LivingEntity); !ok {
		return
	}
	if _, ok := e.Entity.(plugin.Player); ok {
		return
	}
	if p.wg.Config.ActivityHaltToggle {
		e.SetCancelled(true)
		return
	}
	cfg := p.cfg(e.Entity.World())
	if !cfg.BlockPluginSpawning && (e.Reason == plugin.SpawnPlugin || e.Reason == plugin.SpawnCommand) {
		return
	}
	if buildingIfUsed(e.Entity) || cfg.AllowTamedSpawns && isTamed(e.Entity) {
		return
	}
	t := e.Entity.Type()
	loc := e.Entity.Location()
	if cfg.BlockCreatureSpawn[t] {
		e.SetCancelled(true)
		return
	}
	if cfg.UseRegions && p.wg.Config.UseRegionsCreatureSpawn {
		set := p.query().ApplicableRegions(loc)
		if !set.TestState(nil, worldguard.FlagMobSpawning) {
			e.SetCancelled(true)
			return
		}
		if denied, _ := set.QueryValue(nil, worldguard.FlagDenySpawn).([]any); slices.Contains(denied, any(t)) {
			e.SetCancelled(true)
			return
		}
	}
	if cfg.BlockGroundSlimes && t == registry.EntityTypeSlime && loc.Pos.Y >= 60 && e.Reason == plugin.SpawnNatural {
		e.SetCancelled(true)
	}
}

// rulePortalCreate stops a nether portal's exit being built in regions the
// traveller may not build in, and the dragon's exit portal when set.
// WorldGuard: onCreatePortal
func (p *protection) rulePortalCreate(e *plugin.PortalCreateEvent) {
	cfg := p.cfg(e.World)
	if cfg.UseRegions && cfg.RegionNetherPortalProtection && e.Reason == plugin.PortalNetherPair && len(e.Blocks) > 0 {
		cause := p.cause(e.Entity)
		var lp worldguard.LocalPlayer
		if pl, ok := cause.Root().(plugin.Player); ok {
			if p.wg.HasBypass(pl, e.World) {
				return
			}
			lp = p.wg.WrapPlayer(pl)
		}
		regions := worldguard.ApplicableRegionSet(worldguard.FailedLoadRegionSet)
		if m, ok := p.wg.Regions(e.World); ok {
			lo, hi := e.Blocks[0].Pos, e.Blocks[0].Pos
			for _, b := range e.Blocks {
				lo = plugin.BlockPos{X: min(lo.X, b.Pos.X), Y: min(lo.Y, b.Pos.Y), Z: min(lo.Z, b.Pos.Z)}
				hi = plugin.BlockPos{X: max(hi.X, b.Pos.X), Y: max(hi.Y, b.Pos.Y), Z: max(hi.Z, b.Pos.Z)}
			}
			if shape, err := worldguard.NewCuboidRegion("__portal_check", lo, hi); err == nil {
				if check, err := worldguard.NewTransientRegion(shape, "__portal_check"); err == nil {
					regions = m.ApplicableRegionsFor(check, worldguard.ComputeParents)
				}
			}
		}
		who := p.associable(cause)
		build := worldguard.DenyToNone(regions.QueryState(who, worldguard.FlagBuild))
		if !worldguard.TestStates(build, regions.QueryState(who, worldguard.FlagBlockBreak)) ||
			!worldguard.TestStates(build, regions.QueryState(who, worldguard.FlagBlockPlace)) {
			if lp != nil && !cause.Indirect() {
				msg, _ := regions.QueryValue(lp, worldguard.FlagDenyMessage).(string)
				p.sendDeny(cause.Root().(plugin.Player), "create portals", msg)
			}
			e.SetCancelled(true)
		}
	}
	if e.Entity != nil && e.Entity.Type() == registry.EntityTypeEnderDragon && cfg.BlockEnderDragonPortalCreate {
		e.SetCancelled(true)
	}
}

func (p *protection) ruleRegain(e *plugin.EntityRegainHealthEvent) {
	if e.Reason != plugin.RegainNatural && e.Reason != plugin.RegainSatiated {
		return
	}
	cfg := p.cfg(e.Entity.World())
	if cfg.DisableHealthRegain {
		e.SetCancelled(true)
		return
	}
	if pl, ok := e.Entity.(plugin.Player); ok && cfg.UseRegions &&
		!p.query().TestState(pl.Location(), p.wg.WrapPlayer(pl), worldguard.FlagNaturalHealthRegen) {
		e.SetCancelled(true)
	}
}

func (p *protection) ruleFood(e *plugin.FoodLevelChangeEvent) {
	if !e.Item.Empty() || e.Level > e.Player.Food() {
		return
	}
	if p.cfg(e.Player.World()).UseRegions &&
		!p.query().TestState(e.Player.Location(), p.wg.WrapPlayer(e.Player), worldguard.FlagNaturalHungerDrain) {
		e.SetCancelled(true)
	}
}

func (p *protection) ruleChangeBlock(e *plugin.EntityChangeBlockEvent) {
	cfg := p.cfg(e.Block.World)
	switch e.Entity.Type() {
	case registry.EntityTypeFallingBlock:
		id := e.Block.State().Block()
		if id == registry.BlockGravel && cfg.NoPhysicsGravel || (id == registry.BlockSand || id == registry.BlockRedSand) && cfg.NoPhysicsSand {
			e.SetCancelled(true)
		}
	case registry.EntityTypeEnderman:
		if cfg.DisableEndermanGriefing {
			e.SetCancelled(true)
		}
	case registry.EntityTypeWither:
		if cfg.BlockWitherBlockDamage || cfg.BlockWitherExplosions || p.stateDenied(e.Block, worldguard.FlagWitherDamage) {
			e.SetCancelled(true)
		}
	}
}

// ruleHangingBreak protects paintings and item frames from mobs and
// blasts. WorldGuard: WorldGuardHangingListener
func (p *protection) ruleHangingBreak(e *plugin.HangingBreakEvent) {
	h := e.Entity
	cfg := p.cfg(h.World())
	painting := h.Type() == registry.EntityTypePainting
	frame := h.Type() == registry.EntityTypeItemFrame || h.Type() == registry.EntityTypeGlowItemFrame
	denied := func(f *worldguard.StateFlag) bool {
		return cfg.UseRegions && !worldguard.TestStates(p.query().QueryState(h.Location(), nil, f))
	}
	if e.Remover == nil {
		if e.Cause == plugin.HangingBreakExplosion && (painting && cfg.BlockEntityPaintingDestroy || frame && cfg.BlockEntityItemFrameDestroy) {
			e.SetCancelled(true)
		}
		return
	}
	remover := shooterOf(e.Remover)
	if _, ok := remover.(plugin.LivingEntity); !ok && remover != e.Remover {
		remover = nil
	}
	if _, ok := remover.(plugin.Player); ok {
		return
	}
	if remover != nil && remover.Type() == registry.EntityTypeCreeper {
		if cfg.BlockCreeperBlockDamage || cfg.BlockCreeperExplosions || denied(worldguard.FlagCreeperExplosion) {
			e.SetCancelled(true)
			return
		}
	}
	if painting && (cfg.BlockEntityPaintingDestroy || denied(worldguard.FlagEntityPaintingDestroy)) ||
		frame && (cfg.BlockEntityItemFrameDestroy || denied(worldguard.FlagEntityItemFrameDestroy)) {
		e.SetCancelled(true)
	}
}

// ruleKnockback keeps a wind charge from pushing what its burst may not
// touch, and a blow from pushing what it may not damage.
func (p *protection) ruleKnockback(e *plugin.EntityKnockbackEvent) {
	if e.Pusher == nil {
		return
	}
	t := e.Pusher.Type()
	if t != registry.EntityTypeWindCharge && t != registry.EntityTypeBreezeWindCharge {
		p.fireToCancel(e, &event.DamageEntityEvent{Entity: event.NewEntity(e, p.cause(e.Pusher), e.Entity)})
		return
	}
	cfg := p.cfg(e.Entity.World())
	if cfg.BlockWindChargeExplosions {
		e.SetCancelled(true)
		return
	}
	if !cfg.UseRegions {
		return
	}
	var who worldguard.RegionAssociable
	if pl, ok := e.Entity.(plugin.Player); ok {
		who = p.wg.WrapPlayer(pl)
	}
	if cfg.ExplosionFlagCancellation && !p.query().TestState(e.Entity.Location(), who, explosionFlag(t)) {
		e.SetCancelled(true)
	}
}
