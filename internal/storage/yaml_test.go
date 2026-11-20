package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
	"github.com/google/uuid"
)

// upstreamFile is a regions.yml as WorldGuard 7 on Bukkit writes it.
const upstreamFile = `#
# WorldGuard regions file
#
regions:
    spawn:
        min: {x: -10.0, y: 0.0, z: -10.0}
        max: {x: 10.0, y: 255.0, z: 10.0}
        members:
            players: [oldname]
        flags: {pvp: deny, pvp-group: NON_MEMBERS, greeting: Welcome!, deny-spawn: ['minecraft:zombie'],
            teleport: {world: world, x: 0.5, y: 64.0, z: 0.5, yaw: 90.0, pitch: 0.0}, heal-amount: 2,
            heal-max-health: 20.0, blocked-cmds: [/home], other-plugin-flag: 7}
        owners:
            unique-ids: [069a79f4-44e9-4726-a5be-fca90e38aaf5]
            groups: [admins]
        type: cuboid
        priority: 10
    __global__:
        members: {}
        flags: {build: deny}
        owners: {}
        type: global
        priority: 0
    Plot-1:
        min-y: 0
        max-y: 100
        points:
        - {x: 0, z: 0}
        - {x: 20, z: 0}
        - {x: 20, z: 20}
        members: {}
        flags: {}
        owners: {}
        type: poly2d
        priority: 0
        parent: spawn
    broken:
        flags: {}
        priority: 0
`

func TestLoadUpstreamFileAndSaveItBack(t *testing.T) {
	dir := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(dir, "world"), 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "world", "regions.yml"), []byte(upstreamFile), 0o644))
	store := Driver{Root: dir}.Get("world")
	regions, err := store.LoadAll(worldguard.Flags)
	must(t, err)
	check := func(regions []*worldguard.Region) {
		t.Helper()
		byID := map[string]*worldguard.Region{}
		for _, r := range regions {
			byID[r.ID()] = r
		}
		if len(byID) != 3 {
			t.Fatalf("loaded %d regions, want 3 (the broken one skipped)", len(byID))
		}
		spawn, plot, global := byID["spawn"], byID["plot-1"], byID["__global__"]
		if spawn.Min() != (plugin.BlockPos{X: -10, Z: -10}) || spawn.Max() != (plugin.BlockPos{X: 10, Y: 255, Z: 10}) {
			t.Errorf("spawn spans %v to %v", spawn.Min(), spawn.Max())
		}
		if spawn.Priority() != 10 || plot.Parent() != spawn || global.Type() != worldguard.RegionGlobal {
			t.Errorf("priority %d, parent %v, global type %v", spawn.Priority(), plot.Parent(), global.Type())
		}
		if spawn.Flag(worldguard.FlagPVP) != worldguard.StateDeny ||
			spawn.Flag(worldguard.FlagPVP.GroupFlag()) != worldguard.RegionGroupNonMembers ||
			spawn.Flag(worldguard.FlagGreeting) != "Welcome!" || spawn.Flag(worldguard.FlagHealAmount) != 2 ||
			spawn.Flag(worldguard.FlagHealMaxHealth) != 20.0 {
			t.Errorf("spawn flags: %v", spawn.Flags())
		}
		loc, _ := spawn.Flag(worldguard.FlagTeleport).(worldguard.Location)
		if loc.World != "world" || loc.Pos.X != 0.5 || loc.Yaw != 90 {
			t.Errorf("teleport %+v", loc)
		}
		unknown, _ := worldguard.Flags.Get("other-plugin-flag")
		if spawn.Flag(unknown) != 7 {
			t.Errorf("unknown flag value %v", spawn.Flag(unknown))
		}
		if !spawn.Owners().ContainsUUID(uuid.MustParse("069a79f4-44e9-4726-a5be-fca90e38aaf5")) ||
			spawn.Owners().Groups()[0] != "admins" || !spawn.Members().ContainsName("OldName") {
			t.Errorf("domains: %s / %s", spawn.Owners().String(nil), spawn.Members().String(nil))
		}
		if len(plot.Points()) != 3 || plot.Max().Y != 100 || !plot.Contains(plugin.BlockPos{X: 15, Y: 50, Z: 5}) {
			t.Errorf("plot points %v", plot.Points())
		}
		if global.Flag(worldguard.FlagBuild) != worldguard.StateDeny {
			t.Errorf("global build %v", global.Flag(worldguard.FlagBuild))
		}
	}
	check(regions)
	must(t, store.SaveAll(regions))
	data, err := os.ReadFile(store.Path())
	must(t, err)
	if !strings.HasPrefix(string(data), "#\n# WorldGuard regions file") || !strings.Contains(string(data), "    spawn:\n") {
		t.Fatalf("saved file:\n%s", data)
	}
	again, err := store.LoadAll(worldguard.Flags)
	must(t, err)
	check(again)
}

func TestMissingFileHasNoRegions(t *testing.T) {
	regions, err := Driver{Root: t.TempDir()}.Get("nowhere").LoadAll(worldguard.Flags)
	must(t, err)
	if len(regions) != 0 {
		t.Fatalf("got %d regions", len(regions))
	}
}

func TestBrokenFileFailsToLoad(t *testing.T) {
	dir := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(dir, "w"), 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "w", "regions.yml"), []byte("regions: [\n"), 0o644))
	if _, err := (Driver{Root: dir}).Get("w").LoadAll(worldguard.Flags); err == nil {
		t.Fatal("a broken file loaded")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
