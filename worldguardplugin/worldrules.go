package worldguardplugin

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/event"
)

// registerWorldRules applies the weather settings, /halt-activity to
// loading chunks, and the gameplay rules. WorldGuard:
// WorldGuardWeatherListener, WorldGuardWorldListener, WorldRulesListener,
// BlockedPotionsListener
func (p *protection) registerWorldRules() {
	s := p.wg.Server
	high := plugin.ListenOptions{Priority: plugin.High, IgnoreCancelled: true}
	plugin.Listen(s, func(e *plugin.WeatherChangeEvent) {
		cfg := p.cfg(e.World)
		if e.Raining && cfg.DisableWeather || !e.Raining && !cfg.DisableWeather && cfg.AlwaysRaining {
			e.SetCancelled(true)
		}
	}, high)
	plugin.Listen(s, func(e *plugin.ThunderChangeEvent) {
		cfg := p.cfg(e.World)
		if e.Thundering && cfg.DisableThunder || !e.Thundering && !cfg.DisableWeather && cfg.AlwaysThundering {
			e.SetCancelled(true)
		}
	}, high)
	plugin.Listen(s, p.ruleLightning, high)
	plugin.Listen(s, func(e *plugin.ChunkLoadEvent) { p.haltChunk(e.Chunk) })
	plugin.Listen(s, func(e *plugin.WorldLoadEvent) { p.initWeather(e.World) })
	for _, w := range s.Worlds() {
		p.initWeather(w)
	}
	early := plugin.ListenOptions{Priority: plugin.Lowest, IgnoreCancelled: true}
	plugin.Listen(s, func(e *event.SpawnEntityEvent) {
		if e.Type == registry.EntityTypeExperienceOrb && p.cfg(e.World).DisableExpDrops {
			e.SetCancelled(true)
		}
	}, early)
	plugin.Listen(s, func(e *plugin.EntityPotionEffectEvent) {
		if e.Cause == plugin.EffectConduit && p.cfg(e.Entity.World()).DisableConduitEffects {
			e.SetCancelled(true)
		}
	}, early)
	plugin.Listen(s, p.blockedPotion)
	plugin.Listen(s, p.blockedArrow)
}

func (p *protection) ruleLightning(e *plugin.LightningStrikeEvent) {
	cfg := p.cfg(e.World)
	loc := e.Lightning.Location()
	if len(cfg.DisallowedLightningBlocks) > 0 {
		b := plugin.Block{World: e.World, Pos: loc.Pos.Block()}
		t := b.State().Block()
		if isAir(t) {
			t = b.Relative(plugin.Down, 1).State().Block()
		}
		if cfg.DisallowedLightningBlocks[t] {
			e.SetCancelled(true)
		}
	}
	if cfg.UseRegions && !worldguard.TestStates(p.query().QueryState(loc, nil, worldguard.FlagLightning)) {
		e.SetCancelled(true)
	}
}

// haltChunk removes the intensive entities of a loading chunk while
// activity is halted. WorldGuard: WorldGuardWorldListener.onChunkLoad
func (p *protection) haltChunk(c plugin.Chunk) {
	if !p.wg.Config.ActivityHaltToggle {
		return
	}
	removed := 0
	for _, e := range c.Entities() {
		if intensive(e) {
			e.Remove()
			removed++
		}
	}
	if removed > 50 {
		p.wg.Log.Info(fmt.Sprintf("Halt-Act: %d entities (>50) auto-removed from chunk %d, %d", removed, c.Pos().X, c.Pos().Z))
	}
}

// initWeather starts or stops a world's rain and thunder as its settings
// say. WorldGuard: WorldGuardWorldListener.initWorld
func (p *protection) initWeather(w plugin.World) {
	cfg := p.cfg(w)
	switch {
	case cfg.AlwaysRaining && !cfg.DisableWeather:
		w.SetStorm(true, 0)
	case cfg.DisableWeather && !cfg.AlwaysRaining:
		w.SetStorm(false, 0)
	}
	switch {
	case cfg.AlwaysThundering && !cfg.DisableThunder:
		w.SetThundering(true, 0)
	case cfg.DisableThunder && !cfg.AlwaysThundering:
		w.SetStorm(false, 0)
	}
}

// legacyEffectNames maps Bukkit's old potion effect names to vanilla's.
var legacyEffectNames = map[string]string{
	"slow": "slowness", "fast_digging": "haste", "slow_digging": "mining_fatigue", "increase_damage": "strength",
	"heal": "instant_health", "harm": "instant_damage", "jump": "jump_boost", "confusion": "nausea",
	"damage_resistance": "resistance",
}

// effectName answers the vanilla name of a block-potions entry.
func effectName(s string) string {
	s = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "minecraft:")
	if v, ok := legacyEffectNames[s]; ok {
		s = v
	}
	return "minecraft:" + s
}

var (
	potionID       = regexp.MustCompile(`potion\s*:\s*"?([a-z0-9_:.]+)"?`)
	customEffectID = regexp.MustCompile(`\bid\s*:\s*"?([a-z0-9_:.]+)"?`)
)

// potionEffects answers the effect names a potion item carries.
func potionEffects(item plugin.ItemStack) []string {
	snbt := item.Patch.Other[plugin.MinecraftKey("potion_contents")]
	if snbt == "" {
		return nil
	}
	var out []string
	if m := potionID.FindStringSubmatch(snbt); m != nil {
		name := m[1]
		if !strings.Contains(name, ":") {
			name = "minecraft:" + name
		}
		if id, ok := registry.PotionID(name); ok {
			for _, e := range registry.PotionEffects(id) {
				out = append(out, effectName(e.Effect))
			}
		}
	}
	for _, m := range customEffectID.FindAllStringSubmatch(snbt, -1) {
		out = append(out, effectName(m[1]))
	}
	return out
}

// blockedPotion stops potions with an effect gameplay.block-potions names.
// WorldGuard: BlockedPotionsListener.onItemInteract
func (p *protection) blockedPotion(e *event.UseItemEvent) {
	switch e.Item.Item {
	case registry.ItemPotion, registry.ItemSplashPotion, registry.ItemLingeringPotion:
	default:
		return
	}
	cfg := p.cfg(e.World)
	effect := blockedEffect(cfg.BlockPotions, potionEffects(e.Item))
	if effect == "" {
		return
	}
	pl := e.Cause.FirstPlayer()
	if pl == nil {
		e.SetCancelled(true)
		return
	}
	thrown := e.Item.Item == registry.ItemSplashPotion || e.Item.Item == registry.ItemLingeringPotion
	if !p.wg.HasPermission(pl, "worldguard.override.potions") {
		tell(pl, "§cSorry, potions with "+effect+" are presently disabled.")
		e.SetCancelled(true)
	} else if cfg.BlockPotionsAlways && thrown {
		tell(pl, "§cSorry, potions with "+effect+" can't be thrown, even if you have a permission to bypass it, "+
			"due to limitations (and because overly-reliable potion blocking is on).")
		e.SetCancelled(true)
	}
}

// blockedEffect answers the short name of the first of names that the
// gameplay.block-potions list blocks, or "" when none is.
func blockedEffect(blockList, names []string) string {
	if len(blockList) == 0 {
		return ""
	}
	blocked := map[string]bool{}
	for _, n := range blockList {
		blocked[effectName(n)] = true
	}
	for _, n := range names {
		if blocked[n] {
			return shortName(n)
		}
	}
	return ""
}

// arrowEffects answers the names of the effects a tipped arrow applies.
func arrowEffects(key plugin.Key, effects []plugin.EffectInstance) []string {
	var out []string
	if key != (plugin.Key{}) {
		if id, ok := registry.PotionID(key.Namespace + ":" + key.Path); ok {
			for _, e := range registry.PotionEffects(id) {
				out = append(out, effectName(e.Effect))
			}
		}
	}
	for _, e := range effects {
		out = append(out, effectName(registry.MobEffectName(int32(e.Effect))))
	}
	return out
}

// blockedArrow refuses tipped arrows that carry an effect gameplay.block-potions
// names.
// WorldGuard: BlockedPotionsListener.onProjectile
func (p *protection) blockedArrow(e *plugin.ProjectileLaunchEvent) {
	arrow, ok := e.Projectile.(plugin.Arrow)
	if !ok {
		return
	}
	cfg := p.cfg(arrow.World())
	if len(cfg.BlockPotions) == 0 {
		return
	}
	key, effects, tipped := arrow.Potion()
	if !tipped {
		return
	}
	effect := blockedEffect(cfg.BlockPotions, arrowEffects(key, effects))
	if effect == "" {
		return
	}
	shooter, _ := arrow.Shooter()
	pl, _ := shooter.(plugin.Player)
	if pl != nil && p.wg.HasPermission(pl, "worldguard.override.potions") {
		return
	}
	if pl != nil {
		tell(pl, "§cSorry, arrows with "+effect+" are presently disabled.")
	}
	e.SetCancelled(true)
}
