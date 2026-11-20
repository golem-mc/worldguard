package worldguardplugin

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

// registerInvincibility keeps invincible players from harm. WorldGuard:
// InvincibilityListener
func (m *sessionManager) registerInvincibility() {
	wg := m.wg
	opts := plugin.ListenOptions{IgnoreCancelled: true}
	plugin.Listen(wg.Server, wg.onDamageInvincible, opts)
	plugin.Listen(wg.Server, func(e *plugin.EntityCombustEvent) {
		if p, ok := e.Entity.(plugin.Player); ok && wg.invincible(p) {
			e.SetCancelled(true)
		}
	}, opts)
	plugin.Listen(wg.Server, func(e *plugin.FoodLevelChangeEvent) {
		if e.Level < e.Player.Food() && wg.invincible(e.Player) {
			e.SetCancelled(true)
		}
	}, opts)
}

// invincible reports whether god mode or the invincible flag protects p.
func (wg *WorldGuard) invincible(p plugin.Player) bool {
	return wg.sessions.get(p).invincible(wg.local(p))
}

// onDamageInvincible cancels damage to invincible players, removing the
// mob that hit them if the world says so, and drowning of amphibious
// players. WorldGuard: InvincibilityListener.onEntityDamage,
// WorldGuardEntityListener.onEntityDamage (amphibious mode)
func (wg *WorldGuard) onDamageInvincible(e *plugin.EntityDamageEvent) {
	p, ok := e.Entity.(plugin.Player)
	if !ok {
		return
	}
	if wg.invincible(p) {
		p.SetFireTicks(0)
		e.SetCancelled(true)
		attacker := e.Source.Causing
		if attacker == nil || !wg.WorldConfig(p.World()).RegionInvincibilityRemovesMob {
			return
		}
		if _, living := attacker.(plugin.LivingEntity); !living {
			return
		}
		if _, player := attacker.(plugin.Player); player {
			return
		}
		if t, ok := attacker.(plugin.Tameable); ok && t.Tamed() {
			return
		}
		attacker.Remove()
		return
	}
	if e.Source.Type == registry.DamageTypeDrown && wg.sessions.get(p).waterBreath.on {
		p.SetAir(300)
		e.SetCancelled(true)
	}
}

// Groups and the permission that give god mode or water breathing on join.
// WorldGuard: PlayerModesListener
const (
	invinciblePermission = "worldguard.auto-invincible"
	invincibleGroup      = "wg-invincible"
	amphibiousGroup      = "wg-amphibious"
)

// applyPlayerModes turns on god mode and water breathing for the players
// whose group or permission asks for them. WorldGuard:
// PlayerModesListener.onProcessPlayer
func (wg *WorldGuard) applyPlayerModes(p plugin.Player) {
	s := wg.sessions.get(p)
	lp := wg.local(p)
	cfg := wg.Config
	if (cfg.UseGodGroup && lp.HasGroup(invincibleGroup)) ||
		(cfg.UseGodPermission && wg.HasPermission(p, invinciblePermission)) {
		s.godMode.on = true
		wg.Log.Info("Enabled auto-god mode for " + p.Name())
	}
	if cfg.UseAmphibiousGroup && lp.HasGroup(amphibiousGroup) {
		s.waterBreath.on = true
		wg.Log.Info("Enabled water breathing mode for " + p.Name() + " (player is in group 'wg-amphibious')")
	}
}

// intensive reports an entity /halt-activity removes: items, TNT,
// experience orbs, falling blocks and untamed mobs. WorldGuard:
// Entities.isIntensiveEntity
func intensive(e plugin.Entity) bool {
	switch e.Type() {
	case registry.EntityTypeItem, registry.EntityTypeTnt, registry.EntityTypeExperienceOrb, registry.EntityTypeFallingBlock:
		return true
	case registry.EntityTypePlayer, registry.EntityTypeArmorStand:
		return false
	}
	if _, living := e.(plugin.LivingEntity); !living {
		return false
	}
	t, tameable := e.(plugin.Tameable)
	return !tameable || !t.Tamed()
}

// removeIntensiveEntities removes w's intensive entities and answers how
// many.
func removeIntensiveEntities(w plugin.World) int {
	var doomed []plugin.Entity
	for e := range w.Entities() {
		if intensive(e) {
			doomed = append(doomed, e)
		}
	}
	for _, e := range doomed {
		e.Remove()
	}
	return len(doomed)
}
