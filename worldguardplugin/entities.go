package worldguardplugin

import (
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard"
)

// isHostile reports whether e is an enemy (Bukkit's Enemy): a monster.
// WorldGuard: Entities.isHostile
func isHostile(e plugin.Entity) bool {
	if _, ok := e.(plugin.Player); ok {
		return false
	}
	_, living := e.(plugin.LivingEntity)
	return living && e.Type().Category() == registry.MobCategoryMonster
}

// isAmbient reports whether e is an ambient mob, a bat. WorldGuard:
// Entities.isAmbient
func isAmbient(e plugin.Entity) bool {
	_, living := e.(plugin.LivingEntity)
	return living && e.Type().Category() == registry.MobCategoryAmbient
}

// isNonHostile reports whether e is a peaceful creature: an animal, a
// villager, a golem, a fish. WorldGuard: Entities.isNonHostile
func isNonHostile(e plugin.Entity) bool {
	if _, ok := e.(plugin.Mob); !ok || isHostile(e) || isAmbient(e) {
		return false
	}
	switch e.Type() {
	case registry.EntityTypeGhast, registry.EntityTypeHappyGhast, registry.EntityTypeSlime, registry.EntityTypeMagmaCube,
		registry.EntityTypePhantom, registry.EntityTypeEnderDragon:
		return false
	}
	return true
}

// isNonPlayerCreature reports whether e is alive and no player.
func isNonPlayerCreature(e plugin.Entity) bool {
	if _, ok := e.(plugin.Player); ok {
		return false
	}
	_, ok := e.(plugin.LivingEntity)
	return ok
}

// isBoatType reports whether t is a boat or raft, with or without a chest.
func isBoatType(t registry.EntityType) bool {
	name := t.Name()
	return strings.HasSuffix(name, "_boat") || strings.HasSuffix(name, "_raft")
}

// isMinecartType reports whether t is a minecart of any kind.
func isMinecartType(t registry.EntityType) bool {
	switch t {
	case registry.EntityTypeMinecart, registry.EntityTypeChestMinecart, registry.EntityTypeCommandBlockMinecart,
		registry.EntityTypeFurnaceMinecart, registry.EntityTypeHopperMinecart, registry.EntityTypeSpawnerMinecart,
		registry.EntityTypeTntMinecart:
		return true
	}
	return false
}

// isVehicleType reports whether t is a boat or a minecart. WorldGuard:
// Entities.isVehicle
func isVehicleType(t registry.EntityType) bool { return isBoatType(t) || isMinecartType(t) }

// buildingIfUsed reports whether using e changes the world: a hanging
// entity, an armour stand, an end crystal, an allay, a minecart with an
// inventory. WorldGuard: Entities.isConsideredBuildingIfUsed
func buildingIfUsed(e plugin.Entity) bool {
	switch e.Type() {
	case registry.EntityTypePainting, registry.EntityTypeItemFrame, registry.EntityTypeGlowItemFrame,
		registry.EntityTypeLeashKnot, registry.EntityTypeArmorStand, registry.EntityTypeEndCrystal,
		registry.EntityTypeAllay, registry.EntityTypeChestMinecart, registry.EntityTypeHopperMinecart:
		return true
	}
	return false
}

// riddenOnUse reports whether using e rides it: a vehicle, or a mount.
// WorldGuard: Entities.isRiddenOnUse
func riddenOnUse(e plugin.Entity) bool {
	if isVehicleType(e.Type()) {
		return true
	}
	switch e.Type() {
	case registry.EntityTypePig, registry.EntityTypeStrider:
		// Steerable mounts are ridden only when saddled; the saddle is not
		// exposed, so the equipment's saddle slot is read.
		if l, ok := e.(plugin.LivingEntity); ok {
			return !l.Equipment().Get(plugin.SlotSaddle).Empty()
		}
	}
	return false
}

// isTNTBased reports whether e is primed TNT or a TNT minecart.
func isTNTBased(e plugin.Entity) bool {
	return e.Type() == registry.EntityTypeTnt || e.Type() == registry.EntityTypeTntMinecart
}

// isFireballType reports whether t is a ghast's or blaze's fireball.
func isFireballType(t registry.EntityType) bool {
	return t == registry.EntityTypeFireball || t == registry.EntityTypeSmallFireball
}

// explosionFlag answers the flag that governs what an entity of type t
// blows up. WorldGuard: Entities.getExplosionFlag
func explosionFlag(t registry.EntityType) *worldguard.StateFlag {
	switch t {
	case registry.EntityTypeBreezeWindCharge:
		return worldguard.FlagBreezeChargeExplosion
	case registry.EntityTypeWindCharge:
		return worldguard.FlagWindChargeBurst
	case registry.EntityTypeFireworkRocket:
		return worldguard.FlagFireworkDamage
	case registry.EntityTypeFireball, registry.EntityTypeSmallFireball, registry.EntityTypeDragonFireball,
		registry.EntityTypeWitherSkull:
		return worldguard.FlagGhastFireball
	case registry.EntityTypeWither:
		return worldguard.FlagWitherDamage
	case registry.EntityTypeCreeper:
		return worldguard.FlagCreeperExplosion
	case registry.EntityTypeTnt, registry.EntityTypeTntMinecart:
		return worldguard.FlagTNT
	case registry.EntityTypeEnderDragon:
		return worldguard.FlagEnderDragonBlockDamage
	}
	return worldguard.FlagOtherExplosion
}

// shooterOf follows projectiles back to who shot them. WorldGuard:
// Entities.getShooter
func shooterOf(e plugin.Entity) plugin.Entity {
	for e != nil {
		p, ok := e.(plugin.Projectile)
		if !ok {
			return e
		}
		s, ok := p.Shooter()
		if !ok || s == nil || s.UUID() == e.UUID() {
			return e
		}
		e = s
	}
	return e
}

// isTamed reports whether e is a tamed animal.
func isTamed(e plugin.Entity) bool {
	t, ok := e.(plugin.Tameable)
	return ok && t.Tamed()
}
