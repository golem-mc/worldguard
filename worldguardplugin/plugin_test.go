package worldguardplugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/worldguard"
)

// start runs a flat test server with WorldGuard in a temporary folder.
func start(t *testing.T, opts ...plugintest.Option) (*plugintest.Server, *WorldGuard) {
	t.Helper()
	t.Chdir(t.TempDir())
	srv := plugintest.Start(t, opts...)
	wg, ok := Get()
	if !ok {
		t.Fatal("worldguard did not enable")
	}
	return srv, wg
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEnableWritesConfiguration(t *testing.T) {
	_, wg := start(t)
	cfg := read(t, filepath.Join(wg.Server.DataDir(), "config.yml"))
	for _, want := range []string{"# WorldGuard's main configuration file", "    uuid-migration:",
		"use-player-move-event: true", "summary-on-start: true", "    max-claim-volume: 30000",
		"    wand: minecraft:leather", "host-keys: {}"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config.yml lacks %q:\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "    sql:") || strings.Contains(cfg, "simulation:") {
		t.Errorf("config.yml keeps a retired section:\n%s", cfg)
	}
	world := wg.WorldName(wg.Server.Overworld())
	worldDir := filepath.Join(wg.Server.DataDir(), "worlds", world)
	if wc := read(t, filepath.Join(worldDir, "config.yml")); !strings.Contains(wc, "titles-always-use-default-times: true") {
		t.Errorf("world config:\n%s", wc)
	}
	if bl := read(t, filepath.Join(worldDir, "blacklist.txt")); !strings.Contains(bl, "# WorldGuard blacklist") {
		t.Errorf("blacklist.txt:\n%s", bl)
	}
	if _, ok := worldguard.Lookup(wg.Server); !ok {
		t.Error("no worldguard.API service")
	}
	if wg.WorldName(wg.Server.Overworld()) != wg.LevelName() {
		t.Errorf("overworld is named %q, level %q", world, wg.LevelName())
	}
}

func TestRegionsLoadQueryAndSave(t *testing.T) {
	srv, wg := start(t)
	w := wg.Server.Overworld()
	dir := filepath.Join(wg.Server.DataDir(), "worlds", wg.WorldName(w))
	if err := os.WriteFile(filepath.Join(dir, "regions.yml"), []byte(`regions:
    arena:
        min: {x: 0.0, y: 0.0, z: 0.0}
        max: {x: 9.0, y: 100.0, z: 9.0}
        flags: {pvp: deny}
        owners: {}
        members: {}
        type: cuboid
        priority: 0
`), 0o644); err != nil {
		t.Fatal(err)
	}
	wg.regions.reload()
	api, _ := worldguard.Lookup(srv.Host())
	at := func(x float64) plugin.Location { return plugin.Location{World: w, Pos: plugin.Vec3{X: x, Y: 64, Z: 5}} }
	q := api.Query()
	if q.QueryState(at(5), nil, worldguard.FlagPVP) != worldguard.StateDeny || q.QueryState(at(20), nil, worldguard.FlagPVP) != worldguard.StateNone {
		t.Fatal("pvp is not denied in the arena only")
	}

	m, ok := api.RegionManager(w)
	if !ok {
		t.Fatal("no region manager for the overworld")
	}
	r, _ := worldguard.NewCuboidRegion("lobby", plugin.BlockPos{X: 100}, plugin.BlockPos{X: 110, Y: 10, Z: 10})
	m.AddRegion(r)
	wg.regions.backgroundSave()
	if saved := read(t, filepath.Join(dir, "regions.yml")); !strings.Contains(saved, "    lobby:") {
		t.Fatalf("lobby not saved:\n%s", saved)
	}

	// A file that does not parse protects the whole world.
	if err := os.WriteFile(filepath.Join(dir, "regions.yml"), []byte("regions: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wg.regions.reload()
	set := q.ApplicableRegions(at(500))
	if !set.Virtual() || set.QueryValue(nil, worldguard.FlagDenyMessage) != worldguard.FailedLoadMessage {
		t.Fatal("a failed load does not protect the world")
	}
}
