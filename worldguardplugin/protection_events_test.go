package worldguardplugin

import (
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard"
)

func TestPotionLeashLecternMoistureAndKnockback(t *testing.T) {
	srv, wg := start(t, plugintest.WithGameMode("creative"))
	alice := srv.Join("Alice")
	w := wg.Server.Overworld()
	r := addRegion(t, wg, "home", plugin.BlockPos{X: 0, Y: 0, Z: 0}, plugin.BlockPos{X: 10, Y: 100, Z: 10}, alice.Handle())
	cow, err := w.Spawn(registry.EntityTypeCow, plugin.Location{World: w, Pos: plugin.Vec3{X: 5.5, Y: 64, Z: 5.5}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	pot, err := w.Spawn(registry.EntityTypeSplashPotion, plugin.Location{World: w, Pos: plugin.Vec3{X: 5.5, Y: 65, Z: 5.5}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fire := wg.Server.Events().Fire
	home := regionBlock(wg, 5, 64, 5)

	// potion-splash
	r.SetFlag(worldguard.FlagPotionSplash, worldguard.StateDeny)
	splash := &plugin.PotionSplashEvent{Potion: pot.(plugin.Projectile), Affected: map[plugin.LivingEntity]float64{cow.(plugin.LivingEntity): 1}}
	fire(splash)
	if !splash.Cancelled() || splash.Affected[cow.(plugin.LivingEntity)] != 0 {
		t.Errorf("potion-splash deny: cancelled=%v intensity=%v", splash.Cancelled(), splash.Affected[cow.(plugin.LivingEntity)])
	}
	cloud := &plugin.AreaEffectCloudApplyEvent{Cloud: pot, Affected: []plugin.LivingEntity{cow.(plugin.LivingEntity)}}
	fire(cloud)
	if len(cloud.Affected) != 0 && !cloud.Cancelled() {
		t.Errorf("cloud still affects the cow")
	}
	r.SetFlag(worldguard.FlagPotionSplash, nil)

	// ride/interact on leashing: a non-member may not, the owner may
	bob := srv.Join("Bob")
	leash := &plugin.PlayerLeashEntityEvent{Player: bob.Handle(), Entity: cow, Holder: bob.Handle()}
	fire(leash)
	if !leash.Cancelled() {
		t.Error("a non-member leashed a mob in the region")
	}
	leash = &plugin.PlayerLeashEntityEvent{Player: alice.Handle(), Entity: cow, Holder: alice.Handle(), Unleash: true}
	fire(leash)
	if leash.Cancelled() {
		t.Error("the owner could not unleash a mob in the region")
	}

	// lectern books count as chest access
	srv.Console("setblock 5 64 5 minecraft:lectern")
	take := &plugin.PlayerTakeLecternBookEvent{Player: bob.Handle(), Block: home}
	fire(take)
	if !take.Cancelled() {
		t.Error("a non-member took a lectern's book in the region")
	}
	take = &plugin.PlayerTakeLecternBookEvent{Player: alice.Handle(), Block: home}
	fire(take)
	if take.Cancelled() {
		t.Error("the owner could not take a lectern's book")
	}

	// moisture-change
	m := &plugin.MoistureChangeEvent{Block: home}
	fire(m)
	if m.Cancelled() {
		t.Error("moisture cancelled with no flag or config")
	}
	r.SetFlag(worldguard.FlagMoistureChange, worldguard.StateDeny)
	m = &plugin.MoistureChangeEvent{Block: home}
	fire(m)
	if !m.Cancelled() {
		t.Error("moisture-change deny did not cancel")
	}
	r.SetFlag(worldguard.FlagMoistureChange, nil)
	wg.WorldConfig(w).DisableSoilMoistureChange = true
	m = &plugin.MoistureChangeEvent{Block: home}
	fire(m)
	if !m.Cancelled() {
		t.Error("disable-soil-moisture-change did not cancel")
	}

	// wind charge knockback
	wc, err := w.Spawn(registry.EntityTypeWindCharge, plugin.Location{World: w, Pos: plugin.Vec3{X: 5.5, Y: 65, Z: 5.5}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := wg.WorldConfig(w)
	cfg.ExplosionFlagCancellation = true
	r.SetFlag(worldguard.FlagWindChargeBurst, worldguard.StateDeny)
	k := &plugin.EntityKnockbackEvent{Entity: cow, Pusher: wc, Knockback: plugin.Vec3{Y: 1}}
	fire(k)
	if !k.Cancelled() {
		t.Error("wind-charge-burst deny did not stop the knockback")
	}
	// a player is judged the same way: no bypass, the owner included
	for _, pl := range []plugin.Player{alice.Handle(), bob.Handle()} {
		pl.Teleport(plugin.Location{World: w, Pos: plugin.Vec3{X: 3.5, Y: 64, Z: 3.5}}, plugin.TeleportPlugin)
		pk := &plugin.EntityKnockbackEvent{Entity: pl, Pusher: wc, Knockback: plugin.Vec3{Y: 1}}
		fire(pk)
		if !pk.Cancelled() {
			t.Errorf("wind-charge-burst deny did not stop the knockback of %s", pl.Name())
		}
	}
	r.SetFlag(worldguard.FlagWindChargeBurst, nil)
	far, err := w.Spawn(registry.EntityTypeCow, plugin.Location{World: w, Pos: plugin.Vec3{X: 50.5, Y: 64, Z: 50.5}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	breeze, err := w.Spawn(registry.EntityTypeBreezeWindCharge, plugin.Location{World: w, Pos: plugin.Vec3{X: 50.5, Y: 65, Z: 50.5}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	k = &plugin.EntityKnockbackEvent{Entity: far, Pusher: breeze, Knockback: plugin.Vec3{Y: 1}}
	fire(k)
	if k.Cancelled() {
		t.Error("breeze knockback cancelled with the default flag")
	}
	cfg.BlockWindChargeExplosions = true
	k = &plugin.EntityKnockbackEvent{Entity: cow, Pusher: wc}
	fire(k)
	if !k.Cancelled() {
		t.Error("block-windcharge-explosions did not stop the knockback")
	}

	// a blow's knockback is judged like the damage: pvp deny stops it,
	// nothing denied lets it through
	pl, victim := alice.Handle(), bob.Handle()
	pl.Teleport(plugin.Location{World: w, Pos: plugin.Vec3{X: 3.5, Y: 64, Z: 3.5}}, plugin.TeleportPlugin)
	victim.Teleport(plugin.Location{World: w, Pos: plugin.Vec3{X: 4.5, Y: 64, Z: 3.5}}, plugin.TeleportPlugin)
	kb := &plugin.EntityKnockbackEvent{Entity: victim, Pusher: pl, Knockback: plugin.Vec3{X: 1}}
	fire(kb)
	if kb.Cancelled() {
		t.Error("a member's blow was stopped with nothing denied")
	}
	r.SetFlag(worldguard.FlagPVP, worldguard.StateDeny)
	kb = &plugin.EntityKnockbackEvent{Entity: victim, Pusher: pl, Knockback: plugin.Vec3{X: 1}}
	fire(kb)
	if !kb.Cancelled() {
		t.Error("pvp deny did not stop a player's knockback of another player")
	}
	r.SetFlag(worldguard.FlagPVP, nil)
}
