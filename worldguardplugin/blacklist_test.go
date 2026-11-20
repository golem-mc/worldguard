package worldguardplugin

import (
	"math"
	"os"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/golem/registry"
)

func TestBlacklistDeniesAndTells(t *testing.T) {
	srv, wg := start(t, plugintest.WithGameMode("creative"))
	bob := srv.Join("Bob")
	cfg := wg.WorldConfig(wg.Server.Overworld())
	if err := os.WriteFile(cfg.BlacklistFile, []byte("[stone]\non-break=deny,tell\n[dirt]\non-place=deny,tell\nmessage=&cNo dirt!\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wg.loadBlacklist(cfg)

	bob.StartBreak(2, 63, 2)
	if msgs := bob.Messages(2); !said(msgs, "You're not allowed to break") {
		t.Errorf("Bob was told %q", msgs)
	}
	if blockAt(wg, 2, 63, 2) != registry.BlockStone {
		t.Fatal("Bob broke blacklisted stone")
	}
	bob.Handle().Inventory().SetItem(0, plugin.NewStack(registry.ItemDirt, 8))
	bob.PlaceBlock(3, 63, 3, 1)
	if msgs := bob.Messages(2); !said(msgs, "No dirt!") {
		t.Errorf("Bob was told %q", msgs)
	}
	if blockAt(wg, 3, 64, 3) != registry.BlockAir {
		t.Fatal("Bob placed blacklisted dirt")
	}
	bob.Handle().Inventory().SetItem(0, plugin.NewStack(registry.ItemGravel, 8))
	bob.PlaceBlock(4, 63, 4, 1)
	bob.Read(2)
	if blockAt(wg, 4, 64, 4) != registry.BlockGravel {
		t.Fatal("Bob could not place gravel, which is not blacklisted")
	}
}

// A dispenser firing armour at a player is checked against on-equip.
// WorldGuard: BlacklistListener.onBlockDispenseArmor
func TestBlacklistStopsDispenserEquippingPlayer(t *testing.T) {
	srv, wg := start(t)
	bob := srv.Join("Bob")
	cfg := wg.WorldConfig(wg.Server.Overworld())
	if err := os.WriteFile(cfg.BlacklistFile, []byte("[iron_helmet]\non-equip=deny\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wg.loadBlacklist(cfg)

	loc := bob.Handle().Location()
	x, y, z := int(math.Floor(loc.Pos.X)), int(math.Floor(loc.Pos.Y)), int(math.Floor(loc.Pos.Z))
	w := wg.Server.Overworld()
	state, ok := registry.WithProp(registry.BlockDispenser.DefaultState(), registry.PropFacing, "south")
	if !ok {
		t.Fatal("dispenser has no facing")
	}
	dpos := plugin.BlockPos{X: x, Y: y, Z: z - 1}
	w.SetBlockState(dpos, plugin.BlockState(state), plugin.UpdateAll)
	srv.RunTicks(1)
	c, ok := w.BlockEntity(dpos)
	if !ok {
		t.Fatal("no dispenser block entity")
	}
	inv := c.(plugin.Container).Inventory()
	inv.SetItem(0, plugin.NewStack(registry.ItemIronHelmet, 1))
	w.SetBlockState(plugin.BlockPos{X: x, Y: y + 1, Z: z - 1}, plugin.BlockState(registry.BlockRedstoneBlock.DefaultState()), plugin.UpdateAll)
	srv.RunTicks(6)

	if inv.Item(0).Count != 1 {
		t.Error("the dispenser lost the denied helmet")
	}
	if !bob.Handle().Equipment().Get(plugin.SlotHead).Empty() {
		t.Error("the dispenser dressed Bob in a blacklisted helmet")
	}
}
