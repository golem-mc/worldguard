package worldguardplugin

import (
	"strings"
	"sync"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// The bits of a material's flags. WorldGuard: Materials
const (
	modifiedOnRight = 1
	modifiedOnLeft  = 2
	modifiesBlocks  = 4
)

// materialTables are Materials' lookups, built on first use: the tags
// they read are loaded with the server.
type materialTables struct {
	blockFlags map[registry.Block]uint8
	itemFlags  map[registry.Item]uint8

	consideredBuilding, inventory, useFlag, bed, anvil, coral, crop, vine, water, unwaxedCopper, waxedCopper *[registry.BlockCount]bool

	boatItems, chestBoatItems, cushionItems map[registry.Item]bool
	itemEntity                              map[registry.Item]registry.EntityType
	entityItem                              map[registry.EntityType]registry.Item
}

var (
	materialsOnce sync.Once
	materials     materialTables
)

func mats() *materialTables {
	materialsOnce.Do(materials.build)
	return &materials
}

func shortName(name string) string { return strings.TrimPrefix(name, "minecraft:") }

func blockTag(name string) registry.BlockTagID { return registry.BlockTagOf("minecraft:" + name) }

func itemTag(name string) registry.ItemTagID { return registry.ItemTagOf("minecraft:" + name) }

func inBlockTags(b registry.Block, tags ...string) bool {
	for _, t := range tags {
		if b.InTag(blockTag(t)) {
			return true
		}
	}
	return false
}

func (m *materialTables) build() {
	m.blockFlags = map[registry.Block]uint8{}
	m.itemFlags = map[registry.Item]uint8{}
	for name, f := range materialFlagNames {
		if b, ok := registry.BlockByName("minecraft:" + name); ok {
			m.blockFlags[b] = f
		}
		if i, ok := registry.ItemByName("minecraft:" + name); ok {
			m.itemFlags[i] = f
		}
	}
	for b := range registry.Block(registry.BlockCount) {
		name := shortName(b.Name())
		for _, t := range materialFlagTags {
			if !t.item && b.InTag(blockTag(t.tag)) || nameFallback(t.tag, name) {
				m.blockFlags[b] = t.flags
			}
		}
		if inBlockTags(b, "coral_blocks", "corals", "wall_corals") || strings.HasPrefix(name, "dead_") && strings.Contains(name, "coral") {
			m.blockFlags[b] = 0
		}
	}
	m.itemEntity = map[registry.Item]registry.EntityType{}
	m.entityItem = map[registry.EntityType]registry.Item{}
	for e, i := range entityItems {
		et, ok1 := registry.EntityTypeByName("minecraft:" + e)
		it, ok2 := registry.ItemByName("minecraft:" + i)
		if ok1 && ok2 {
			m.entityItem[et], m.itemEntity[it] = it, et
		}
	}
	m.boatItems, m.chestBoatItems, m.cushionItems = map[registry.Item]bool{}, map[registry.Item]bool{}, map[registry.Item]bool{}
	for i := range registry.Item(registry.ItemCount) {
		name := shortName(i.Name())
		if b, ok := i.Block(); ok {
			if f, ok := m.blockFlags[b]; ok {
				if _, set := m.itemFlags[i]; !set {
					m.itemFlags[i] = f
				}
			}
		}
		for _, t := range materialFlagTags {
			if t.item && i.InTag(itemTag(t.tag)) || nameFallback(t.tag, name) {
				m.itemFlags[i] = t.flags
			}
		}
		if strings.HasSuffix(name, "_spawn_egg") || strings.HasPrefix(name, "music_disc_") || strings.HasSuffix(name, "_smithing_template") {
			m.itemFlags[i] = 0
		}
		if i.InTag(itemTag("boats")) || strings.HasSuffix(name, "_boat") || strings.HasSuffix(name, "_raft") {
			m.boatItems[i] = true
			if et, ok := registry.EntityTypeByName(i.Name()); ok {
				m.entityItem[et], m.itemEntity[i] = i, et
			}
		}
		if i.InTag(itemTag("chest_boats")) || strings.HasSuffix(name, "_chest_boat") || strings.HasSuffix(name, "_chest_raft") {
			m.chestBoatItems[i] = true
		}
		if i.InTag(itemTag("cushions")) || strings.HasSuffix(name, "_cushion") {
			m.cushionItems[i] = true
		}
	}
	m.consideredBuilding = registry.BlocksOf(func(b registry.Block) bool {
		switch b {
		case registry.BlockRepeater, registry.BlockComparator, registry.BlockCake, registry.BlockDragonEgg:
			return true
		}
		return inBlockTags(b, "flower_pots", "candles", "candle_cakes", "all_signs")
	})
	m.inventory = registry.BlocksOf(func(b registry.Block) bool {
		switch b {
		case registry.BlockChest, registry.BlockJukebox, registry.BlockDispenser, registry.BlockFurnace,
			registry.BlockBrewingStand, registry.BlockTrappedChest, registry.BlockHopper, registry.BlockDropper,
			registry.BlockBarrel, registry.BlockBlastFurnace, registry.BlockSmoker, registry.BlockChiseledBookshelf,
			registry.BlockCrafter, registry.BlockDecoratedPot:
			return true
		}
		name := shortName(b.Name())
		return inBlockTags(b, "wooden_shelves", "copper_chests", "shulker_boxes") ||
			strings.HasSuffix(name, "_shelf") || strings.HasSuffix(name, "copper_chest") || strings.HasSuffix(name, "shulker_box")
	})
	m.useFlag = registry.BlocksOf(func(b registry.Block) bool {
		switch shortName(b.Name()) {
		case "lever", "lectern", "enchanting_table", "bell", "loom", "cartography_table", "stonecutter", "grindstone", "vault":
			return true
		}
		return inBlockTags(b, "buttons", "doors", "trapdoors", "fence_gates", "pressure_plates")
	})
	m.bed = registry.BlocksOf(func(b registry.Block) bool {
		return inBlockTags(b, "beds") || shortName(b.Name()) == "straw_bed"
	})
	m.anvil = registry.BlocksOf(func(b registry.Block) bool { return inBlockTags(b, "anvil") })
	m.coral = registry.BlocksOf(func(b registry.Block) bool {
		return inBlockTags(b, "coral_blocks", "coral_plants", "corals", "wall_corals")
	})
	m.crop = registry.BlocksOf(func(b registry.Block) bool {
		switch shortName(b.Name()) {
		case "pumpkin", "melon", "cactus", "sugar_cane", "bamboo", "bamboo_sapling", "sweet_berry_bush",
			"nether_wart", "cave_vines", "cave_vines_plant", "cocoa":
			return true
		}
		return inBlockTags(b, "crops")
	})
	m.vine = registry.BlocksOf(func(b registry.Block) bool {
		switch shortName(b.Name()) {
		case "vine", "kelp", "twisting_vines", "weeping_vines":
			return true
		}
		return inBlockTags(b, "cave_vines")
	})
	m.water = registry.BlocksOf(func(b registry.Block) bool {
		switch b {
		case registry.BlockWater, registry.BlockBubbleColumn, registry.BlockKelpPlant, registry.BlockSeagrass, registry.BlockTallSeagrass:
			return true
		}
		return false
	})
	m.waxedCopper = registry.BlocksOf(func(b registry.Block) bool {
		name := shortName(b.Name())
		return strings.HasPrefix(name, "waxed_") && strings.Contains(name, "copper")
	})
	m.unwaxedCopper = registry.BlocksOf(func(b registry.Block) bool {
		switch name := shortName(b.Name()); name {
		case "copper_block", "chiseled_copper", "copper_door", "copper_trapdoor", "copper_grate", "copper_bulb",
			"copper_bars", "copper_torch", "copper_chest", "copper_chain", "copper_wall_torch", "copper_lantern",
			"copper_golem_statue":
			return true
		default:
			return strings.HasPrefix(name, "exposed_") || strings.HasPrefix(name, "weathered_") ||
				strings.HasPrefix(name, "oxidized_") || strings.HasPrefix(name, "cut_copper")
		}
	})
}

// nameFallback covers a tag the server does not know (Bukkit's own, or one
// of a newer version) by the names of its members.
func nameFallback(tag, name string) bool {
	if registry.BlockTagOf("minecraft:"+tag) != registry.BlockTagOf("minecraft:no/such/tag") ||
		registry.ItemTagOf("minecraft:"+tag) != registry.ItemTagOf("minecraft:no/such/tag") {
		return false
	}
	switch tag {
	case "concrete":
		return strings.HasSuffix(name, "_concrete")
	case "concrete_powders":
		return strings.HasSuffix(name, "_concrete_powder")
	case "glazed_terracotta":
		return strings.HasSuffix(name, "_glazed_terracotta")
	case "clonable_maps":
		return name == "filled_map"
	case "bars":
		return strings.HasSuffix(name, "_bars")
	case "chains":
		return name == "chain" || strings.HasSuffix(name, "_chain")
	case "copper_chests":
		return strings.HasSuffix(name, "copper_chest")
	case "copper_golem_statues":
		return strings.HasSuffix(name, "copper_golem_statue")
	case "lanterns":
		return strings.HasSuffix(name, "lantern")
	case "lightning_rods":
		return strings.HasSuffix(name, "lightning_rod")
	case "wooden_shelves":
		return strings.HasSuffix(name, "_shelf")
	case "spears":
		return strings.HasSuffix(name, "_spear")
	case "harnesses":
		return strings.HasSuffix(name, "_harness")
	case "cushions":
		return strings.HasSuffix(name, "_cushion")
	}
	return false
}

// blockModifiedOnClick reports whether clicking b changes the world, so
// that it needs more than the right to use it. WorldGuard:
// Materials.isBlockModifiedOnClick
func blockModifiedOnClick(b registry.Block, right bool) bool {
	f, ok := mats().blockFlags[b]
	return !ok || right && f&modifiedOnRight != 0 || !right && f&modifiedOnLeft != 0
}

// itemAppliedToBlock reports whether using item on b changes b.
// WorldGuard: Materials.isItemAppliedToBlock
func itemAppliedToBlock(item registry.Item, b registry.Block) bool {
	f, ok := mats().itemFlags[item]
	return !ok || f&modifiesBlocks != 0 || toolApplicable(item, b)
}

// toolApplicable reports whether a tool changes b: a hoe tilling, an axe
// stripping, dye on a sign. WorldGuard: Materials.isToolApplicable
func toolApplicable(item registry.Item, b registry.Block) bool {
	name, target := shortName(item.Name()), shortName(b.Name())
	switch {
	case strings.HasSuffix(name, "_hoe"):
		return target == "grass_block" || target == "dirt" || target == "dirt_path" || target == "rooted_dirt"
	case strings.HasSuffix(name, "_axe"):
		if mats().waxedCopper[b] || inBlockTags(b, "logs") {
			return true
		}
		switch target {
		case "oak_wood", "dark_oak_wood", "acacia_wood", "birch_wood", "spruce_wood", "pumpkin", "bamboo_block",
			"jungle_wood", "crimson_stem", "warped_stem", "crimson_hyphae", "warped_hyphae":
			return true
		}
		return false
	case strings.HasSuffix(name, "_shovel"):
		return target == "grass_block" || target == "campfire" || target == "soul_campfire"
	case name == "shears":
		return target == "pumpkin" || target == "bee_nest" || target == "beehive"
	case strings.HasSuffix(name, "_dye") || name == "glow_ink_sac" || name == "ink_sac":
		return inBlockTags(b, "all_signs")
	case name == "honeycomb":
		return mats().unwaxedCopper[b] || inBlockTags(b, "all_signs")
	case name == "brush":
		return target == "suspicious_gravel" || target == "suspicious_sand"
	case name == "written_book" || name == "writable_book":
		return b == registry.BlockLectern
	}
	return false
}

func consideredBuildingIfUsed(b registry.Block) bool { return mats().consideredBuilding[b] }
func inventoryBlock(b registry.Block) bool           { return mats().inventory[b] }
func useFlagApplicable(b registry.Block) bool        { return mats().useFlag[b] }
func isBed(b registry.Block) bool                    { return mats().bed[b] }
func isAnvil(b registry.Block) bool                  { return mats().anvil[b] }
func isCoral(b registry.Block) bool                  { return mats().coral[b] }
func isCrop(b registry.Block) bool                   { return mats().crop[b] }
func isVine(b registry.Block) bool                   { return mats().vine[b] }
func isWater(b registry.Block) bool                  { return mats().water[b] }
func isUnwaxedCopper(b registry.Block) bool          { return mats().unwaxedCopper[b] }
func isLava(b registry.Block) bool                   { return b == registry.BlockLava }
func isLiquid(b registry.Block) bool                 { return isWater(b) || isLava(b) }
func isFire(b registry.Block) bool                   { return b == registry.BlockFire || b == registry.BlockSoulFire }
func isMushroom(b registry.Block) bool {
	return b == registry.BlockRedMushroom || b == registry.BlockBrownMushroom
}
func isPistonBlock(b registry.Block) bool {
	return b == registry.BlockPiston || b == registry.BlockStickyPiston || b == registry.BlockMovingPiston
}
func isAmethystGrowth(b registry.Block) bool {
	switch b {
	case registry.BlockBuddingAmethyst, registry.BlockAmethystCluster, registry.BlockLargeAmethystBud,
		registry.BlockMediumAmethystBud, registry.BlockSmallAmethystBud:
		return true
	}
	return false
}
func isSculkGrowth(b registry.Block) bool {
	return b == registry.BlockSculk || b == registry.BlockSculkVein
}
func isCauldron(b registry.Block) bool {
	switch b {
	case registry.BlockCauldron, registry.BlockLavaCauldron, registry.BlockPowderSnowCauldron, registry.BlockWaterCauldron:
		return true
	}
	return false
}

// waterloggable reports whether b holds water in its state.
func waterloggable(s plugin.BlockState) bool {
	_, ok := s.Prop(registry.PropWaterlogged)
	return ok
}

func isMinecartItem(i registry.Item) bool {
	switch i {
	case registry.ItemMinecart, registry.ItemCommandBlockMinecart, registry.ItemTntMinecart, registry.ItemHopperMinecart,
		registry.ItemFurnaceMinecart, registry.ItemChestMinecart:
		return true
	}
	return false
}
func isBoatItem(i registry.Item) bool    { return mats().boatItems[i] || mats().chestBoatItems[i] }
func isCushionItem(i registry.Item) bool { return mats().cushionItems[i] }
func isShulkerBoxItem(i registry.Item) bool {
	b, ok := i.Block()
	return ok && strings.HasSuffix(b.Name(), "shulker_box")
}

// spawnEggEntity answers the entity a spawn egg spawns. WorldGuard:
// Materials.getEntitySpawnEgg
func spawnEggEntity(i registry.Item) (registry.EntityType, bool) {
	name, ok := strings.CutSuffix(i.Name(), "_spawn_egg")
	if !ok {
		return 0, false
	}
	return registry.EntityTypeByName(name)
}

// entityOfItem answers the entity an item places. WorldGuard:
// Materials.getRelatedEntity
func entityOfItem(i registry.Item) (registry.EntityType, bool) {
	t, ok := mats().itemEntity[i]
	return t, ok
}

// itemOfEntity answers the item that places an entity. WorldGuard:
// Materials.getRelatedMaterial
func itemOfEntity(t registry.EntityType) (registry.Item, bool) {
	i, ok := mats().entityItem[t]
	return i, ok
}

// bucketBlock answers the block a bucket empties. WorldGuard:
// Materials.getBucketBlockMaterial
func bucketBlock(i registry.Item) registry.Block {
	switch i {
	case registry.ItemLavaBucket:
		return registry.BlockLava
	case registry.ItemPowderSnowBucket:
		return registry.BlockPowderSnow
	}
	return registry.BlockWater
}

func isCauldronBucket(i registry.Item) bool {
	return i == registry.ItemWaterBucket || i == registry.ItemLavaBucket || i == registry.ItemPowderSnowBucket
}

// isBlockItem reports whether i places a block.
func isBlockItem(i registry.Item) bool {
	_, ok := i.Block()
	return ok
}

// isAir reports whether b is any kind of air.
func isAir(b registry.Block) bool {
	return b == registry.BlockAir || b == registry.BlockCaveAir || b == registry.BlockVoidAir
}
