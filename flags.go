package worldguard

import (
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// FlagRegistry holds every flag by name. WorldGuard: SimpleFlagRegistry
type FlagRegistry struct {
	mu          sync.RWMutex
	flags       map[string]Flag
	order       []string
	initialized bool
}

// ErrFlagConflict is answered for a flag whose name is taken.
// WorldGuard: FlagConflictException
type ErrFlagConflict struct{ Name string }

func (e ErrFlagConflict) Error() string { return "A flag already exists by the name " + e.Name }

// Flags is the registry of every flag, WorldGuard's and other plugins'.
// A plugin registers its own flags from an init function or its Load, before
// WorldGuard loads the regions.
var Flags = &FlagRegistry{flags: map[string]Flag{}}

// Register adds f. It fails for a taken name, and once WorldGuard has loaded
// its regions ("New flags cannot be registered at this time").
func (r *FlagRegistry) Register(f Flag) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.initialized {
		return fmt.Errorf("new flags cannot be registered at this time")
	}
	return r.add(f)
}

func (r *FlagRegistry) add(f Flag) error {
	name := strings.ToLower(f.Name())
	if _, ok := r.flags[name]; ok {
		return ErrFlagConflict{name}
	}
	r.flags[name] = f
	r.order = append(r.order, name)
	return nil
}

// SetInitialized closes or reopens registration; WorldGuard closes it once
// it has loaded its regions.
func (r *FlagRegistry) SetInitialized(v bool) {
	r.mu.Lock()
	r.initialized = v
	r.mu.Unlock()
}

// Get answers the flag named name, ignoring case.
func (r *FlagRegistry) Get(name string) (Flag, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.flags[strings.ToLower(name)]
	return f, ok
}

// All answers every flag in the order they were registered.
func (r *FlagRegistry) All() []Flag {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Flag, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.flags[n])
	}
	return out
}

// Len answers the number of flags.
func (r *FlagRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.flags)
}

// FuzzyMatch answers the flag named id, ignoring case and dashes.
// WorldGuard: Flags.fuzzyMatchFlag
func (r *FlagRegistry) FuzzyMatch(id string) (Flag, bool) {
	want := strings.ReplaceAll(id, "-", "")
	for _, f := range r.All() {
		if strings.EqualFold(strings.ReplaceAll(f.Name(), "-", ""), want) {
			return f, true
		}
	}
	return nil, false
}

func (r *FlagRegistry) getOrCreate(name string) Flag {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.flags[strings.ToLower(name)]; ok {
		return f
	}
	f := newUnknownFlag(name)
	_ = r.add(f)
	return f
}

// Unmarshal reads a region's "flags" section; with createUnknown, a flag no
// plugin registered is kept as an UnknownFlag. A value that does not parse is
// dropped with a warning on log. WorldGuard: SimpleFlagRegistry.unmarshal
func (r *FlagRegistry) Unmarshal(raw map[string]any, createUnknown bool, log *slog.Logger) map[Flag]any {
	values := map[Flag]any{}
	groups := map[string]any{}
	for _, name := range slices.Sorted(maps.Keys(raw)) {
		v := raw[name]
		if strings.HasSuffix(name, "-group") {
			groups[name] = v
			continue
		}
		var f Flag
		if createUnknown {
			f = r.getOrCreate(name)
		} else if g, ok := r.Get(name); ok {
			f = g
		}
		if f == nil {
			continue
		}
		if u := f.Unmarshal(v); u != nil {
			values[f] = u
		} else if log != nil {
			log.Warn(fmt.Sprintf("Failed to parse flag '%s' with value '%v'", f.Name(), v))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(groups)) {
		v := groups[name]
		parent, ok := r.Get(strings.ReplaceAll(name, "-group", ""))
		if _, unknown := parent.(*UnknownFlag); !ok || unknown || parent.GroupFlag() == nil {
			if _, exists := r.Get(name); createUnknown && !exists {
				r.getOrCreate(name)
			}
			if unk, ok := r.Get(name); ok {
				values[unk] = v
			}
			continue
		}
		if g := parent.GroupFlag().Unmarshal(v); g != nil {
			values[parent.GroupFlag()] = g
		}
	}
	return values
}

// MarshalFlags writes a region's flags for regions.yml; a group flag is
// written under "<flag>-group". WorldGuard: FlagUtil.marshal
func MarshalFlags(values map[Flag]any) map[string]any {
	out := make(map[string]any, len(values))
	for f, v := range values {
		if m := f.Marshal(v); m != nil {
			out[f.Name()] = m
		}
	}
	return out
}

// builtin lists WorldGuard's own flag names in registration order.
var builtin []string

// IsBuiltinFlag reports whether name is one of WorldGuard's own flags.
func IsBuiltinFlag(name string) bool { return slices.Contains(builtin, strings.ToLower(name)) }

func reg[F Flag](f F) F {
	if err := Flags.add(f); err != nil {
		panic(err)
	}
	builtin = append(builtin, f.Name())
	return f
}

func state(name string, def bool, opts ...FlagOption) *StateFlag {
	return reg(NewStateFlag(name, def, opts...))
}

// The default messages, as WorldGuard serialises its components into
// legacy text.
const (
	defaultTeleportMessage  = "§dTeleported you to the region '%id%'."
	defaultDenyMessage      = "§c§lHey!§7 Sorry, but you can't %what% here."
	defaultEntryDenyMessage = "§c§lHey!§7 You are not permitted to enter this area."
	defaultExitDenyMessage  = "§c§lHey!§7 You are not permitted to leave this area."
)

// WorldGuard's flags, by their upstream names. WorldGuard: Flags
var (
	FlagPassthrough                = state("passthrough", false)
	FlagNonPlayerProtectionDomains = reg(NewSetFlag("nonplayer-protection-domains", NewStringFlag("", ""), WithoutGroup()))
	FlagBuild                      = reg(&StateFlag{baseFlag: base("build", nil), def: true, build: true})
	FlagBlockBreak                 = state("block-break", false)
	FlagBlockPlace                 = state("block-place", false)
	FlagUse                        = state("use", false)
	FlagInteract                   = state("interact", false)
	FlagDamageAnimals              = state("damage-animals", false)
	FlagPVP                        = state("pvp", false)
	FlagSleep                      = state("sleep", false)
	FlagRespawnAnchors             = state("respawn-anchors", false)
	FlagTNT                        = state("tnt", false)
	FlagChestAccess                = state("chest-access", false)
	FlagVehiclePlace               = state("vehicle-place", false)
	FlagVehicleDestroy             = state("vehicle-destroy", false)
	FlagLighter                    = state("lighter", false)
	FlagRide                       = state("ride", false)
	FlagPotionSplash               = state("potion-splash", false)
	FlagItemFrameRotation          = state("item-frame-rotation", false)
	FlagBlockTrampling             = state("block-trampling", false)
	FlagFireworkDamage             = state("firework-damage", false)
	FlagUseAnvil                   = state("use-anvil", false)
	FlagUseDripleaf                = state("use-dripleaf", false)
	FlagWindChargeBurst            = state("wind-charge-burst", false)

	FlagItemPickup             = state("item-pickup", true)
	FlagItemDrop               = state("item-drop", true)
	FlagExpDrops               = state("exp-drops", true)
	FlagMobDamage              = state("mob-damage", true)
	FlagCreeperExplosion       = state("creeper-explosion", true)
	FlagEnderDragonBlockDamage = state("enderdragon-block-damage", true)
	FlagGhastFireball          = state("ghast-fireball", true)
	FlagOtherExplosion         = state("other-explosion", true)
	FlagBreezeChargeExplosion  = state("breeze-charge-explosion", true)
	FlagWitherDamage           = state("wither-damage", true)
	FlagEndermanGrief          = state("enderman-grief", true)
	FlagSnowmanTrails          = state("snowman-trails", true)
	FlagRavagerGrief           = state("ravager-grief", true)
	FlagEntityPaintingDestroy  = state("entity-painting-destroy", true)
	FlagEntityItemFrameDestroy = state("entity-item-frame-destroy", true)
	FlagMobSpawning            = state("mob-spawning", true)
	FlagDenySpawn              = reg(NewSetFlag("deny-spawn", NewRegistryFlag("", EntityTypes)))
	FlagPistons                = state("pistons", true)
	FlagFireSpread             = state("fire-spread", true)
	FlagLavaFire               = state("lava-fire", true)
	FlagLightning              = state("lightning", true)
	FlagSnowFall               = state("snow-fall", true)
	FlagSnowMelt               = state("snow-melt", true)
	FlagIceForm                = state("ice-form", true)
	FlagIceMelt                = state("ice-melt", true)
	FlagFrostedIceMelt         = state("frosted-ice-melt", true)
	FlagFrostedIceForm         = state("frosted-ice-form", false)
	FlagMushroomGrowth         = state("mushroom-growth", true)
	FlagLeafDecay              = state("leaf-decay", true)
	FlagGrassGrowth            = state("grass-growth", true)
	FlagMyceliumSpread         = state("mycelium-spread", true)
	FlagVineGrowth             = state("vine-growth", true)
	FlagRockGrowth             = state("rock-growth", true)
	FlagSculkGrowth            = state("sculk-growth", true)
	FlagCropGrowth             = state("crop-growth", true)
	FlagSoilDry                = state("soil-dry", true)
	FlagCoralFade              = state("coral-fade", true)
	FlagCopperFade             = state("copper-fade", true)
	FlagWaterFlow              = state("water-flow", true)
	FlagLavaFlow               = state("lava-flow", true)
	FlagMoistureChange         = state("moisture-change", true)
	FlagLavaHarden             = state("lava-harden", true)
	FlagWeatherLock            = reg(NewRegistryFlag("weather-lock", WeatherTypes))
	FlagTimeLock               = reg(NewStringFlag("time-lock", ""))
	FlagSendChat               = state("send-chat", true)
	FlagReceiveChat            = state("receive-chat", true)
	FlagBlockedCmds            = reg(NewSetFlag("blocked-cmds", NewCommandStringFlag("")))
	FlagAllowedCmds            = reg(NewSetFlag("allowed-cmds", NewCommandStringFlag("")))
	FlagTeleport               = reg(NewLocationFlag("teleport"))
	FlagSpawn                  = reg(NewLocationFlag("spawn", WithGroup(RegionGroupMembers)))
	FlagTeleportMessage        = reg(NewStringFlag("teleport-message", defaultTeleportMessage))
	FlagInvincible             = state("invincible", false)
	FlagFallDamage             = state("fall-damage", true)
	FlagNaturalHealthRegen     = state("natural-health-regen", true)
	FlagNaturalHungerDrain     = state("natural-hunger-drain", true)
	FlagEntry                  = state("entry", true, WithGroup(RegionGroupNonMembers))
	FlagExit                   = state("exit", true, WithGroup(RegionGroupNonMembers))
	FlagExitOverride           = reg(NewBooleanFlag("exit-override"))
	FlagExitViaTeleport        = state("exit-via-teleport", true)
	FlagEnderpearl             = state("enderpearl", true)
	FlagChorusFruitTeleport    = state("chorus-fruit-teleport", true)
	FlagGreeting               = reg(NewStringFlag("greeting", ""))
	FlagFarewell               = reg(NewStringFlag("farewell", ""))
	FlagGreetingTitle          = reg(NewStringFlag("greeting-title", ""))
	FlagFarewellTitle          = reg(NewStringFlag("farewell-title", ""))
	FlagNotifyEnter            = reg(NewBooleanFlag("notify-enter"))
	FlagNotifyLeave            = reg(NewBooleanFlag("notify-leave"))
	FlagGameMode               = reg(NewRegistryFlag("game-mode", GameModes))
	FlagHealDelay              = reg(NewIntegerFlag("heal-delay").SetSuggestions("0", "1", "5"))
	FlagHealAmount             = reg(NewIntegerFlag("heal-amount").SetSuggestions("0", "5", "10", "20"))
	FlagHealMinHealth          = reg(NewDoubleFlag("heal-min-health").SetSuggestions("0", "10"))
	FlagHealMaxHealth          = reg(NewDoubleFlag("heal-max-health").SetSuggestions("10", "20"))
	FlagFeedDelay              = reg(NewIntegerFlag("feed-delay").SetSuggestions("0", "1", "5"))
	FlagFeedAmount             = reg(NewIntegerFlag("feed-amount").SetSuggestions("0", "5", "10", "20"))
	FlagFeedMinHunger          = reg(NewIntegerFlag("feed-min-hunger").SetSuggestions("0", "10"))
	FlagFeedMaxHunger          = reg(NewIntegerFlag("feed-max-hunger").SetSuggestions("10", "20"))
	FlagDenyMessage            = reg(NewStringFlag("deny-message", defaultDenyMessage))
	FlagEntryDenyMessage       = reg(NewStringFlag("entry-deny-message", defaultEntryDenyMessage))
	FlagExitDenyMessage        = reg(NewStringFlag("exit-deny-message", defaultExitDenyMessage))
)

// mapRegistry is a Registry over a fixed table.
type mapRegistry struct {
	name   string
	keys   []string
	values map[string]any
}

func (r *mapRegistry) Name() string { return r.name }
func (r *mapRegistry) Lookup(key string) (any, bool) {
	v, ok := r.values[key]
	return v, ok
}
func (r *mapRegistry) Keys() []string { return r.keys }
func (r *mapRegistry) ID(v any) string {
	for _, k := range r.keys {
		if r.values[k] == v {
			return k
		}
	}
	return fmt.Sprint(v)
}

// GameModes is the registry of game modes, by WorldEdit's ids.
var GameModes Registry = &mapRegistry{name: "game mode", keys: []string{"survival", "creative", "adventure", "spectator"},
	values: map[string]any{"survival": plugin.Survival, "creative": plugin.Creative, "adventure": plugin.Adventure, "spectator": plugin.Spectator}}

// WeatherTypes is the registry of weather types, by WorldEdit's ids.
var WeatherTypes Registry = &mapRegistry{name: "weather type", keys: []string{"clear", "rain", "thunder_storm"},
	values: map[string]any{"clear": plugin.WeatherClear, "rain": plugin.WeatherRain, "thunder_storm": plugin.WeatherThunder}}

// EntityTypes is the registry of entity types, by namespaced name; a name
// without a namespace is in minecraft's.
var EntityTypes Registry = entityTypeRegistry{}

type entityTypeRegistry struct{}

func (entityTypeRegistry) Name() string { return "entity type" }
func (entityTypeRegistry) Lookup(key string) (any, bool) {
	if !strings.Contains(key, ":") {
		key = "minecraft:" + key
	}
	t, ok := registry.EntityTypeByName(key)
	if !ok {
		return nil, false
	}
	return t, true
}
func (entityTypeRegistry) ID(v any) string {
	if t, ok := v.(registry.EntityType); ok {
		return t.Name()
	}
	return fmt.Sprint(v)
}
func (entityTypeRegistry) Keys() []string {
	var out []string
	for t := range registry.EntityType(registry.EntityTypeCount) {
		out = append(out, t.Name())
	}
	return out
}
