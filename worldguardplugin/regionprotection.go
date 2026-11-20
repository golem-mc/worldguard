package worldguardplugin

import (
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/event"
)

// registerRegionProtection judges WorldGuard's events against the
// regions. WorldGuard: RegionProtectionListener
func (p *protection) registerRegionProtection() {
	s := p.wg.Server
	skip := plugin.ListenOptions{Priority: plugin.Normal, IgnoreCancelled: true}
	plugin.Listen(s, p.onPlaceBlock, skip)
	plugin.Listen(s, p.onBreakBlock, skip)
	plugin.Listen(s, p.onUseBlock, skip)
	plugin.Listen(s, p.onSpawnEntity, skip)
	plugin.Listen(s, p.onDestroyEntity, skip)
	plugin.Listen(s, p.onUseEntity, skip)
	plugin.Listen(s, p.onDamageEntity, skip)
	plugin.Listen(s, p.onEntityMount, plugin.ListenOptions{Priority: plugin.Normal})
	plugin.Listen(s, p.onDismount, skip)
}

// skipCheck reports whether the protection check does not apply: the
// event is already allowed, regions are off, or the cause bypasses them.
func (p *protection) skipCheck(d *event.Delegate, w plugin.World, pvp bool) bool {
	return d.Result == event.Allow || !p.regionsOn(w) || p.whitelisted(d.Cause, w, pvp)
}

func (p *protection) onPlaceBlock(e *event.PlaceBlockEvent) {
	if p.skipCheck(&e.Delegate, e.World, false) {
		return
	}
	q, who := p.query(), p.associable(e.Cause)
	t := e.Type
	if _, ok := e.Cause.Root().(plugin.Block); ok && isLiquid(t) && !p.cfg(e.World).CheckLiquidFlow {
		return
	}
	e.Filter(func(loc plugin.Location) bool {
		var ok bool
		var what string
		switch {
		case isFire(t):
			flags := []*worldguard.StateFlag{worldguard.FlagBlockPlace, worldguard.FlagLighter}
			if b, found := e.Cause.FirstBlock(); found {
				switch src := b.State().Block(); {
				case isFire(src):
					flags = append(flags, worldguard.FlagFireSpread)
				case isLava(src):
					flags = append(flags, worldguard.FlagLavaFire)
				}
			}
			ok, what = q.TestBuild(loc, who, combine(&e.Delegate, flags...)...), "place fire"
		case t == registry.BlockFrostedIce:
			e.Silent = true
			ok, what = q.TestBuild(loc, who, combine(&e.Delegate, worldguard.FlagBlockPlace, worldguard.FlagFrostedIceForm)...), "use frostwalker"
		default:
			ok, what = q.TestBuild(loc, who, combine(&e.Delegate, worldguard.FlagBlockPlace)...), "place that block"
		}
		if !ok {
			p.tellDenied(&e.Delegate, loc, what)
		}
		return ok
	}, false)
}

func (p *protection) onBreakBlock(e *event.BreakBlockEvent) {
	if p.skipCheck(&e.Delegate, e.World, false) {
		return
	}
	q, who := p.query(), p.associable(e.Cause)
	_, dynamite := e.Cause.Find(registry.EntityTypeTnt, registry.EntityTypeTntMinecart)
	e.Filter(func(loc plugin.Location) bool {
		var ok bool
		var what string
		if dynamite {
			ok, what = q.TestBuild(loc, who, combine(&e.Delegate, worldguard.FlagBlockBreak, worldguard.FlagTNT)...), "use dynamite"
		} else {
			ok, what = q.TestBuild(loc, who, combine(&e.Delegate, worldguard.FlagBlockBreak)...), "break that block"
		}
		if !ok {
			p.tellDenied(&e.Delegate, loc, what)
		}
		return ok
	}, false)
}

func (p *protection) onUseBlock(e *event.UseBlockEvent) {
	if p.skipCheck(&e.Delegate, e.World, false) {
		return
	}
	q, who := p.query(), p.associable(e.Cause)
	e.Filter(func(loc plugin.Location) bool {
		t := loc.World.BlockState(loc.Pos.Block()).Block()
		var flags []*worldguard.StateFlag
		what := "use that"
		switch {
		case consideredBuildingIfUsed(t):
		case inventoryBlock(t):
			flags, what = []*worldguard.StateFlag{worldguard.FlagChestAccess}, "open that"
		case isAnvil(t):
			flags = []*worldguard.StateFlag{worldguard.FlagUseAnvil}
		case isBed(t):
			flags, what = []*worldguard.StateFlag{worldguard.FlagInteract, worldguard.FlagSleep}, "sleep"
		case t == registry.BlockRespawnAnchor:
			flags, what = []*worldguard.StateFlag{worldguard.FlagInteract, worldguard.FlagRespawnAnchors}, "use anchors"
		case t == registry.BlockTnt:
			flags, what = []*worldguard.StateFlag{worldguard.FlagInteract, worldguard.FlagTNT}, "use explosives"
		case useFlagApplicable(t):
			flags = []*worldguard.StateFlag{worldguard.FlagInteract, worldguard.FlagUse}
		default:
			flags = []*worldguard.StateFlag{worldguard.FlagInteract}
		}
		ok := q.TestBuild(loc, who, combine(&e.Delegate, flags...)...)
		if !ok {
			p.tellDenied(&e.Delegate, loc, what)
		}
		return ok
	}, false)
}

func (p *protection) onSpawnEntity(e *event.SpawnEntityEvent) {
	if p.skipCheck(&e.Delegate, e.World, false) {
		return
	}
	q, who := p.query(), p.associable(e.Cause)
	var flag *worldguard.StateFlag
	what := "place things"
	switch {
	case isVehicleType(e.Type):
		flag, what = worldguard.FlagVehiclePlace, "place vehicles"
	case e.Type == registry.EntityTypeItem:
		flag, what = worldguard.FlagItemDrop, "drop items"
	case e.Type == registry.EntityTypeExperienceOrb:
		flag, what = worldguard.FlagExpDrops, "drop XP"
	case e.Type == registry.EntityTypeAreaEffectCloud:
		flag, what = worldguard.FlagPotionSplash, "use lingering potions"
	}
	var flags []*worldguard.StateFlag
	if flag != nil {
		flags = append(flags, flag)
	}
	if !q.TestBuild(e.Target, who, combine(&e.Delegate, flags...)...) {
		p.tellDenied(&e.Delegate, e.Target, what)
		e.SetCancelled(true)
	}
}

func (p *protection) onDestroyEntity(e *event.DestroyEntityEvent) {
	if p.skipCheck(&e.Delegate, e.World, false) {
		return
	}
	q, who := p.query(), p.associable(e.Cause)
	t := e.Entity.Entity.Type()
	var flags []*worldguard.StateFlag
	what := "break things"
	switch {
	case isVehicleType(t):
		flags, what = []*worldguard.StateFlag{worldguard.FlagVehicleDestroy}, "break vehicles"
	case t == registry.EntityTypeItem || t == registry.EntityTypeExperienceOrb:
		flags, what = []*worldguard.StateFlag{worldguard.FlagItemPickup}, "pick up items"
	}
	if !q.TestBuild(e.Target, who, combine(&e.Delegate, flags...)...) {
		p.tellDenied(&e.Delegate, e.Target, what)
		e.SetCancelled(true)
	}
}

func (p *protection) onUseEntity(e *event.UseEntityEvent) {
	if p.skipCheck(&e.Delegate, e.World, false) {
		return
	}
	q, who := p.query(), p.associable(e.Cause)
	ent := e.Entity.Entity
	_, isPlayer := ent.(plugin.Player)
	_, inventory := e.Original.(*plugin.InventoryOpenEvent)
	_, moveItem := e.Original.(*plugin.InventoryMoveItemEvent)
	var ok bool
	what := "use that"
	switch {
	case isHostile(ent) || isAmbient(ent) || isPlayer:
		ok = len(e.Flags) == 0 || q.QueryState(e.Target, who, combine(&e.Delegate)...) != worldguard.StateDeny
	case buildingIfUsed(ent) || inventory:
		frame, isFrame := ent.(plugin.ItemFrame)
		switch {
		case isFrame && e.Cause.FirstPlayer() != nil && !frame.Item().Empty():
			ok, what = q.TestBuild(e.Target, who, combine(&e.Delegate, worldguard.FlagItemFrameRotation)...), "change that"
		case inventory || moveItem:
			ok, what = q.TestBuild(e.Target, who, combine(&e.Delegate, worldguard.FlagChestAccess)...), "open that"
		default:
			ok, what = q.TestBuild(e.Target, who, combine(&e.Delegate)...), "change that"
		}
	case riddenOnUse(ent):
		ok, what = true, "ride that"
	default:
		ok = q.TestBuild(e.Target, who, combine(&e.Delegate, worldguard.FlagInteract)...)
	}
	if !ok {
		p.tellDenied(&e.Delegate, e.Target, what)
		e.SetCancelled(true)
	}
}

func (p *protection) onDamageEntity(e *event.DamageEntityEvent) {
	if e.Result == event.Allow || !p.regionsOn(e.World) {
		return
	}
	q, who := p.query(), p.associable(e.Cause)
	ent := e.Entity.Entity
	attacker := e.Cause.FirstPlayer()
	defender, isPlayer := ent.(plugin.Player)
	pvp := isPlayer && attacker != nil && attacker.UUID() != defender.UUID()
	if p.whitelisted(e.Cause, e.World, pvp) {
		return
	}
	var ok bool
	what := "hit that"
	switch {
	case isHostile(ent) || isAmbient(ent):
		ok = len(e.Flags) == 0 || q.QueryState(e.Target, who, combine(&e.Delegate)...) != worldguard.StateDeny
	case isVehicleType(ent.Type()):
		ok, what = q.TestBuild(e.Target, who, combine(&e.Delegate, worldguard.FlagVehicleDestroy)...), "change that"
	case buildingIfUsed(ent):
		ok, what = q.TestBuild(e.Target, who, combine(&e.Delegate)...), "change that"
	case pvp:
		la := p.wg.WrapPlayer(attacker)
		flags := combine(&e.Delegate, worldguard.FlagPVP)
		ok = q.TestBuild(e.Target, who, flags...) &&
			q.QueryState(attacker.Location(), la, flags...) != worldguard.StateDeny &&
			q.QueryState(e.Target, la, flags...) != worldguard.StateDeny
		if !ok && p.disallowedPVP(attacker, defender, e.Original) {
			ok = true
		}
		what = "PvP"
	case isPlayer:
		ok, what = len(e.Flags) == 0 || q.QueryState(e.Target, who, combine(&e.Delegate)...) != worldguard.StateDeny, "damage that"
	case isNonHostile(ent):
		ok, what = q.TestBuild(e.Target, who, combine(&e.Delegate, worldguard.FlagDamageAnimals)...), "harm that"
	default:
		ok = q.TestBuild(e.Target, who, combine(&e.Delegate, worldguard.FlagInteract)...)
	}
	if !ok {
		p.tellDenied(&e.Delegate, e.Target, what)
		e.SetCancelled(true)
	}
}

// disallowedPVP fires DisallowedPVPEvent; true when a listener lets the
// fight happen after all. WorldGuard: DisallowedPVPEvent
func (p *protection) disallowedPVP(attacker, defender plugin.Player, original plugin.Event) bool {
	ev := &event.DisallowedPVPEvent{Attacker: attacker, Defender: defender, Original: original}
	p.fire(ev)
	return ev.Cancelled()
}

// onEntityMount stops a player getting on a vehicle or mount where they may
// not ride. WorldGuard: RegionProtectionListener.onEntityMount
func (p *protection) onEntityMount(e *plugin.EntityMountEvent) {
	pl, ok := e.Entity.(plugin.Player)
	if !ok || !p.regionsOn(e.Vehicle.World()) {
		return
	}
	cause := p.cause(pl)
	if p.whitelisted(cause, e.Vehicle.World(), false) {
		return
	}
	loc := e.Vehicle.Location()
	if !p.query().TestBuild(loc, p.wg.WrapPlayer(pl), worldguard.FlagRide, worldguard.FlagInteract) {
		e.SetCancelled(true)
		p.tellDenied(&event.Delegate{Original: e, Cause: cause}, loc, "ride that")
	}
}

// onDismount keeps a player on a tamed mount they could not get back on.
// WorldGuard: RegionProtectionListener.onVehicleExit
func (p *protection) onDismount(e *plugin.EntityDismountEvent) {
	pl, ok := e.Entity.(plugin.Player)
	if !ok || !isTamed(e.Vehicle) || !p.regionsOn(e.Vehicle.World()) {
		return
	}
	if p.whitelisted(p.cause(pl), e.Vehicle.World(), false) {
		return
	}
	if p.query().TestBuild(e.Vehicle.Location(), p.wg.WrapPlayer(pl), worldguard.FlagRide, worldguard.FlagInteract) {
		return
	}
	now := time.Now()
	p.mu.Lock()
	last, seen := p.lastDisembk[pl.UUID()]
	if !seen || now.Sub(last) >= 500*time.Millisecond {
		p.lastDisembk[pl.UUID()] = now
		p.mu.Unlock()
		tell(pl, "§6Don't disembark here!§7 You can't get back on.")
	} else {
		p.mu.Unlock()
	}
	e.SetCancelled(true)
}
