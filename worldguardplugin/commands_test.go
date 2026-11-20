package worldguardplugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

func TestGodHealAndSlay(t *testing.T) {
	srv, wg := start(t)
	p := join(srv, "Alice")
	p.Op()
	other := join(srv, "Bob")
	h := p.Handle()

	p.Run("/god")
	contains(t, p.Messages(2), "God mode enabled! Use /ungod to disable.")
	if !wg.invincible(h) {
		t.Fatal("god mode is off")
	}
	if h.Damage(5, plugin.DamageSource{Type: registry.DamageTypeGeneric}) {
		t.Fatal("a god took damage")
	}
	p.Run("/ungod")
	contains(t, p.Messages(2), "God mode disabled!")

	h.SetHealth(3)
	p.Run("/heal")
	contains(t, p.Messages(2), "Healed!")
	if h.Health() != h.MaxHealth() {
		t.Errorf("health %v after /heal", h.Health())
	}

	p.Run("/god -s Bob")
	contains(t, p.Messages(2), "Players now have god mode.")
	if msgs := other.Messages(1); len(msgs) != 0 {
		t.Errorf("-s still told Bob: %q", msgs)
	}
	p.Run("/heal Bob")
	contains(t, other.Messages(2), "Healed by Alice.")

	other.Run("/god")
	contains(t, other.Messages(2), "You don't have permission to do that.")
	other.Run("/stack")
	contains(t, other.Messages(2), "You don't have permission.")
	p.Run("/god a b")
	msgs := p.Messages(2)
	contains(t, msgs, "Too many arguments.")
	contains(t, msgs, "/god [-s] [player]")

	p.Run("/slay Bob")
	contains(t, other.Messages(2), "Slain by Alice.")
}

func TestStack(t *testing.T) {
	srv, _ := start(t)
	p := join(srv, "Alice")
	p.Op()
	inv := p.Handle().Inventory()
	inv.SetItem(0, plugin.NewStack(registry.ItemDirt, 40))
	inv.SetItem(5, plugin.NewStack(registry.ItemDirt, 40))
	inv.SetItem(6, plugin.NewStack(registry.ItemDiamondSword, 1))
	inv.SetItem(7, plugin.NewStack(registry.ItemDiamondSword, 1))
	p.Run("/stack")
	contains(t, p.Messages(2), "Items compacted into stacks!")
	if a, b := inv.Item(0).Count, inv.Item(5).Count; a != 64 || b != 16 {
		t.Errorf("dirt stacks %d and %d", a, b)
	}
	if inv.Item(7).Count != 1 {
		t.Error("swords were stacked")
	}
}

func TestToggles(t *testing.T) {
	srv, wg := start(t)
	world := wg.WorldName(wg.Server.Overworld())
	srv.Console("stopfire " + world)
	if !wg.WorldConfig(wg.Server.Overworld()).FireSpreadDisableToggle {
		t.Fatal("/stopfire did not stop fire")
	}
	contains(t, srv.Console("stopfire "+world), "Fire spread was already globally disabled.")
	contains(t, srv.Console("stopfire nowhere"), "No world by that exact name found.")
	srv.Console("allowfire " + world)
	if wg.WorldConfig(wg.Server.Overworld()).FireSpreadDisableToggle {
		t.Fatal("/allowfire did not allow fire")
	}

	contains(t, srv.Console("halt-activity"), "PERMANENTLY")
	if wg.Config.ActivityHaltToggle {
		t.Fatal("halted without confirming")
	}
	contains(t, srv.Console("stoplag confirm"), "ALL intensive server activity halted.")
	contains(t, srv.Console("halt-activity -i"), "ALL intensive server activity is not allowed.")
	contains(t, srv.Console("halt-activity -c"), "ALL intensive server activity is now allowed.")
}

func TestWorldGuardCommands(t *testing.T) {
	srv, wg := start(t)
	contains(t, srv.Console("wg version"), "WorldGuard ")
	contains(t, srv.Console("wg"), "/wg <version|reload|report|profile|stopprofile|flushstates|debug|running>")
	contains(t, srv.Console("worldguard reload"), "WorldGuard configuration reloaded.")
	contains(t, srv.Console("wg flushstates"), "Cleared all states.")
	contains(t, srv.Console("wg running"), "There are currently no running tasks.")
	contains(t, srv.Console("wg stopprofile"), "No CPU profile is currently running.")
	contains(t, srv.Console("wg profile 11"), "You can profile for, at maximum, 10 minutes.")
	out := srv.Console("wg report")
	contains(t, out, "WorldGuard report written to ")
	data, err := os.ReadFile(filepath.Join(wg.Server.DataDir(), "report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "WorldGuard Configuration") {
		t.Errorf("report:\n%s", data)
	}
}
