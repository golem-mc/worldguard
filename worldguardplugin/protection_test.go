package worldguardplugin

import (
	"slices"
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard"
)

// addRegion adds a cuboid region to the overworld, owned by owners.
func addRegion(t *testing.T, wg *WorldGuard, id string, a, b plugin.BlockPos, owners ...plugin.Player) *worldguard.Region {
	t.Helper()
	m, ok := wg.Regions(wg.Server.Overworld())
	if !ok {
		t.Fatal("no regions in the overworld")
	}
	r, err := worldguard.NewCuboidRegion(id, a, b)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range owners {
		r.Owners().AddPlayer(o.UUID())
	}
	m.AddRegion(r)
	return r
}

func blockAt(wg *WorldGuard, x, y, z int) registry.Block {
	return wg.Server.Overworld().BlockState(plugin.BlockPos{X: x, Y: y, Z: z}).Block()
}

func said(lines []string, want string) bool {
	return slices.ContainsFunc(lines, func(l string) bool { return strings.Contains(l, want) })
}

func TestRegionKeepsNonMembersFromBuilding(t *testing.T) {
	srv, wg := start(t, plugintest.WithGameMode("creative"))
	alice, bob := srv.Join("Alice"), srv.Join("Bob")
	addRegion(t, wg, "home", plugin.BlockPos{X: 0, Y: 0, Z: 0}, plugin.BlockPos{X: 10, Y: 100, Z: 10}, alice.Handle())

	bob.StartBreak(3, 63, 3)
	if msgs := bob.Messages(2); !said(msgs, "Hey! Sorry, but you can't break that block here.") {
		t.Errorf("Bob was told %q", msgs)
	}
	if blockAt(wg, 3, 63, 3) != registry.BlockStone {
		t.Fatal("Bob broke a block in Alice's region")
	}
	bob.Handle().Inventory().SetItem(0, plugin.NewStack(registry.ItemStone, 8))
	bob.PlaceBlock(4, 63, 4, 1)
	// Deny messages come at most every half second, so this one is not told.
	bob.Read(2)
	if blockAt(wg, 4, 64, 4) != registry.BlockAir {
		t.Fatal("Bob placed a block in Alice's region")
	}
	alice.StartBreak(3, 63, 3)
	alice.Read(2)
	if blockAt(wg, 3, 63, 3) != registry.BlockAir {
		t.Fatal("Alice could not break a block in her own region")
	}
	// Outside the region anyone builds.
	bob.StartBreak(-3, 63, -3)
	bob.Read(2)
	if blockAt(wg, -3, 63, -3) != registry.BlockAir {
		t.Fatal("Bob could not break a block outside the region")
	}
}

func TestFlagsOverrideMembership(t *testing.T) {
	srv, wg := start(t, plugintest.WithGameMode("creative"))
	bob := srv.Join("Bob")
	r := addRegion(t, wg, "open", plugin.BlockPos{X: 0, Y: 0, Z: 0}, plugin.BlockPos{X: 10, Y: 100, Z: 10})
	r.SetFlag(worldguard.FlagBlockBreak, worldguard.StateAllow)

	bob.StartBreak(3, 63, 3)
	bob.Read(2)
	if blockAt(wg, 3, 63, 3) != registry.BlockAir {
		t.Fatal("block-break allow did not let a non-member break")
	}
	bob.Handle().Inventory().SetItem(0, plugin.NewStack(registry.ItemStone, 8))
	bob.PlaceBlock(4, 63, 4, 1)
	bob.Read(2)
	if blockAt(wg, 4, 64, 4) != registry.BlockAir {
		t.Fatal("block-break allow let a non-member place")
	}
}

func TestPvPFlag(t *testing.T) {
	srv, wg := start(t)
	alice, bob := srv.Join("Alice"), srv.Join("Bob")
	r := addRegion(t, wg, "arena", plugin.BlockPos{X: -10, Y: 0, Z: -10}, plugin.BlockPos{X: 10, Y: 100, Z: 10}, alice.Handle(), bob.Handle())
	r.SetFlag(worldguard.FlagPVP, worldguard.StateDeny)

	alice.Attack(bob.Handle().ID())
	if msgs := alice.Messages(2); !said(msgs, "can't PvP here") {
		t.Errorf("Alice was told %q", msgs)
	}
	if h := bob.Handle().Health(); h < 20 {
		t.Fatalf("Bob lost health in a pvp deny region: %v", h)
	}
	r.SetFlag(worldguard.FlagPVP, nil)
	alice.Attack(bob.Handle().ID())
	alice.Read(2)
	if h := bob.Handle().Health(); h >= 20 {
		t.Fatal("Bob took no damage once pvp was unset for members")
	}
}

func TestChatAndCommandFlags(t *testing.T) {
	srv, wg := start(t)
	alice, bob := srv.Join("Alice"), srv.Join("Bob")
	r := addRegion(t, wg, "quiet", plugin.BlockPos{X: -10, Y: 0, Z: -10}, plugin.BlockPos{X: 10, Y: 100, Z: 10})
	r.SetFlag(worldguard.FlagSendChat, worldguard.StateDeny)
	r.SetFlag(worldguard.FlagBlockedCmds, []any{"/me"})

	bob.Run("hello there")
	if msgs := bob.Messages(2); !said(msgs, "you can't chat here") {
		t.Errorf("Bob was told %q", msgs)
	}
	for _, line := range alice.Chat(1) {
		if strings.Contains(line.Message, "hello there") {
			t.Fatal("Alice heard chat sent from a send-chat deny region")
		}
	}
	bob.Run("/me waves")
	if msgs := bob.Messages(2); !said(msgs, "you can't use /me waves here") {
		t.Errorf("Bob was told %q", msgs)
	}
}

func TestRegionWand(t *testing.T) {
	srv, wg := start(t, plugintest.Ops("Alice"), plugintest.WithGameMode("creative"))
	alice := srv.Join("Alice")
	addRegion(t, wg, "home", plugin.BlockPos{X: 0, Y: 0, Z: 0}, plugin.BlockPos{X: 10, Y: 100, Z: 10}, alice.Handle())
	alice.Handle().Inventory().SetItem(0, plugin.NewStack(registry.ItemLeather, 1))
	alice.PlaceBlock(3, 63, 3, 1)
	msgs := alice.Messages(2)
	if !said(msgs, "Can you build? Yes") || !said(msgs, "Applicable regions: home") {
		t.Fatalf("the wand said %q", msgs)
	}
}

func TestBuildPermissionNodes(t *testing.T) {
	srv, wg := start(t, plugintest.WithGameMode("creative"))
	bob := srv.Join("Bob")
	wg.WorldConfig(wg.Server.Overworld()).BuildPermissions = true
	bob.Handle().Inventory().SetItem(0, plugin.NewStack(registry.ItemStone, 8))
	bob.PlaceBlock(-4, 63, -4, 1)
	if msgs := bob.Messages(2); !said(msgs, "Sorry, but you are not permitted to do that here.") {
		t.Errorf("Bob was told %q", msgs)
	}
	if blockAt(wg, -4, 64, -4) != registry.BlockAir {
		t.Fatal("Bob placed a block without worldguard.build.block.stone.place")
	}
}

func TestTNTFlagKeepsRegionBlocks(t *testing.T) {
	srv, wg := start(t, plugintest.WithGameMode("creative"))
	// A player keeps the chunks at spawn loaded.
	srv.Join("Alice")
	r := addRegion(t, wg, "safe", plugin.BlockPos{X: 0, Y: 0, Z: -10}, plugin.BlockPos{X: 10, Y: 100, Z: 10})
	r.SetFlag(worldguard.FlagTNT, worldguard.StateDeny)
	srv.Console("summon minecraft:tnt 0 64 0 {fuse:1}")
	srv.RunTicks(5)
	if blockAt(wg, 1, 63, 0) != registry.BlockStone {
		t.Fatal("TNT broke a block in a tnt deny region")
	}
	if blockAt(wg, -1, 63, 0) != registry.BlockAir {
		t.Fatal("TNT kept the blocks outside the region")
	}
}

func TestMaterials(t *testing.T) {
	for _, c := range []struct {
		b     registry.Block
		right bool
		want  bool
	}{
		{registry.BlockStone, true, false},
		{registry.BlockOakDoor, true, true},
		{registry.BlockOakDoor, false, false},
		{registry.BlockWhiteConcrete, true, false},
		{registry.BlockWhiteBed, true, true},
		{registry.BlockLever, true, true},
	} {
		if got := blockModifiedOnClick(c.b, c.right); got != c.want {
			t.Errorf("blockModifiedOnClick(%s, %v) = %v", c.b.Name(), c.right, got)
		}
	}
	if !itemAppliedToBlock(registry.ItemIronHoe, registry.BlockDirt) || itemAppliedToBlock(registry.ItemStick, registry.BlockDirt) {
		t.Error("tool application")
	}
	if !inventoryBlock(registry.BlockChest) || inventoryBlock(registry.BlockStone) || !useFlagApplicable(registry.BlockStoneButton) {
		t.Error("block kinds")
	}
	if et, ok := spawnEggEntity(registry.ItemZombieSpawnEgg); !ok || et != registry.EntityTypeZombie {
		t.Error("spawn eggs")
	}
	if et, ok := entityOfItem(registry.ItemOakBoat); !ok || et != registry.EntityTypeOakBoat {
		t.Error("boats")
	}
}

func TestHaltActivityStopsSpawnsAndExplosions(t *testing.T) {
	srv, wg := start(t)
	srv.Join("Alice")
	wg.Config.ActivityHaltToggle = true
	srv.Console("summon minecraft:zombie 3 64 3")
	srv.Console("summon minecraft:tnt 0 64 0 {fuse:1}")
	srv.RunTicks(5)
	for e := range wg.Server.Overworld().Entities() {
		if e.Type() == registry.EntityTypeZombie {
			t.Fatal("a zombie spawned while activity was halted")
		}
	}
	if blockAt(wg, 0, 63, 0) != registry.BlockStone {
		t.Fatal("TNT broke blocks while activity was halted")
	}
}

// regionBlock is a block of the overworld.
func regionBlock(wg *WorldGuard, x, y, z int) plugin.Block {
	return plugin.Block{World: wg.Server.Overworld(), Pos: plugin.BlockPos{X: x, Y: y, Z: z}}
}

func TestIgniteIsJudgedByTheSpreadingFire(t *testing.T) {
	srv, wg := start(t, plugintest.WithGameMode("creative"))
	alice := srv.Join("Alice")
	addRegion(t, wg, "home", plugin.BlockPos{X: 0, Y: 0, Z: 0}, plugin.BlockPos{X: 10, Y: 100, Z: 10}, alice.Handle())
	// The fire at 20 lies outside the region and is not a member's.
	for _, c := range []struct {
		name  string
		src   plugin.Block
		cause plugin.IgniteCause
		deny  bool
	}{
		{"spread from outside", regionBlock(wg, 11, 64, 5), plugin.IgniteSpread, true},
		{"spread from inside", regionBlock(wg, 5, 64, 5), plugin.IgniteSpread, false},
		{"lava from outside", regionBlock(wg, 11, 64, 5), plugin.IgniteLava, true},
		{"lava from inside", regionBlock(wg, 5, 64, 5), plugin.IgniteLava, false},
	} {
		e := &plugin.BlockIgniteEvent{Block: regionBlock(wg, 6, 64, 5), Cause: c.cause, Source: &c.src}
		wg.Server.Events().Fire(e)
		if e.Cancelled() != c.deny {
			t.Errorf("%s: cancelled = %v, want %v", c.name, e.Cancelled(), c.deny)
		}
	}
}

func TestCombustIsJudgedByTheBlockOrEntityBehindIt(t *testing.T) {
	srv, wg := start(t, plugintest.WithGameMode("creative"))
	alice := srv.Join("Alice")
	addRegion(t, wg, "home", plugin.BlockPos{X: 0, Y: 0, Z: 0}, plugin.BlockPos{X: 10, Y: 100, Z: 10}, alice.Handle())
	cow, err := wg.Server.Overworld().Spawn(registry.EntityTypeCow, plugin.Location{World: wg.Server.Overworld(), Pos: plugin.Vec3{X: 5.5, Y: 64, Z: 5.5}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	outside, inside := regionBlock(wg, 11, 64, 5), regionBlock(wg, 5, 64, 5)
	for _, c := range []struct {
		name string
		e    *plugin.EntityCombustEvent
		deny bool
	}{
		{"fire outside", &plugin.EntityCombustEvent{Entity: cow, ByBlock: &outside}, true},
		{"fire inside", &plugin.EntityCombustEvent{Entity: cow, ByBlock: &inside}, false},
		{"unknown", &plugin.EntityCombustEvent{Entity: cow}, false},
	} {
		wg.Server.Events().Fire(c.e)
		if c.e.Cancelled() != c.deny {
			t.Errorf("%s: cancelled = %v, want %v", c.name, c.e.Cancelled(), c.deny)
		}
	}
}

func TestHopperPullIsJudgedByTheHopper(t *testing.T) {
	srv, wg := start(t)
	srv.Join("Alice")
	// The chest stands in a region, the hopper below it outside.
	addRegion(t, wg, "vault", plugin.BlockPos{X: 0, Y: 64, Z: 0}, plugin.BlockPos{X: 0, Y: 64, Z: 0})
	srv.Console("setblock 0 64 0 minecraft:chest{Items:[{Slot:0,id:\"minecraft:stone\",count:5}]}")
	srv.Console("setblock 0 63 0 minecraft:hopper[facing=south]")
	srv.RunTicks(20)
	be, ok := wg.Server.Overworld().BlockEntity(plugin.BlockPos{X: 0, Y: 64, Z: 0})
	if !ok {
		t.Fatal("no chest")
	}
	if got := be.(plugin.Container).Inventory().Item(0); got.Count != 5 {
		t.Fatalf("the hopper pulled from the region's chest: %d stone left", got.Count)
	}
}

func TestEntityOriginJudgesTNTWhereItWasSpawned(t *testing.T) {
	srv, wg := start(t)
	srv.Join("Alice")
	w := wg.Server.Overworld()
	tnt, err := w.Spawn(registry.EntityTypeTnt, plugin.Location{World: w, Pos: plugin.Vec3{X: 5.5, Y: 64, Z: 5.5}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tnt.Teleport(plugin.Location{World: w, Pos: plugin.Vec3{X: 50.5, Y: 64, Z: 50.5}}, plugin.TeleportPlugin)
	p := &protection{wg: wg}
	cfg := wg.WorldConfig(w)
	defer func(old bool) { cfg.UsePaperEntityOrigin = old }(cfg.UsePaperEntityOrigin)
	cfg.UsePaperEntityOrigin = false
	if x := p.entityLocation(tnt).Pos.X; x != 50.5 {
		t.Errorf("without the option the TNT stands at x=%v, want 50.5", x)
	}
	cfg.UsePaperEntityOrigin = true
	if x := p.entityLocation(tnt).Pos.X; x != 5.5 {
		t.Errorf("with the option the TNT stands at x=%v, want its origin 5.5", x)
	}
}
