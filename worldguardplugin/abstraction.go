package worldguardplugin

import (
	"slices"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/event"
)

// registerAbstraction turns the server's events into WorldGuard's.
// WorldGuard: EventAbstractionListener
func (p *protection) registerAbstraction() {
	s := p.wg.Server
	skip := plugin.ListenOptions{Priority: plugin.Normal, IgnoreCancelled: true}
	plugin.Listen(s, p.onBlockBreak, skip)
	plugin.Listen(s, p.onBlockPlace, skip)
	plugin.Listen(s, p.onBlockBurn, skip)
	plugin.Listen(s, p.onStructureGrow, skip)
	plugin.Listen(s, p.onEntityChangeBlock, skip)
	plugin.Listen(s, p.onEntityExplode, skip)
	plugin.Listen(s, p.onBlockExplode, skip)
	plugin.Listen(s, p.onPiston, skip)
	plugin.Listen(s, p.onBlockDamage, skip)
	plugin.Listen(s, p.onPlayerInteract, plugin.ListenOptions{Priority: plugin.Normal})
	plugin.Listen(s, p.onEntityBlockForm, skip)
	plugin.Listen(s, p.onEntityInteract, skip)
	plugin.Listen(s, p.onBlockFertilize, skip)
	plugin.Listen(s, p.onBlockIgnite, skip)
	plugin.Listen(s, p.onSignChange, skip)
	plugin.Listen(s, p.onBedEnter, skip)
	plugin.Listen(s, p.onBucket, skip)
	plugin.Listen(s, p.onBlockFromTo, skip)
	plugin.Listen(s, p.onCreatureSpawn, skip)
	plugin.Listen(s, p.onHangingPlace, skip)
	plugin.Listen(s, p.onHangingBreak, skip)
	plugin.Listen(s, p.onVehicle, skip)
	plugin.Listen(s, p.onFish, skip)
	plugin.Listen(s, p.onProjectileHit, skip)
	plugin.Listen(s, p.onExpBottle, skip)
	plugin.Listen(s, func(e *plugin.EntityDeathEvent) { p.onDeathExperience(e, e.Entity, &e.Experience) }, skip)
	plugin.Listen(s, func(e *plugin.PlayerDeathEvent) { p.onDeathExperience(e, e.Entity, &e.Experience) }, skip)
	plugin.Listen(s, p.onInteractEntity, skip)
	plugin.Listen(s, p.onEntityDamage, skip)
	plugin.Listen(s, p.onEntityTame, skip)
	plugin.Listen(s, p.onShearEntity, skip)
	plugin.Listen(s, p.onPickupItem, skip)
	plugin.Listen(s, p.onDropItem, skip)
	plugin.Listen(s, p.onItemConsume, skip)
	plugin.Listen(s, p.onInventoryOpen, skip)
	plugin.Listen(s, p.onInventoryMove, skip)
	plugin.Listen(s, p.onDispense, skip)
	plugin.Listen(s, p.onCauldronChange, skip)
	plugin.Listen(s, p.onEntityTransform, skip)
}

func (p *protection) onBlockBreak(e *plugin.BlockBreakEvent) {
	ev := &event.BreakBlockEvent{Blocks: event.NewBlocks(e, p.cause(e.Player), e.Block.World, e.State.Block(), e.Block.Pos)}
	if p.fireToCancel(e, ev) {
		p.denyEffect(e.Player, e.Block.World, e.Block.Pos.Center().Add(plugin.Vec3{Y: .5}))
		return
	}
	// The experience the block drops. WorldGuard: onBlockExp
	if e.Experience > 0 {
		xp := &event.SpawnEntityEvent{Original: e, Cause: p.cause(e.Block),
			World: e.Block.World, Target: blockLoc(e.Block), Type: registry.EntityTypeExperienceOrb}
		if p.fireAndTest(xp) {
			e.Experience = 0
		}
	}
}

func (p *protection) onBlockPlace(e *plugin.BlockPlaceEvent) {
	placed := e.Placed.Block()
	cause := p.cause(e.Player)
	if len(e.Blocks) > 1 {
		if placed == registry.BlockBedrock || placed == registry.BlockEndPortalFrame {
			return
		}
		ev := &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, cause, e.Block.World, placed, slices.Clone(e.Blocks)...)}
		p.fireToCancel(e, ev)
		if len(ev.Positions) != len(e.Blocks) {
			e.SetCancelled(true)
		}
		return
	}
	if old := e.Replaced.Block(); !isAir(old) && old != placed {
		p.fireToCancel(e, &event.BreakBlockEvent{Blocks: event.NewBlocks(e, cause, e.Block.World, old, e.Block.Pos)})
	}
	if !e.Cancelled() && !e.Item.Empty() {
		p.fireToCancel(e, &event.UseItemEvent{Original: e, Cause: cause, World: e.Block.World, Item: e.Item})
	}
	if !e.Cancelled() {
		p.fireToCancel(e, &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, cause, e.Block.World, placed, e.Block.Pos)})
	}
	if e.Cancelled() {
		p.denyEffect(e.Player, e.Block.World, e.Block.Pos.Center())
	}
}

var sixFaces = []plugin.BlockFace{plugin.North, plugin.South, plugin.West, plugin.East, plugin.Up, plugin.Down}

// onBlockBurn asks, for every fire next to the burning block, whether that
// fire may break it; a fire that may not is put out. WorldGuard: onBlockBurn
func (p *protection) onBlockBurn(e *plugin.BlockBurnEvent) {
	found, allowed := 0, false
	for _, f := range sixFaces {
		src := e.Block.Relative(f, 1)
		if !isFire(src.State().Block()) {
			continue
		}
		found++
		ev := &event.BreakBlockEvent{Blocks: event.NewBlocks(e, p.cause(src), e.Block.World, e.Block.State().Block(), e.Block.Pos)}
		if p.fireAndTest(ev) {
			src.Set(plugin.DefaultState(registry.BlockAir))
		} else {
			allowed = true
		}
	}
	if found > 0 && !allowed {
		e.SetCancelled(true)
	}
}

// keepPositions trims changes to the positions left in kept.
func keepPositions(changes []plugin.BlockChange, kept []plugin.BlockPos) []plugin.BlockChange {
	return slices.DeleteFunc(changes, func(c plugin.BlockChange) bool { return !slices.Contains(kept, c.Pos) })
}

func changePositions(changes []plugin.BlockChange) []plugin.BlockPos {
	out := make([]plugin.BlockPos, len(changes))
	for i, c := range changes {
		out[i] = c.Pos
	}
	return out
}

func (p *protection) onStructureGrow(e *plugin.StructureGrowEvent) {
	if len(e.Blocks) == 0 {
		return
	}
	src := plugin.Block{World: e.World, Pos: e.Pos}
	cause := p.cause(src)
	if e.Player != nil {
		cause = p.cause(e.Player)
	}
	n := len(e.Blocks)
	ev := &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, cause, e.World, e.Blocks[0].State.Block(), changePositions(e.Blocks)...)}
	if p.fireBulk(e, ev) {
		return
	}
	e.Blocks = keepPositions(e.Blocks, ev.Positions)
	if len(e.Blocks) != n {
		src.Set(plugin.DefaultState(registry.BlockAir))
	}
}

// changeOptions marks the quiet changes: trampled farmland, redstone ore
// lit by a step, a tilting dripleaf. WorldGuard: setDelegateEventMaterialOptions
func changeOptions(d *event.Delegate, from, to registry.Block) {
	switch {
	case from == registry.BlockFarmland && to == registry.BlockDirt:
		d.Silent = true
		d.AddFlags(worldguard.FlagBlockTrampling)
	case from == registry.BlockRedstoneOre || from == registry.BlockDeepslateRedstoneOre:
		d.Silent = true
	case from == registry.BlockBigDripleaf && to == registry.BlockBigDripleaf:
		d.Silent = true
		d.AddFlags(worldguard.FlagUseDripleaf)
	}
}

func (p *protection) onEntityChangeBlock(e *plugin.EntityChangeBlockEvent) {
	from, to := e.Block.State().Block(), e.NewState.Block()
	cause := p.cause(e.Entity)
	pl, _ := e.Entity.(plugin.Player)
	switch {
	case !isAir(to) && !isAir(from):
		br := &event.BreakBlockEvent{Blocks: event.NewBlocks(e, cause, e.Block.World, from, e.Block.Pos)}
		changeOptions(&br.Delegate, from, to)
		denied := p.fireToCancel(e, br)
		if !denied {
			pl := &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, cause, e.Block.World, to, e.Block.Pos)}
			changeOptions(&pl.Delegate, from, to)
			denied = p.fireToCancel(e, pl)
		}
		if denied && pl != nil {
			p.denyEffect(pl, e.Block.World, e.Block.Pos.Center())
		}
		p.fallingBlockLanded(e, denied)
	case isAir(to):
		if _, ok := e.Entity.(plugin.FallingBlock); ok {
			event.TrackParent(e.Entity, e.Block)
			p.fireToCancel(e, &event.SpawnEntityEvent{Entity: event.NewEntity(e, p.cause(e.Block), e.Entity), Type: e.Entity.Type()})
			return
		}
		key := [2]any{e.Block, e.Entity.UUID()}
		p.entityBreak.fire(p, key, e, &event.BreakBlockEvent{Blocks: event.NewBlocks(e, cause, e.Block.World, from, e.Block.Pos)})
	default:
		denied := p.fireToCancel(e, &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, cause, e.Block.World, to, e.Block.Pos)})
		p.fallingBlockLanded(e, denied)
	}
}

// fallingBlockLanded drops a falling block as an item where it may not
// land, and forgets where it fell from. WorldGuard: handleFallingBlock
func (p *protection) fallingBlockLanded(e *plugin.EntityChangeBlockEvent, denied bool) {
	fb, ok := e.Entity.(plugin.FallingBlock)
	if !ok {
		return
	}
	defer event.UntrackParent(e.Entity)
	if !denied || !fb.DropItem() {
		return
	}
	item, ok := fb.State().Block().Item()
	if !ok || item == registry.ItemAir {
		return
	}
	drop := e.Block.World.DropItem(fb.Pos(), plugin.NewStack(item, 1), false)
	if drop == nil {
		return
	}
	drop.SetVelocity(plugin.Vec3{})
	if p.fireAndTest(&event.SpawnEntityEvent{Entity: event.NewEntity(e, p.cause(e.Block, e.Entity), drop), Type: drop.Type()}) {
		drop.Remove()
	}
}

// blocksOf answers the positions of blocks.
func blocksOf(blocks []plugin.Block) []plugin.BlockPos {
	out := make([]plugin.BlockPos, len(blocks))
	for i, b := range blocks {
		out[i] = b.Pos
	}
	return out
}

// keepBlocks trims blocks to the positions left in kept.
func keepBlocks(blocks []plugin.Block, kept []plugin.BlockPos) []plugin.Block {
	return slices.DeleteFunc(blocks, func(b plugin.Block) bool { return !slices.Contains(kept, b.Pos) })
}

func (p *protection) onEntityExplode(e *plugin.EntityExplodeEvent) {
	w := e.Location.World
	if w == nil || len(e.Blocks) == 0 {
		if e.Type == registry.EntityTypeCreeper {
			event.UntrackParent(e.Entity)
		}
		return
	}
	cause := p.cause(e.Entity)
	positions := blocksOf(e.Blocks)
	switch e.Type {
	case registry.EntityTypeWindCharge, registry.EntityTypeBreezeWindCharge:
		// A wind charge only triggers blocks (doors, buttons).
		ev := &event.UseBlockEvent{Blocks: event.NewBlocks(e, cause, w, registry.BlockAir, positions...)}
		ev.AddFlags(explosionFlag(e.Type))
		ev.Silent = true
		if !p.fireBulk(e, ev) {
			e.Blocks = keepBlocks(e.Blocks, ev.Positions)
		}
	default:
		ev := &event.BreakBlockEvent{Blocks: event.NewBlocks(e, cause, w, registry.BlockAir, positions...)}
		if !p.fireBulk(e, ev) {
			e.Blocks = keepBlocks(e.Blocks, ev.Positions)
		}
	}
	if e.Type == registry.EntityTypeCreeper {
		event.UntrackParent(e.Entity)
	}
}

func (p *protection) onBlockExplode(e *plugin.BlockExplodeEvent) {
	if len(e.Blocks) == 0 {
		return
	}
	ev := &event.BreakBlockEvent{Blocks: event.NewBlocks(e, p.cause(e.Block), e.Block.World, registry.BlockAir, blocksOf(e.Blocks)...)}
	ev.AddFlags(worldguard.FlagOtherExplosion)
	if !p.fireBulk(e, ev) {
		e.Blocks = keepBlocks(e.Blocks, ev.Positions)
	}
}

// onPiston asks whether the piston may break the blocks it moves and place
// them where they go. WorldGuard: onBlockPistonExtend, onBlockPistonRetract
func (p *protection) onPiston(e *plugin.BlockPistonEvent) {
	if len(e.Blocks) == 0 {
		return
	}
	w := e.Piston.World
	cause := p.cause(e.Piston)
	n := len(e.Blocks)
	br := &event.BreakBlockEvent{Blocks: event.NewBlocks(e, cause, w, registry.BlockAir, blocksOf(e.Blocks)...)}
	if p.fireBulk(e, br) {
		return
	}
	if len(br.Positions) != n {
		e.SetCancelled(true)
		return
	}
	dir := e.Direction
	if !e.Extending {
		dir = dir.Opposite()
	}
	dest := make([]plugin.BlockPos, n)
	for i, b := range e.Blocks {
		dest[i] = b.Pos.Offset(dir, 1)
	}
	pl := &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, cause, w, registry.BlockStone, dest...)}
	p.fireBulk(e, pl)
	if len(pl.Positions) != n {
		e.SetCancelled(true)
	}
	if e.Cancelled() {
		p.denyEffect(nil, w, e.Piston.Pos.Center().Add(plugin.Vec3{Y: .5}))
	}
}

func (p *protection) onBlockDamage(e *plugin.BlockDamageEvent) {
	if e.Block.State().Block() == registry.BlockCake {
		p.fireToCancel(e, &event.UseBlockEvent{Blocks: event.NewBlocks(e, p.cause(e.Player), e.Block.World, registry.BlockCake, e.Block.Pos)})
	}
}

// connected answers the other half of a bed or a double chest.
// WorldGuard: Blocks.getConnected
func connected(b plugin.Block) []plugin.Block {
	s := b.State()
	facing, ok := s.Prop(registry.PropFacing)
	if !ok {
		return nil
	}
	f, ok := faceNamed(facing)
	if !ok {
		return nil
	}
	if part, ok := s.Prop(registry.PropPart); ok && isBed(s.Block()) {
		if part == "foot" {
			return []plugin.Block{b.Relative(f, 1)}
		}
		return []plugin.Block{b.Relative(f.Opposite(), 1)}
	}
	if typ, ok := s.Prop(registry.PropType); ok && (s.Block() == registry.BlockChest || s.Block() == registry.BlockTrappedChest) {
		switch typ {
		case "left":
			return []plugin.Block{b.Relative(clockwise(f), 1)}
		case "right":
			return []plugin.Block{b.Relative(clockwise(f).Opposite(), 1)}
		}
	}
	return nil
}

func faceNamed(s string) (plugin.BlockFace, bool) {
	switch s {
	case "down":
		return plugin.Down, true
	case "up":
		return plugin.Up, true
	case "north":
		return plugin.North, true
	case "south":
		return plugin.South, true
	case "west":
		return plugin.West, true
	case "east":
		return plugin.East, true
	}
	return 0, false
}

// clockwise turns a horizontal face a quarter clockwise seen from above.
func clockwise(f plugin.BlockFace) plugin.BlockFace {
	switch f {
	case plugin.North:
		return plugin.East
	case plugin.East:
		return plugin.South
	case plugin.South:
		return plugin.West
	case plugin.West:
		return plugin.North
	}
	return f
}

// onPlayerInteract judges a click: stepping on a block, using one, an
// item on one, an item in the air. WorldGuard: onPlayerInteract
func (p *protection) onPlayerInteract(e *plugin.PlayerInteractEvent) {
	pl := e.Player
	cause := p.cause(pl)
	item := e.Item
	switch e.Action {
	case plugin.Physical:
		if e.DenyBlock || e.Block == nil {
			return
		}
		p.physicalInteract(e, cause)
		return
	case plugin.RightClickBlock, plugin.LeftClickBlock:
		if e.Block == nil {
			return
		}
		clicked := *e.Block
		placed := clicked.Relative(e.Face, 1)
		right := e.Action == plugin.RightClickBlock
		if right && !e.DenyBlock {
			p.blockRightClick(e, cause, item, clicked, placed)
		}
		if !e.DenyBlock {
			if p.clickBlock(e, cause, item, clicked, placed, right) {
				return
			}
		}
	}
	if !e.DenyItem && !item.Empty() && !isBlockItem(item.Item) &&
		p.fireAndTest(&event.UseItemEvent{Original: e, Cause: cause, World: pl.World(), Item: item}) {
		e.DenyItem = true
	}
	if !item.Empty() && p.cfg(pl.World()).BlockUseAtFeet.HasItem(item.Item) {
		feet := plugin.Block{World: pl.World(), Pos: pl.Pos().Block()}
		if p.fireAndTest(&event.UseBlockEvent{Blocks: event.NewBlocks(e, cause, feet.World, feet.State().Block(), feet.Pos)}) {
			e.DenyBlock = true
		}
	}
}

// clickBlock judges using the clicked block; true when a broken fire
// ended the judging. WorldGuard: onPlayerInteract (LEFT_CLICK_BLOCK)
func (p *protection) clickBlock(e *plugin.PlayerInteractEvent, cause event.Cause, item plugin.ItemStack,
	clicked, placed plugin.Block, right bool) bool {
	t := clicked.State().Block()
	itemUse := right && !item.Empty() && p.itemApplied(item.Item, clicked)
	modifies := itemUse || blockModifiedOnClick(t, right) && !p.interactBypass(clicked)
	use := &event.UseBlockEvent{Blocks: event.NewBlocks(e, cause, clicked.World, t, clicked.Pos)}
	use.SetAllowed(!modifies)
	if p.fireAndTest(use) {
		e.DenyBlock = true
	}
	for _, c := range connected(clicked) {
		use := &event.UseBlockEvent{Blocks: event.NewBlocks(e, cause, c.World, c.State().Block(), c.Pos)}
		use.SetAllowed(!modifies)
		if p.fireAndTest(use) {
			e.DenyBlock = true
			break
		}
	}
	if itemUse && p.fireAndTest(&event.PlaceBlockEvent{Blocks: event.NewBlocks(e, cause, clicked.World, t, clicked.Pos)}) {
		e.DenyItem, e.DenyBlock = true, true
	}
	if !right && isFire(placed.State().Block()) &&
		p.fireAndTest(&event.BreakBlockEvent{Blocks: event.NewBlocks(e, cause, placed.World, placed.State().Block(), placed.Pos)}) {
		e.DenyBlock = true
		return true
	}
	if e.DenyBlock {
		p.denyEffect(e.Player, clicked.World, clicked.Pos.Center().Add(plugin.Vec3{Y: .5}))
	}
	return false
}

// itemApplied reports whether item changes clicked. WorldGuard:
// EventAbstractionListener.isItemAppliedToBlock
func (p *protection) itemApplied(item registry.Item, clicked plugin.Block) bool {
	return itemAppliedToBlock(item, clicked.State().Block()) && !p.interactBypass(clicked) &&
		!p.interactBypassItem(clicked.World, item)
}

// physicalInteract judges stepping on a block: trampling farmland or eggs
// breaks them, anything else uses it. WorldGuard: onPlayerInteract (PHYSICAL)
func (p *protection) physicalInteract(e *plugin.PlayerInteractEvent, cause event.Cause) {
	clicked := *e.Block
	t := clicked.State().Block()
	if t == registry.BlockFarmland || t == registry.BlockTurtleEgg || t == registry.BlockSnifferEgg {
		br := &event.BreakBlockEvent{Blocks: event.NewBlocks(e, cause, clicked.World, t, clicked.Pos)}
		br.Silent = true
		br.AddFlags(worldguard.FlagBlockTrampling)
		denied := p.fireToCancel(e, br)
		if !denied {
			to := t
			if t == registry.BlockFarmland {
				to = registry.BlockDirt
			}
			pl := &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, cause, clicked.World, to, clicked.Pos)}
			pl.Silent = true
			pl.AddFlags(worldguard.FlagBlockTrampling)
			denied = p.fireToCancel(e, pl)
		}
		if denied {
			p.denyEffect(e.Player, clicked.World, clicked.Pos.Center())
		}
		return
	}
	use := &event.UseBlockEvent{Blocks: event.NewBlocks(e, cause, clicked.World, t, clicked.Pos)}
	use.SetAllowed(p.interactBypass(clicked))
	if t == registry.BlockRedstoneOre || t == registry.BlockDeepslateRedstoneOre {
		use.Silent = true
	}
	if t == registry.BlockBigDripleaf {
		use.AddFlags(worldguard.FlagUseDripleaf)
		use.Silent = true
	}
	p.interact.fire(p, [2]any{clicked, e.Player.UUID()}, e, use)
	if e.Cancelled() && !use.Silent {
		p.denyEffect(e.Player, clicked.World, clicked.Pos.Center().Add(plugin.Vec3{Y: 1}))
	}
}

// blockRightClick judges what an item does when used on a block: placing
// TNT, a vehicle, an armour stand, a mob from an egg, a liquid.
// WorldGuard: handleBlockRightClick
func (p *protection) blockRightClick(original interface{ SetCancelled(bool) }, cause event.Cause, item plugin.ItemStack, clicked, placed plugin.Block) {
	if item.Empty() {
		return
	}
	ev, _ := original.(plugin.Event)
	spawn := func(at plugin.Block, t registry.EntityType) {
		loc := blockLoc(at)
		loc.Pos = loc.Pos.Add(plugin.Vec3{X: .5, Z: .5})
		p.fireToCancel(original, &event.SpawnEntityEvent{Original: ev, Cause: cause,
			World: at.World, Target: loc, Type: t})
	}
	switch i := item.Item; {
	case i == registry.ItemTnt:
		p.fireToCancel(original, &event.UseBlockEvent{Blocks: event.NewBlocks(ev, cause, clicked.World, registry.BlockTnt, clicked.Pos)})
		p.fireToCancel(original, &event.PlaceBlockEvent{Blocks: event.NewBlocks(ev, cause, placed.World, registry.BlockTnt, placed.Pos)})
	case isMinecartItem(i):
		t, ok := entityOfItem(i)
		if !ok {
			t = registry.EntityTypeMinecart
		}
		spawn(clicked, t)
	case isBoatItem(i):
		t, ok := entityOfItem(i)
		if !ok {
			t = registry.EntityTypeOakBoat
		}
		spawn(placed, t)
	case isCushionItem(i):
		if t, ok := registry.EntityTypeByName("minecraft:cushion"); ok {
			spawn(placed, t)
		}
	case i == registry.ItemArmorStand:
		spawn(placed, registry.EntityTypeArmorStand)
	case i == registry.ItemEndCrystal:
		spawn(placed, registry.EntityTypeEndCrystal)
	default:
		if t, ok := spawnEggEntity(i); ok {
			spawn(placed, t)
			return
		}
		if i == registry.ItemWaterBucket || i == registry.ItemLavaBucket {
			p.fireToCancel(original, &event.PlaceBlockEvent{Blocks: event.NewBlocks(ev, cause, placed.World, bucketBlock(i), placed.Pos)})
		}
	}
}

func (p *protection) onEntityBlockForm(e *plugin.BlockFormEvent) {
	if e.Entity == nil {
		return
	}
	p.entityBreak.fire(p, [2]any{e.Block, e.Entity.UUID()}, e,
		&event.PlaceBlockEvent{Blocks: event.NewBlocks(e, p.cause(e.Entity), e.Block.World, e.NewState.Block(), e.Block.Pos)})
}

func (p *protection) onEntityInteract(e *plugin.EntityInteractEvent) {
	use := &event.UseBlockEvent{Blocks: event.NewBlocks(e, p.cause(e.Entity), e.Block.World, e.Block.State().Block(), e.Block.Pos)}
	use.SetAllowed(p.interactBypass(e.Block))
	p.interact.fire(p, [2]any{e.Block, e.Entity.UUID()}, e, use)
}

func (p *protection) onBlockFertilize(e *plugin.BlockFertilizeEvent) {
	if len(e.Blocks) == 0 {
		return
	}
	var cause event.Cause
	if e.Player != nil {
		cause = p.cause(e.Player, e.Block)
	} else {
		cause = p.cause(e.Block)
	}
	p.fireToCancel(e, &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, cause, e.Block.World, e.Blocks[0].State.Block(), changePositions(e.Blocks)...)})
}

// onBlockIgnite asks whether fire may be placed. The server does not say
// which fire or lava spread it, so a fire or lava block nearby stands in.
// WorldGuard: onBlockIgnite
func (p *protection) onBlockIgnite(e *plugin.BlockIgniteEvent) {
	var cause event.Cause
	switch {
	case e.Player != nil:
		cause = p.cause(e.Player)
	case e.Entity != nil:
		cause = p.cause(e.Entity)
	case e.Cause == plugin.IgniteSpread:
		cause = p.causeNear(e.Block, isFire, 1, -1, 4)
	case e.Cause == plugin.IgniteLava:
		cause = p.causeNear(e.Block, isLava, 2, -1, 1)
	default:
		cause = event.Unknown()
	}
	p.fireToCancel(e, &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, cause, e.Block.World, registry.BlockFire, e.Block.Pos)})
}

// causeNear answers the cause of a block that matches near b, within r
// blocks sideways and from dy0 to dy1 vertically; unknown when none.
func (p *protection) causeNear(b plugin.Block, match func(registry.Block) bool, r, dy0, dy1 int) event.Cause {
	for dy := dy0; dy <= dy1; dy++ {
		for dx := -r; dx <= r; dx++ {
			for dz := -r; dz <= r; dz++ {
				n := plugin.Block{World: b.World, Pos: plugin.BlockPos{X: b.Pos.X + dx, Y: b.Pos.Y + dy, Z: b.Pos.Z + dz}}
				if (dx != 0 || dy != 0 || dz != 0) && match(n.State().Block()) {
					return p.cause(n)
				}
			}
		}
	}
	return event.Unknown()
}

func (p *protection) onSignChange(e *plugin.SignChangeEvent) {
	if p.fireToCancel(e, &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, p.cause(e.Player), e.Block.World, e.Block.State().Block(), e.Block.Pos)}) {
		p.denyEffect(e.Player, e.Block.World, e.Block.Pos.Center())
	}
}

func (p *protection) onBedEnter(e *plugin.PlayerBedEnterEvent) {
	p.fireToCancel(e, &event.UseBlockEvent{Blocks: event.NewBlocks(e, p.cause(e.Player), e.Bed.World, e.Bed.State().Block(), e.Bed.Pos)})
}

// onBucket judges filling or emptying a bucket at the block it changes.
// WorldGuard: onPlayerBucketEmpty, onPlayerBucketFill
func (p *protection) onBucket(e *plugin.PlayerBucketEvent) {
	cause := p.cause(e.Player)
	affected := e.Block
	bucket := e.Item.Item
	milk := bucket == registry.ItemMilkBucket || e.Result.Item == registry.ItemMilkBucket
	if e.Fill {
		br := &event.BreakBlockEvent{Blocks: event.NewBlocks(e, cause, affected.World, affected.State().Block(), affected.Pos)}
		br.SetAllowed(milk)
		p.fireToCancel(e, br)
	} else {
		pl := &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, cause, affected.World, bucketBlock(bucket), affected.Pos)}
		pl.SetAllowed(milk)
		p.fireToCancel(e, pl)
	}
	use := &event.UseItemEvent{Original: e, Cause: cause, World: e.Player.World(), Item: plugin.NewStack(bucket, 1)}
	use.SetAllowed(milk)
	p.fireToCancel(e, use)
	if e.Cancelled() {
		p.denyEffect(e.Player, affected.World, affected.Pos.Center())
	}
}

// onBlockFromTo asks whether a liquid may flow into a block, when regions
// check liquid flow. WorldGuard: onBlockFromTo
func (p *protection) onBlockFromTo(e *plugin.BlockFromToEvent) {
	cfg := p.cfg(e.From.World)
	if !cfg.UseRegions || !cfg.HighFreqFlags && !cfg.CheckLiquidFlow {
		return
	}
	from, to := e.From.State().Block(), e.To.State().Block()
	if isLiquid(from) && !isAir(to) && !to.NoCollision() {
		return
	}
	if isWater(from) && isWater(to) || isLava(from) && isLava(to) {
		return
	}
	p.fireToCancel(e, &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, p.cause(e.From), e.To.World, from, e.To.Pos)})
}

func (p *protection) onCreatureSpawn(e *plugin.EntitySpawnEvent) {
	switch e.Reason {
	case plugin.SpawnEgg, plugin.SpawnSpawnerEgg, plugin.SpawnDispenser:
		if _, mob := e.Entity.(plugin.Mob); !mob {
			return
		}
		if p.cfg(e.Entity.World()).StrictEntitySpawn {
			p.fireToCancel(e, &event.SpawnEntityEvent{Entity: event.NewEntity(e, event.Unknown(), e.Entity), Type: e.Entity.Type()})
		}
	}
}

func (p *protection) onHangingPlace(e *plugin.HangingPlaceEvent) {
	var cause event.Cause
	if e.Player != nil {
		cause = p.cause(e.Player)
	}
	if p.fireToCancel(e, &event.SpawnEntityEvent{Entity: event.NewEntity(e, cause, e.Entity), Type: e.Entity.Type()}) {
		p.denyEffect(e.Player, e.Block.World, e.Block.Relative(e.Face, 1).Pos.Center())
	}
}

func (p *protection) onHangingBreak(e *plugin.HangingBreakEvent) {
	if e.Remover != nil {
		if p.fireToCancel(e, &event.DestroyEntityEvent{Entity: event.NewEntity(e, p.cause(e.Remover), e.Entity)}) {
			if pl, ok := e.Remover.(plugin.Player); ok {
				p.denyEffect(pl, e.Entity.World(), e.Entity.Pos())
			}
		}
		return
	}
	if e.Cause != plugin.HangingBreakExplosion {
		return
	}
	ev := &event.DestroyEntityEvent{Entity: event.NewEntity(e, event.Unknown(), e.Entity)}
	ev.AddFlags(worldguard.FlagOtherExplosion)
	switch e.Entity.Type() {
	case registry.EntityTypeItemFrame, registry.EntityTypeGlowItemFrame:
		ev.AddFlags(worldguard.FlagEntityItemFrameDestroy)
	case registry.EntityTypePainting:
		ev.AddFlags(worldguard.FlagEntityPaintingDestroy)
	}
	p.fireToCancel(e, ev)
}

func (p *protection) onVehicle(e *plugin.VehicleEvent) {
	cause := event.Unknown()
	if e.Entity != nil {
		cause = p.cause(e.Entity)
	}
	switch e.Action {
	case plugin.VehicleDestroy:
		p.fireToCancel(e, &event.DestroyEntityEvent{Entity: event.NewEntity(e, cause, e.Vehicle)})
	case plugin.VehicleDamage:
		p.fireToCancel(e, &event.DamageEntityEvent{Entity: event.NewEntity(e, cause, e.Vehicle)})
	}
}

func (p *protection) onFish(e *plugin.PlayerFishEvent) {
	cause := p.cause(e.Player, e.Hook)
	switch e.State {
	case plugin.FishCast:
		rod := e.Player.Equipment().Get(plugin.SlotMainHand)
		if rod.Item != registry.ItemFishingRod {
			rod = e.Player.Equipment().Get(plugin.SlotOffHand)
		}
		if p.fireAndTest(&event.UseItemEvent{Original: e, Cause: cause, World: e.Player.World(), Item: rod}) {
			e.SetCancelled(true)
		}
	case plugin.FishCaught:
		if e.Experience > 0 && e.Hook != nil {
			xp := &event.SpawnEntityEvent{Original: e, Cause: cause,
				World: e.Hook.World(), Target: e.Hook.Location(), Type: registry.EntityTypeExperienceOrb}
			if p.fireAndTest(xp) {
				e.Experience = 0
			}
		}
	case plugin.FishCaughtEntity:
		caught := e.Caught
		if caught == nil {
			return
		}
		_, isPlayer := caught.(plugin.Player)
		switch {
		case caught.Type() == registry.EntityTypeItem:
			p.fireToCancel(e, &event.DestroyEntityEvent{Entity: event.NewEntity(e, cause, caught)})
		case buildingIfUsed(caught):
			p.fireToCancel(e, &event.UseEntityEvent{Entity: event.NewEntity(e, cause, caught)})
		case !isHostile(caught) || isPlayer:
			p.fireToCancel(e, &event.DamageEntityEvent{Entity: event.NewEntity(e, cause, caught)})
		}
	}
}

func (p *protection) onProjectileHit(e *plugin.ProjectileHitEvent) {
	if e.Projectile.Type() == registry.EntityTypeFishingBobber && e.Hit.Entity != nil {
		p.fireToCancel(e, &event.DamageEntityEvent{Entity: event.NewEntity(e, p.cause(e.Projectile), e.Hit.Entity)})
	}
}

func (p *protection) onExpBottle(e *plugin.ExpBottleEvent) {
	xp := &event.SpawnEntityEvent{Original: e, Cause: p.cause(e.Bottle),
		World: e.Bottle.World(), Target: e.Bottle.Location(), Type: registry.EntityTypeExperienceOrb}
	if !p.fireAndTest(xp) {
		return
	}
	e.Experience = 0
	if shooter, ok := e.Bottle.Shooter(); ok {
		if pl, ok := shooter.(plugin.Player); ok && pl.GameMode() != plugin.Creative {
			pl.Inventory().Add(plugin.NewStack(registry.ItemExperienceBottle, 1))
		}
	}
}

func (p *protection) onDeathExperience(e plugin.Event, dead plugin.LivingEntity, xp *int) {
	if *xp <= 0 || dead == nil {
		return
	}
	ev := &event.SpawnEntityEvent{Original: e, Cause: p.cause(dead),
		World: dead.World(), Target: dead.Location(), Type: registry.EntityTypeExperienceOrb}
	if p.fireAndTest(ev) {
		*xp = 0
	}
}

func (p *protection) onInteractEntity(e *plugin.PlayerInteractEntityEvent) {
	pl := e.Player
	slot := plugin.SlotMainHand
	if e.Hand == plugin.OffHand {
		slot = plugin.SlotOffHand
	}
	item := pl.Equipment().Get(slot)
	cause := p.cause(pl)
	if p.fireToCancel(e, &event.UseItemEvent{Original: e, Cause: cause, World: pl.World(), Item: item}) {
		return
	}
	use := &event.UseEntityEvent{Entity: event.NewEntity(e, cause, e.Target)}
	if i, ok := itemOfEntity(e.Target.Type()); ok && p.interactBypassItem(pl.World(), i) {
		use.SetAllowed(true)
	}
	p.fireToCancel(e, use)
}

// onEntityDamage judges a hurt dealt by an entity or a block, and the
// item a player hits with. WorldGuard: onEntityDamage
func (p *protection) onEntityDamage(e *plugin.EntityDamageEvent) {
	damager := e.Source.Direct
	if damager == nil {
		damager = e.Source.Causing
	}
	if damager == nil {
		if e.Source.Pos != nil {
			b := plugin.Block{World: e.Entity.World(), Pos: e.Source.Pos.Block()}
			p.fireToCancel(e, &event.DamageEntityEvent{Entity: event.NewEntity(e, p.cause(b), e.Entity)})
		}
		return
	}
	ev := &event.DamageEntityEvent{Entity: event.NewEntity(e, p.cause(damager), e.Entity)}
	switch damager.Type() {
	case registry.EntityTypeFireworkRocket:
		ev.AddFlags(worldguard.FlagFireworkDamage)
	case registry.EntityTypeCreeper:
		ev.AddFlags(worldguard.FlagCreeperExplosion)
	}
	if p.fireToCancel(e, ev) {
		if m, ok := damager.(plugin.Tameable); ok {
			m.SetTarget(nil)
		}
	}
	if pl, ok := damager.(plugin.Player); ok {
		if item := pl.Equipment().Get(plugin.SlotMainHand); !item.Empty() {
			p.fireToCancel(e, &event.UseItemEvent{Original: e, Cause: p.cause(pl), World: e.Entity.World(), Item: item})
		}
	}
}

func (p *protection) onEntityTame(e *plugin.EntityTameEvent) {
	p.fireToCancel(e, &event.UseEntityEvent{Entity: event.NewEntity(e, p.cause(e.Owner), e.Entity)})
}

func (p *protection) onShearEntity(e *plugin.PlayerShearEntityEvent) {
	if e.Player == nil {
		return
	}
	p.fireToCancel(e, &event.UseEntityEvent{Entity: event.NewEntity(e, p.cause(e.Player), e.Target)})
}

func (p *protection) onPickupItem(e *plugin.EntityPickupItemEvent) {
	p.pickup.fire(p, [2]any{e.Entity.UUID(), e.Item.UUID()}, e,
		&event.DestroyEntityEvent{Entity: event.NewEntity(e, p.cause(e.Entity), e.Item)})
}

func (p *protection) onDropItem(e *plugin.PlayerDropItemEvent) {
	p.fireToCancel(e, &event.SpawnEntityEvent{Entity: event.NewEntity(e, p.cause(e.Player), e.Item), Type: e.Item.Type()})
}

func (p *protection) onItemConsume(e *plugin.PlayerItemConsumeEvent) {
	p.fireToCancel(e, &event.UseItemEvent{Original: e, Cause: p.cause(e.Player), World: e.Player.World(), Item: e.Item})
}

func (p *protection) onInventoryOpen(e *plugin.InventoryOpenEvent) {
	if e.View == nil || e.View.Top() == nil {
		return
	}
	holder := e.View.Top().Holder()
	if ent, ok := holder.(plugin.Entity); ok && ent.UUID() == e.Player.UUID() {
		return
	}
	p.holderUse(e, p.cause(e.Player), holder)
}

// holderUse judges using what holds an inventory: an entity, a block or
// both halves of a double chest. WorldGuard: handleInventoryHolderUse
func (p *protection) holderUse(original interface {
	plugin.Event
	Cancelled() bool
	SetCancelled(bool)
}, cause event.Cause, holder any) {
	if original.Cancelled() {
		return
	}
	switch h := holder.(type) {
	case plugin.Entity:
		use := &event.UseEntityEvent{Entity: event.NewEntity(original, cause, h)}
		if i, ok := itemOfEntity(h.Type()); ok && p.interactBypassItem(h.World(), i) {
			use.SetAllowed(true)
		}
		p.fireToCancel(original, use)
	case plugin.BlockEntity:
		b := plugin.Block{World: h.World(), Pos: h.Pos()}
		use := &event.UseBlockEvent{Blocks: event.NewBlocks(original, cause, b.World, b.State().Block(), b.Pos)}
		use.SetAllowed(p.interactBypass(b))
		p.fireToCancel(original, use)
		for _, c := range connected(b) {
			p.fireToCancel(original, &event.UseBlockEvent{Blocks: event.NewBlocks(original, cause, c.World, c.State().Block(), c.Pos)})
		}
	}
}

// onInventoryMove judges a hopper or hopper minecart moving items between
// containers. The server does not say which side started the move, so a
// hopper or hopper minecart among the two stands in. WorldGuard: onInventoryMoveItem
func (p *protection) onInventoryMove(e *plugin.InventoryMoveItemEvent) {
	src, dst := e.Source.Holder(), e.Destination.Holder()
	initiator := dst
	if isHopper(src) || !isHopper(dst) {
		initiator = src
	}
	if !isHopper(initiator) {
		return
	}
	w := holderWorld(initiator)
	if w == nil {
		return
	}
	cfg := p.cfg(w)
	if cfg.IgnoreHopperMoveEvents {
		return
	}
	key := [3]any{holderKey(initiator), holderKey(src), holderKey(dst)}
	if cancelled, ok := p.moveItem.lookup(key); ok {
		if cancelled {
			e.SetCancelled(true)
		}
		return
	}
	var cause event.Cause
	switch h := initiator.(type) {
	case plugin.Entity:
		cause = p.cause(h)
	case plugin.BlockEntity:
		cause = p.cause(plugin.Block{World: h.World(), Pos: h.Pos()})
	}
	if holderKey(src) != holderKey(initiator) {
		p.holderUse(e, cause, src)
	}
	if holderKey(dst) != holderKey(initiator) {
		p.holderUse(e, cause, dst)
	}
	if be, ok := initiator.(plugin.BlockEntity); ok && e.Cancelled() && cfg.BreakDeniedHoppers {
		b := plugin.Block{World: be.World(), Pos: be.Pos()}
		p.wg.Server.Scheduler().RunTask(func() { b.BreakNaturally() })
		return
	}
	p.moveItem.store(key, e.Cancelled())
}

func isHopper(h any) bool {
	switch x := h.(type) {
	case plugin.Entity:
		return x.Type() == registry.EntityTypeHopperMinecart
	case plugin.BlockEntity:
		return x.World().BlockState(x.Pos()).Block() == registry.BlockHopper
	}
	return false
}

func holderWorld(h any) plugin.World {
	switch x := h.(type) {
	case plugin.Entity:
		return x.World()
	case plugin.BlockEntity:
		return x.World()
	}
	return nil
}

func holderKey(h any) any {
	switch x := h.(type) {
	case plugin.Entity:
		return x.UUID()
	case plugin.BlockEntity:
		return plugin.Block{World: x.World(), Pos: x.Pos()}
	}
	return h
}

// onDispense judges a dispenser using its item on the block in front.
// WorldGuard: onBlockDispense
func (p *protection) onDispense(e *plugin.BlockDispenseEvent) {
	s := e.Block.State()
	if s.Block() != registry.BlockDispenser {
		return
	}
	cause := p.cause(e.Block)
	if p.fireToCancel(e, &event.UseItemEvent{Original: e, Cause: cause, World: e.Block.World, Item: e.Item}) {
		return
	}
	facing, _ := s.Prop(registry.PropFacing)
	f, ok := faceNamed(facing)
	if !ok {
		return
	}
	placed := e.Block.Relative(f, 1)
	clicked := placed.Relative(f, 1)
	p.blockRightClick(e, cause, e.Item, clicked, placed)
	switch {
	case isShulkerBoxItem(e.Item.Item):
		b, _ := e.Item.Item.Block()
		if p.fireToCancel(e, &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, cause, placed.World, b, placed.Pos)}) {
			p.denyEffect(nil, placed.World, placed.Pos.Center())
		}
	case p.itemApplied(e.Item.Item, placed):
		if p.fireToCancel(e, &event.PlaceBlockEvent{Blocks: event.NewBlocks(e, cause, placed.World, placed.State().Block(), placed.Pos)}) {
			p.denyEffect(nil, placed.World, placed.Pos.Center())
		}
	}
}

func (p *protection) onCauldronChange(e *plugin.CauldronLevelChangeEvent) {
	if e.Entity == nil {
		return
	}
	use := &event.UseBlockEvent{Blocks: event.NewBlocks(e, p.cause(e.Entity), e.Block.World, e.Block.State().Block(), e.Block.Pos)}
	use.SetAllowed(p.interactBypass(e.Block))
	p.interact.fire(p, [2]any{e.Block, e.Entity.UUID()}, e, use)
}

// onEntityTransform judges lightning turning a mob into another. The bolt
// is not known, so the mob's own block stands in for it. WorldGuard:
// PaperListener.onEntityTransform (EntityZapEvent)
func (p *protection) onEntityTransform(e *plugin.EntityTransformEvent) {
	if e.Reason != plugin.TransformLightning {
		return
	}
	b := plugin.Block{World: e.Entity.World(), Pos: e.Entity.Pos().Block()}
	p.fireToCancel(e, &event.DamageEntityEvent{Entity: event.NewEntity(e, p.cause(b), e.Entity)})
}
