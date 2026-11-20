package worldguardplugin

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
)

// withRegions writes regions.yml for the overworld and loads it.
func withRegions(t *testing.T, wg *WorldGuard, yml string) {
	t.Helper()
	dir := filepath.Join(wg.Server.DataDir(), "worlds", wg.WorldName(wg.Server.Overworld()))
	if err := os.WriteFile(filepath.Join(dir, "regions.yml"), []byte("regions:\n"+yml), 0o644); err != nil {
		t.Fatal(err)
	}
	wg.regions.reload()
}

// cuboid is a regions.yml entry over x and z from lo to hi, y 0 to 200.
func cuboid(id string, lo, hi int, flags string) string {
	return "    " + id + ":\n        type: cuboid\n" +
		"        min: {x: " + itoa(lo) + ", y: 0, z: " + itoa(lo) + "}\n" +
		"        max: {x: " + itoa(hi) + ", y: 200, z: " + itoa(hi) + "}\n" +
		"        priority: 0\n        flags: {" + flags + "}\n"
}

func itoa(n int) string { return strconv.Itoa(n) }

// teleport moves p to (x, 64, z) as a plugin would, answering whether it
// happened.
func teleport(p *plugintest.Player, x float64) bool {
	h := p.Handle()
	return h.Teleport(plugin.Location{World: h.World(), Pos: plugin.Vec3{X: x, Y: 64, Z: x}}, plugin.TeleportPlugin)
}

func contains(t *testing.T, got []string, want string) {
	t.Helper()
	if !slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, want) }) {
		t.Fatalf("messages %q lack %q", got, want)
	}
}

func TestGreetingFarewellAndEntry(t *testing.T) {
	srv, wg := start(t)
	withRegions(t, wg, cuboid("lobby", 100, 110, `greeting: "Hi %name%", farewell: Bye, greeting-title: "Lobby\nsub"`)+
		cuboid("vault", 200, 210, "entry: deny"))
	p := join(srv, "Alice")
	if !teleport(p, 105) {
		t.Fatal("teleport into the lobby was refused")
	}
	contains(t, p.Messages(2), "Hi Alice")
	titles, subs := p.Titles(1)
	if !slices.Contains(titles, "Lobby") || !slices.Contains(subs, "sub") {
		t.Errorf("titles %q / %q", titles, subs)
	}
	if !teleport(p, 50) {
		t.Fatal("teleport out of the lobby was refused")
	}
	contains(t, p.Messages(2), "Bye")
	if teleport(p, 205) {
		t.Fatal("entry deny let the player in")
	}
	contains(t, p.Messages(2), "Hey! You are not permitted to enter this area.")
}

func TestExitDenyAndOverride(t *testing.T) {
	srv, wg := start(t)
	withRegions(t, wg, cuboid("jail", 100, 110, "exit: deny, exit-via-teleport: deny")+cuboid("yard", 300, 310, "exit-override: true"))
	p := join(srv, "Bob")
	if !teleport(p, 105) {
		t.Fatal("teleport into the jail was refused")
	}
	if teleport(p, 50) {
		t.Fatal("exit deny let the player out")
	}
	contains(t, p.Messages(2), "You are not permitted to leave this area.")
	if !teleport(p, 305) {
		t.Fatal("exit-override did not let the player out")
	}
}

func TestGameModeAndHealFlags(t *testing.T) {
	srv, wg := start(t)
	withRegions(t, wg, cuboid("arena", 100, 110, "game-mode: adventure, heal-amount: 20, heal-delay: 0, time-lock: '6000', weather-lock: rain"))
	p := join(srv, "Carol")
	h := p.Handle()
	h.SetHealth(5)
	if !teleport(p, 105) {
		t.Fatal("teleport refused")
	}
	srv.RunTicks(sessionTickPeriod + 2)
	if h.GameMode() != plugin.Adventure {
		t.Errorf("game mode %v in the arena", h.GameMode())
	}
	if h.Health() != h.MaxHealth() {
		t.Errorf("health %v after heal-delay 0", h.Health())
	}
	if ticks, fixed, set := h.PlayerTime(); ticks != 6000 || !fixed || !set {
		t.Errorf("player time %d fixed %v set %v", ticks, fixed, set)
	}
	if raining, set := h.PlayerWeather(); !raining || !set {
		t.Errorf("player weather raining %v set %v", raining, set)
	}
	teleport(p, 50)
	srv.RunTicks(1)
	if h.GameMode() != plugin.Survival {
		t.Errorf("game mode %v after leaving the arena", h.GameMode())
	}
	if _, _, set := h.PlayerTime(); set {
		t.Error("player time still locked after leaving")
	}
	if _, set := h.PlayerWeather(); set {
		t.Error("player weather still locked after leaving")
	}
}

// join joins a player and lets their join finish.
func join(srv *plugintest.Server, name string) *plugintest.Player {
	p := srv.Join(name)
	p.Messages(2)
	return p
}
