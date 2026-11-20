package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/internal/yamlfile"
)

// World is one world's configuration: its own file's values over
// config.yml's. WorldGuard: WorldConfiguration, YamlWorldConfiguration,
// BukkitWorldConfiguration
type World struct {
	Name string
	Dir  string
	// BlacklistFile is worlds/<world>/blacklist.txt.
	BlacklistFile string
	// Blacklist is the loaded blacklist, set by Manager.OnWorldLoad; nil
	// when it is empty.
	Blacklist any

	parent, file *yamlfile.File

	SummaryOnStart             bool
	OpPermissions              bool
	BuildPermissions           bool
	BuildPermissionDenyMessage string
	StrictEntitySpawn          bool
	AllowAllInteract           TargetSet
	BlockUseAtFeet             TargetSet
	IgnoreHopperMoveEvents     bool
	BreakDeniedHoppers         bool
	UsePaperEntityOrigin       bool
	ItemDurability             bool
	RemoveInfiniteStacks       bool
	DisableExpDrops            bool
	UseMaxPriorityAssociation  bool
	BlockPotions               []string
	BlockPotionsAlways         bool
	DisableConduitEffects      bool
	SimulateSponge             bool
	SpongeRadius               int
	RedstoneSponges            bool
	PumpkinScuba               bool
	DisableHealthRegain        bool
	NoPhysicsGravel            bool
	NoPhysicsSand              bool
	RopeLadders                bool
	AllowPortalAnywhere        bool
	PreventWaterDamage         map[registry.Block]bool
	BlockTNTExplosions         bool
	BlockTNTBlockDamage        bool
	BlockLighter               bool
	PreventLavaFire            bool
	DisableFireSpread          bool
	DisableFireSpreadBlocks    map[registry.Block]bool
	AllowedLavaSpreadOver      map[registry.Block]bool

	BlockCreeperExplosions        bool
	BlockCreeperBlockDamage       bool
	BlockWitherExplosions         bool
	BlockWitherBlockDamage        bool
	BlockWitherSkullExplosions    bool
	BlockWitherSkullBlockDamage   bool
	BlockEnderDragonBlockDamage   bool
	BlockEnderDragonPortalCreate  bool
	BlockFireballExplosions       bool
	BlockFireballBlockDamage      bool
	BlockWindChargeExplosions     bool
	AntiWolfDumbness              bool
	AllowTamedSpawns              bool
	DisableEndermanGriefing       bool
	DisableSnowmanTrails          bool
	BlockEntityPaintingDestroy    bool
	BlockEntityItemFrameDestroy   bool
	BlockEntityArmorStandDestroy  bool
	BlockPluginSpawning           bool
	BlockGroundSlimes             bool
	BlockOtherExplosions          bool
	BlockZombieDoorDestruction    bool
	BlockEntityVehicleEntry       bool
	DisableFallDamage             bool
	DisableLavaDamage             bool
	DisableFireDamage             bool
	DisableLightningDamage        bool
	DisableDrowningDamage         bool
	DisableSuffocationDamage      bool
	DisableContactDamage          bool
	TeleportOnSuffocation         bool
	DisableVoidDamage             bool
	TeleportOnVoid                bool
	SafeFallOnVoid                bool
	DisableExplosionDamage        bool
	DisableMobDamage              bool
	DisableDeathMessages          bool
	SignChestProtection           bool
	DisableSignChestProtectionChk bool

	DisableCreatureCropTrampling      bool
	DisablePlayerCropTrampling        bool
	DisableCreatureTurtleEggTrampling bool
	DisablePlayerTurtleEggTrampling   bool
	DisableCreatureSnifferEggTrample  bool
	DisablePlayerSnifferEggTrample    bool
	DisallowedLightningBlocks         map[registry.Block]bool
	PreventLightningFire              bool
	DisableThunder                    bool
	DisableWeather                    bool
	DisablePigZap                     bool
	DisableVillagerZap                bool
	DisableCreeperPower               bool
	AlwaysRaining                     bool
	AlwaysThundering                  bool
	DisableMushroomSpread             bool
	DisableIceMelting                 bool
	DisableSnowMelting                bool
	DisableSnowFormation              bool
	DisableIceFormation               bool
	DisableLeafDecay                  bool
	DisableGrassGrowth                bool
	DisableMyceliumSpread             bool
	DisableVineGrowth                 bool
	DisableRockGrowth                 bool
	DisableSculkGrowth                bool
	DisableCropGrowth                 bool
	DisableSoilDehydration            bool
	DisableSoilMoistureChange         bool
	DisableCoralBlockFade             bool
	DisableCopperBlockFade            bool
	DisableLavaHarden                 bool
	AllowedSnowFallOver               map[registry.Block]bool

	UseRegions                    bool
	RegionInvincibilityRemovesMob bool
	RegionCancelEmptyChatEvents   bool
	RegionNetherPortalProtection  bool
	ForceDefaultTitleTimes        bool
	FakePlayerBuildOverride       bool
	ExplosionFlagCancellation     bool
	HighFreqFlags                 bool
	CheckLiquidFlow               bool
	RegionWand                    registry.Item
	MaxClaimVolume                int
	ClaimOnlyInsideExisting       bool
	SetParentOnClaim              string
	BoundedLocationFlags          bool
	MaxRegionCountPerPlayer       int
	maxRegionCounts               map[string]int
	BlockCreatureSpawn            map[registry.EntityType]bool

	UseBlacklistAsWhitelist bool
	LogConsole              bool
	LogDatabase             bool
	LogDatabaseDSN          string
	LogDatabaseUser         string
	LogDatabasePass         string
	LogDatabaseTable        string
	LogFile                 bool
	LogFilePattern          string
	LogFileCacheSize        int
}

// TargetSet is a list of blocks and items a setting names. WorldGuard:
// TargetMatcherSet
type TargetSet struct {
	Blocks map[registry.Block]bool
	Items  map[registry.Item]bool
}

// HasBlock reports whether the set names b.
func (t TargetSet) HasBlock(b registry.Block) bool { return t.Blocks[b] }

// HasItem reports whether the set names i.
func (t TargetSet) HasItem(i registry.Item) bool { return t.Items[i] }

func (w *World) has(path string) bool { return w.file.Has(path) }

func (w *World) bool(path string, def bool) bool {
	v := w.parent.Bool(path, def)
	if w.has(path) {
		return w.file.Bool(path, def)
	}
	return v
}

func (w *World) string(path, def string) string {
	v := w.parent.String(path, def)
	if w.has(path) {
		return w.file.String(path, def)
	}
	return v
}

func (w *World) int(path string, def int) int {
	v := w.parent.Int(path, def)
	if w.has(path) {
		return w.file.Int(path, def)
	}
	return v
}

// stringList reads a list, storing an empty one in config.yml when it has
// none. WorldGuard: YamlWorldConfiguration.getStringList
func (w *World) stringList(path string) []string {
	res := w.parent.StringList(path, nil)
	if len(res) == 0 {
		w.parent.Set(path, []any{})
	}
	if w.has(path) {
		res = w.file.StringList(path, nil)
	}
	return res
}

func (w *World) keys(path string) []string {
	res := w.parent.Keys(path)
	if len(res) == 0 {
		res = w.file.Keys(path)
	}
	return res
}

func (w *World) property(path string) (any, bool) {
	if v, ok := w.file.Get(path); ok {
		return v, true
	}
	return w.parent.Get(path)
}

// removeProperty drops a retired setting from both files and reports
// whether config.yml had it.
func (w *World) removeProperty(path string) bool {
	if w.has(path) {
		w.file.Remove(path)
	}
	if w.parent.Has(path) {
		w.parent.Remove(path)
		return true
	}
	return false
}

func (w *World) targets(path string, m *Manager) TargetSet {
	set := TargetSet{Blocks: map[registry.Block]bool{}, Items: map[registry.Item]bool{}}
	for _, in := range w.stringList(path) {
		b, okB := ParseBlock(in)
		it, okI := ParseItem(in)
		if okB {
			set.Blocks[b] = true
		}
		if okI {
			set.Items[it] = true
		}
		if !okB && !okI {
			m.Log.Warn(fmt.Sprintf("Failed to parse the block / item type specified as '%s'", in))
		}
	}
	return set
}

func (m *Manager) loadWorld(name string) *World {
	dir := filepath.Join(m.WorldsDir(), name)
	path := filepath.Join(dir, "config.yml")
	w := &World{Name: name, Dir: dir, BlacklistFile: filepath.Join(dir, "blacklist.txt"), parent: m.file}
	copyDefault(path, "config_world.yml")
	copyDefault(w.BlacklistFile, "blacklist.txt")
	f, err := yamlfile.Load(path, true)
	if err != nil {
		m.Log.Error(fmt.Sprintf("Error reading configuration for world %s: %v", name, err))
	}
	w.file = f
	w.load(m)
	if m.OnWorldLoad != nil {
		m.OnWorldLoad(w)
	}
	if w.SummaryOnStart {
		m.Log.Info(fmt.Sprintf("Loaded configuration for world '%s'", name))
		w.summary(m)
	}
	f.Header = worldHeader
	if err := f.Save(); err != nil {
		m.Log.Error(fmt.Sprintf("Error saving configuration for world %s: %v", name, err))
	}
	return w
}

func (w *World) summary(m *Manager) {
	permitted := func(blocked bool, what string) {
		if blocked {
			m.Log.Info(fmt.Sprintf("(%s) %s is blocked.", w.Name, what))
		} else {
			m.Log.Info(fmt.Sprintf("(%s) %s is PERMITTED.", w.Name, what))
		}
	}
	permitted(w.BlockTNTExplosions, "TNT ignition")
	if w.BlockLighter {
		m.Log.Info(fmt.Sprintf("(%s) Lighters are blocked.", w.Name))
	} else {
		m.Log.Info(fmt.Sprintf("(%s) Lighters are PERMITTED.", w.Name))
	}
	permitted(w.PreventLavaFire, "Lava fire")
	switch {
	case w.DisableFireSpread:
		m.Log.Info(fmt.Sprintf("(%s) All fire spread is disabled.", w.Name))
	case len(w.DisableFireSpreadBlocks) > 0:
		m.Log.Info(fmt.Sprintf("(%s) Fire spread is limited to %d block types.", w.Name, len(w.DisableFireSpreadBlocks)))
	default:
		m.Log.Info(fmt.Sprintf("(%s) Fire spread is UNRESTRICTED.", w.Name))
	}
}

// load reads every setting in WorldGuard's order, so a new config.yml
// lists them as WorldGuard's does. WorldGuard:
// BukkitWorldConfiguration.loadConfiguration
//
//nolint:funlen // one line per setting, in the upstream order
func (w *World) load(m *Manager) {
	needParentSave := false
	w.SummaryOnStart = w.bool("summary-on-start", true)
	w.OpPermissions = w.bool("op-permissions", true)
	w.BuildPermissions = w.bool("build-permission-nodes.enable", false)
	w.BuildPermissionDenyMessage = worldguard.ReplaceColorMacros(
		w.string("build-permission-nodes.deny-message", "&eSorry, but you are not permitted to do that here."))
	w.StrictEntitySpawn = w.bool("event-handling.block-entity-spawns-with-untraceable-cause", false)
	w.AllowAllInteract = w.targets("event-handling.interaction-whitelist", m)
	w.BlockUseAtFeet = w.targets("event-handling.emit-block-use-at-feet", m)
	w.IgnoreHopperMoveEvents = w.bool("event-handling.ignore-hopper-item-move-events", false)
	w.BreakDeniedHoppers = w.bool("event-handling.break-hoppers-on-denied-move", true)
	w.UsePaperEntityOrigin = w.bool("regions.use-paper-entity-origin", false)
	w.ItemDurability = w.bool("protection.item-durability", true)
	w.RemoveInfiniteStacks = w.bool("protection.remove-infinite-stacks", false)
	w.DisableExpDrops = w.bool("protection.disable-xp-orb-drops", false)
	needParentSave = w.removeProperty("protection.disable-obsidian-generators") || needParentSave
	w.UseMaxPriorityAssociation = w.bool("protection.use-max-priority-association", false)
	w.BlockPotions = w.stringList("gameplay.block-potions")
	w.BlockPotionsAlways = w.bool("gameplay.block-potions-overly-reliably", false)
	w.DisableConduitEffects = w.bool("gameplay.disable-conduit-effects", false)
	w.SimulateSponge = w.bool("simulation.sponge.enable", false)
	w.SpongeRadius = max(1, w.int("simulation.sponge.radius", 3)) - 1
	w.RedstoneSponges = w.bool("simulation.sponge.redstone", false)
	if w.SimulateSponge {
		m.Log.Warn("Sponge simulation is deprecated for removal in a future version. We recommend using CraftBook's sponge simulation instead.")
	} else {
		needParentSave = w.removeProperty("simulation") || needParentSave
	}
	w.PumpkinScuba = w.bool("default.pumpkin-scuba", false)
	w.DisableHealthRegain = w.bool("default.disable-health-regain", false)
	w.NoPhysicsGravel = w.bool("physics.no-physics-gravel", false)
	w.NoPhysicsSand = w.bool("physics.no-physics-sand", false)
	w.RopeLadders = w.bool("physics.vine-like-rope-ladders", false)
	w.AllowPortalAnywhere = w.bool("physics.allow-portal-anywhere", false)
	w.PreventWaterDamage = parseBlocks(m.Log, w.stringList("physics.disable-water-damage-blocks"))
	w.BlockTNTExplosions = w.bool("ignition.block-tnt", false)
	w.BlockTNTBlockDamage = w.bool("ignition.block-tnt-block-damage", false)
	w.BlockLighter = w.bool("ignition.block-lighter", false)
	w.PreventLavaFire = w.bool("fire.disable-lava-fire-spread", false)
	w.DisableFireSpread = w.bool("fire.disable-all-fire-spread", false)
	w.DisableFireSpreadBlocks = parseBlocks(m.Log, w.stringList("fire.disable-fire-spread-blocks"))
	w.AllowedLavaSpreadOver = parseBlocks(m.Log, w.stringList("fire.lava-spread-blocks"))
	w.BlockCreeperExplosions = w.bool("mobs.block-creeper-explosions", false)
	w.BlockCreeperBlockDamage = w.bool("mobs.block-creeper-block-damage", false)
	w.BlockWitherExplosions = w.bool("mobs.block-wither-explosions", false)
	w.BlockWitherBlockDamage = w.bool("mobs.block-wither-block-damage", false)
	w.BlockWitherSkullExplosions = w.bool("mobs.block-wither-skull-explosions", false)
	w.BlockWitherSkullBlockDamage = w.bool("mobs.block-wither-skull-block-damage", false)
	w.BlockEnderDragonBlockDamage = w.bool("mobs.block-enderdragon-block-damage", false)
	w.BlockEnderDragonPortalCreate = w.bool("mobs.block-enderdragon-portal-creation", false)
	w.BlockFireballExplosions = w.bool("mobs.block-fireball-explosions", false)
	w.BlockFireballBlockDamage = w.bool("mobs.block-fireball-block-damage", false)
	w.BlockWindChargeExplosions = w.bool("mobs.block-windcharge-explosions", false)
	w.AntiWolfDumbness = w.bool("mobs.anti-wolf-dumbness", false)
	w.AllowTamedSpawns = w.bool("mobs.allow-tamed-spawns", true)
	w.DisableEndermanGriefing = w.bool("mobs.disable-enderman-griefing", false)
	w.DisableSnowmanTrails = w.bool("mobs.disable-snowman-trails", false)
	w.BlockEntityPaintingDestroy = w.bool("mobs.block-painting-destroy", false)
	w.BlockEntityItemFrameDestroy = w.bool("mobs.block-item-frame-destroy", false)
	w.BlockEntityArmorStandDestroy = w.bool("mobs.block-armor-stand-destroy", false)
	w.BlockPluginSpawning = w.bool("mobs.block-plugin-spawning", true)
	w.BlockGroundSlimes = w.bool("mobs.block-above-ground-slimes", false)
	w.BlockOtherExplosions = w.bool("mobs.block-other-explosions", false)
	w.BlockZombieDoorDestruction = w.bool("mobs.block-zombie-door-destruction", false)
	w.BlockEntityVehicleEntry = w.bool("mobs.block-vehicle-entry", false)
	w.DisableFallDamage = w.bool("player-damage.disable-fall-damage", false)
	w.DisableLavaDamage = w.bool("player-damage.disable-lava-damage", false)
	w.DisableFireDamage = w.bool("player-damage.disable-fire-damage", false)
	w.DisableLightningDamage = w.bool("player-damage.disable-lightning-damage", false)
	w.DisableDrowningDamage = w.bool("player-damage.disable-drowning-damage", false)
	w.DisableSuffocationDamage = w.bool("player-damage.disable-suffocation-damage", false)
	w.DisableContactDamage = w.bool("player-damage.disable-contact-damage", false)
	w.TeleportOnSuffocation = w.bool("player-damage.teleport-on-suffocation", false)
	w.DisableVoidDamage = w.bool("player-damage.disable-void-damage", false)
	w.TeleportOnVoid = w.bool("player-damage.teleport-on-void-falling", false)
	w.SafeFallOnVoid = w.bool("player-damage.reset-fall-on-void-teleport", false)
	w.DisableExplosionDamage = w.bool("player-damage.disable-explosion-damage", false)
	w.DisableMobDamage = w.bool("player-damage.disable-mob-damage", false)
	w.DisableDeathMessages = w.bool("player-damage.disable-death-messages", false)
	w.SignChestProtection = w.bool("chest-protection.enable", false)
	w.DisableSignChestProtectionChk = w.bool("chest-protection.disable-off-check", true)
	if w.SignChestProtection {
		m.Log.Warn("Sign-based chest protection is deprecated for removal in a future version. See https://worldguard.enginehub.org/en/latest/chest-protection/ for details.")
	} else {
		needParentSave = w.removeProperty("chest-protection") || needParentSave
	}
	w.DisableCreatureCropTrampling = w.bool("crops.disable-creature-trampling", false)
	w.DisablePlayerCropTrampling = w.bool("crops.disable-player-trampling", false)
	w.DisableCreatureTurtleEggTrampling = w.bool("turtle-egg.disable-creature-trampling", false)
	w.DisablePlayerTurtleEggTrampling = w.bool("turtle-egg.disable-player-trampling", false)
	w.DisableCreatureSnifferEggTrample = w.bool("sniffer-egg.disable-creature-trampling", false)
	w.DisablePlayerSnifferEggTrample = w.bool("sniffer-egg.disable-player-trampling", false)
	w.DisallowedLightningBlocks = parseBlocks(m.Log, w.stringList("weather.prevent-lightning-strike-blocks"))
	w.PreventLightningFire = w.bool("weather.disable-lightning-strike-fire", false)
	w.DisableThunder = w.bool("weather.disable-thunderstorm", false)
	w.DisableWeather = w.bool("weather.disable-weather", false)
	w.DisablePigZap = w.bool("weather.disable-pig-zombification", false)
	w.DisableVillagerZap = w.bool("weather.disable-villager-witchification", false)
	w.DisableCreeperPower = w.bool("weather.disable-powered-creepers", false)
	w.AlwaysRaining = w.bool("weather.always-raining", false)
	w.AlwaysThundering = w.bool("weather.always-thundering", false)
	w.DisableMushroomSpread = w.bool("dynamics.disable-mushroom-spread", false)
	w.DisableIceMelting = w.bool("dynamics.disable-ice-melting", false)
	w.DisableSnowMelting = w.bool("dynamics.disable-snow-melting", false)
	w.DisableSnowFormation = w.bool("dynamics.disable-snow-formation", false)
	w.DisableIceFormation = w.bool("dynamics.disable-ice-formation", false)
	w.DisableLeafDecay = w.bool("dynamics.disable-leaf-decay", false)
	w.DisableGrassGrowth = w.bool("dynamics.disable-grass-growth", false)
	w.DisableMyceliumSpread = w.bool("dynamics.disable-mycelium-spread", false)
	w.DisableVineGrowth = w.bool("dynamics.disable-vine-growth", false)
	w.DisableRockGrowth = w.bool("dynamics.disable-rock-growth", false)
	w.DisableSculkGrowth = w.bool("dynamics.disable-sculk-growth", false)
	w.DisableCropGrowth = w.bool("dynamics.disable-crop-growth", false)
	w.DisableSoilDehydration = w.bool("dynamics.disable-soil-dehydration", false)
	w.DisableSoilMoistureChange = w.bool("dynamics.disable-soil-moisture-change", false)
	w.DisableCoralBlockFade = w.bool("dynamics.disable-coral-block-fade", false)
	w.DisableCopperBlockFade = w.bool("dynamics.disable-copper-block-fade", false)
	w.DisableLavaHarden = w.bool("dynamics.disable-lava-harden", false)
	w.AllowedSnowFallOver = parseBlocks(m.Log, w.stringList("dynamics.snow-fall-blocks"))
	w.UseRegions = w.bool("regions.enable", true)
	w.RegionInvincibilityRemovesMob = w.bool("regions.invincibility-removes-mobs", false)
	w.RegionCancelEmptyChatEvents = w.bool("regions.cancel-chat-without-recipients", true)
	w.RegionNetherPortalProtection = w.bool("regions.nether-portal-protection", true)
	// Read from the world's own file only, as WorldGuard does.
	w.ForceDefaultTitleTimes = w.file.Bool("regions.titles-always-use-default-times", true)
	w.FakePlayerBuildOverride = w.bool("regions.fake-player-build-override", true)
	w.ExplosionFlagCancellation = w.bool("regions.explosion-flags-block-entity-damage", true)
	w.HighFreqFlags = w.bool("regions.high-frequency-flags", false)
	w.CheckLiquidFlow = w.bool("regions.protect-against-liquid-flow", false)
	wand := w.string("regions.wand", "minecraft:leather")
	if it, ok := ParseItem(wand); ok {
		w.RegionWand = it
	} else {
		w.RegionWand = registry.ItemLeather
	}
	w.MaxClaimVolume = w.int("regions.max-claim-volume", 30000)
	w.ClaimOnlyInsideExisting = w.bool("regions.claim-only-inside-existing-regions", false)
	w.SetParentOnClaim = w.string("regions.set-parent-on-claim", "")
	w.BoundedLocationFlags = w.bool("regions.location-flags-only-inside-regions", false)
	w.MaxRegionCountPerPlayer = w.int("regions.max-region-count-per-player.default", 7)
	w.maxRegionCounts = map[string]int{}
	for _, k := range w.keys("regions.max-region-count-per-player") {
		if strings.EqualFold(k, "default") {
			continue
		}
		if v, ok := w.property("regions.max-region-count-per-player." + k); ok {
			switch n := v.(type) {
			case int:
				w.maxRegionCounts[k] = n
			case float64:
				w.maxRegionCounts[k] = int(n)
			}
		}
	}
	w.BlockCreatureSpawn = map[registry.EntityType]bool{}
	for _, n := range w.stringList("mobs.block-creature-spawn") {
		if t, ok := EntityType(n); ok {
			w.BlockCreatureSpawn[t] = true
		} else {
			m.Log.Warn(fmt.Sprintf("Unknown entity type '%s'", n))
		}
	}
	w.UseBlacklistAsWhitelist = w.bool("blacklist.use-as-whitelist", false)
	w.LogConsole = w.bool("blacklist.logging.console.enable", true)
	w.LogDatabase = w.bool("blacklist.logging.database.enable", false)
	w.LogDatabaseDSN = w.string("blacklist.logging.database.dsn", "jdbc:mysql://localhost:3306/minecraft")
	w.LogDatabaseUser = w.string("blacklist.logging.database.user", "root")
	w.LogDatabasePass = w.string("blacklist.logging.database.pass", "")
	w.LogDatabaseTable = w.string("blacklist.logging.database.table", "blacklist_events")
	w.LogFile = w.bool("blacklist.logging.file.enable", false)
	w.LogFilePattern = w.string("blacklist.logging.file.path", "worldguard/logs/%Y-%m-%d.log")
	w.LogFileCacheSize = max(1, w.int("blacklist.logging.file.open-files", 10))
	if needParentSave {
		_ = w.parent.Save()
	}
}
