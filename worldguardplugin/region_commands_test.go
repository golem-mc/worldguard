package worldguardplugin

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/worldedit"
	weregion "github.com/golem-mc/worldedit/region"
	"github.com/golem-mc/worldguard"
	"github.com/google/uuid"
)

var (
	bob   = plugin.Profile{Name: "Bob", ID: uuid.MustParse("00000000-0000-0000-0000-0000000000b0")}
	carol = plugin.Profile{Name: "Carol", ID: uuid.MustParse("00000000-0000-0000-0000-0000000000c0")}
)

// startCommands starts a server whose name lookups never leave it: Mojang
// knows no one.
func startCommands(t *testing.T, opts ...plugintest.Option) (*plugintest.Server, *WorldGuard) {
	t.Helper()
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("[]")) }))
	t.Cleanup(mojang.Close)
	old := mojangLookupURL
	mojangLookupURL = mojang.URL
	t.Cleanup(func() { mojangLookupURL = old })
	return start(t, append([]plugintest.Option{plugintest.WithUsercache(bob, carol)}, opts...)...)
}

// await runs ticks until the player was sent a message containing want,
// and answers every message read.
func await(t *testing.T, p *plugintest.Player, want string) []string {
	t.Helper()
	var all []string
	for range 40 {
		all = append(all, p.Messages(2)...)
		if slices.ContainsFunc(all, func(m string) bool { return strings.Contains(m, want) }) {
			return all
		}
	}
	t.Fatalf("no message with %q; got:\n%s", want, strings.Join(all, "\n"))
	return nil
}

// selectCuboid makes a WorldEdit cuboid selection for p.
func selectCuboid(t *testing.T, srv *plugintest.Server, p *plugintest.Player, a, b plugin.BlockPos) {
	t.Helper()
	we, ok := plugin.Service[worldedit.API](srv.Host())
	if !ok {
		t.Fatal("no WorldEdit")
	}
	h := p.Handle()
	sel := we.Session(h).SetSelectorType(h.World(), weregion.SelectorCuboid).(*weregion.CuboidSelector)
	sel.Pos1, sel.Pos2 = &a, &b
}

func TestDefineMembersFlagsAndInfo(t *testing.T) {
	srv, wg := startCommands(t, plugintest.Ops("alice"))
	alice := srv.Join("alice")
	w := alice.Handle().World()

	alice.Run("/rg define spawn")
	await(t, alice, "Please select an area first.")

	selectCuboid(t, srv, alice, plugin.BlockPos{X: -5, Y: 60, Z: -5}, plugin.BlockPos{X: 5, Y: 70, Z: 5})
	alice.Run("/rg define Spawn bob")
	await(t, alice, "A new region has been made named 'spawn'.")
	m, _ := wg.Regions(w)
	spawn, ok := m.Region("spawn")
	if !ok || !spawn.Owners().ContainsUUID(bob.ID) || spawn.Min() != (plugin.BlockPos{X: -5, Y: 60, Z: -5}) {
		t.Fatalf("spawn: %v, owners %s", ok, spawn.Owners().String(nil))
	}

	alice.Run("/rg define spawn")
	await(t, alice, "A region with that name already exists. Please choose another name. To change the shape, use /region redefine spawn.")

	alice.Run("/rg addmember spawn carol g:builders")
	await(t, alice, "Region 'spawn' updated with new members.")
	if !spawn.Members().ContainsUUID(carol.ID) || spawn.Members().Groups()[0] != "builders" {
		t.Fatalf("members %s", spawn.Members().String(nil))
	}
	alice.Run("/rg addmember spawn nobody")
	await(t, alice, "Failed to add new members: Unable to resolve the names nobody")

	alice.Run("/rg flag spawn pvp deny")
	msgs := await(t, alice, "Region flag pvp set on 'spawn' to 'DENY'.")
	if !slices.Contains(msgs, "(Current flags: pvp: DENY [Flags])") {
		t.Errorf("flag output:\n%s", strings.Join(msgs, "\n"))
	}
	alice.Run("/rg flag spawn pvp -g nonmembers")
	await(t, alice, "Region group flag for 'pvp' set.")
	alice.Run("/rg flag spawn greeting &aHello `bthere")
	await(t, alice, "Region flag greeting set on 'spawn' to 'Hello there'.")
	alice.Run("/rg flag spawn nosuchflag allow")
	await(t, alice, "Unknown flag specified: nosuchflag")
	if spawn.Flag(worldguard.FlagPVP) != worldguard.StateDeny || spawn.Flag(worldguard.FlagPVP.GroupFlag()) != worldguard.RegionGroupNonMembers ||
		spawn.Flag(worldguard.FlagGreeting) != "§aHello §9there" {
		t.Fatalf("flags %v", spawn.Flags())
	}

	alice.Run("/rg setpriority spawn 5")
	await(t, alice, "Priority of 'spawn' set to 5 (higher numbers override).")
	alice.Run("/rg info")
	info := strings.Join(await(t, alice, "Bounds:"), "\n")
	for _, want := range []string{"Region Info", "Region: spawn (type=cuboid, priority=5)",
		"Flags: pvp -g NON_MEMBERS: DENY, greeting: §aHello §9there [Flags]", "Owners: Bob [Add] [Rem] [Clr]",
		"Members: Carol; g:builders [Add] [Rem] [Clr]", "Bounds: (-5, 60, -5) -> (5, 70, 5) [Center Teleport]"} {
		if !strings.Contains(info, want) {
			t.Errorf("info lacks %q:\n%s", want, info)
		}
	}

	console := strings.Join(srv.Console("rg info -w "+wg.WorldName(w)+" spawn"), "\n")
	if !strings.Contains(console, "Owners: Bob*") || !strings.Contains(console, "'last seen names'") {
		t.Errorf("console info:\n%s", console)
	}
	alice.Run("/rg flags spawn")
	flags := strings.Join(await(t, alice, "Flags for spawn"), "\n")
	if !strings.Contains(flags, "pvp ....") || !strings.Contains(flags, "Allow Deny") || !strings.Contains(flags, "Page 1 of ") {
		t.Errorf("flags:\n%s", flags)
	}

	alice.Run("/rg flag spawn pvp")
	await(t, alice, "Region flag pvp removed from 'spawn'. (Any -g(roups) were also removed.)")
	if spawn.Flag(worldguard.FlagPVP) != nil || spawn.Flag(worldguard.FlagPVP.GroupFlag()) != nil {
		t.Fatal("pvp still set")
	}

	alice.Run("/rg removemember spawn carol")
	await(t, alice, "Region 'spawn' updated with members removed.")
	alice.Run("/rg removeowner -a spawn")
	await(t, alice, "Region 'spawn' updated with owners removed.")
	if spawn.Members().ContainsUUID(carol.ID) || spawn.Owners().Size() != 0 {
		t.Fatalf("domains %s / %s", spawn.Owners().String(nil), spawn.Members().String(nil))
	}
}

func TestParentsListAndRemove(t *testing.T) {
	srv, wg := startCommands(t, plugintest.Ops("alice"))
	alice := srv.Join("alice")
	m, _ := wg.Regions(alice.Handle().World())
	for _, id := range []string{"town", "plot"} {
		r, _ := worldguard.NewCuboidRegion(id, plugin.BlockPos{X: -5, Y: 60, Z: -5}, plugin.BlockPos{X: 5, Y: 70, Z: 5})
		m.AddRegion(r)
	}
	alice.Run("/rg setparent plot town")
	msgs := strings.Join(await(t, alice, "(Current inheritance:"), "\n")
	if !strings.Contains(msgs, "Inheritance set for region 'plot'.") || !strings.Contains(msgs, "town\n ⤷plot (parent") &&
		!strings.Contains(msgs, " ⤷plot") {
		t.Errorf("setparent:\n%s", msgs)
	}
	alice.Run("/rg setparent town plot")
	await(t, alice, "Uh oh! Setting 'plot' to be the parent of 'town' would cause circular inheritance.")

	alice.Run("/rg info")
	await(t, alice, "You're standing in several regions (please pick one).")

	alice.Run("/rg list")
	list := strings.Join(await(t, alice, "2. town"), "\n")
	if !strings.Contains(list, "1. plot [Info] [TP-Center]") || !strings.Contains(list, "Page 1 of 1") {
		t.Errorf("list:\n%s", list)
	}

	alice.Run("/rg remove town")
	await(t, alice, "Failed to remove region: The region 'town' has child regions.")
	alice.Run("/rg remove -f town")
	await(t, alice, "Successfully removed town, plot.")
	if m.HasRegion("town") || m.HasRegion("plot") {
		t.Fatal("regions left")
	}

	tall, _ := worldguard.NewCuboidRegion("tall", plugin.BlockPos{Y: 0}, plugin.BlockPos{X: 5, Y: 255, Z: 5})
	m.AddRegion(tall)
	if out := srv.Console("rg migrateheights"); len(out) == 0 || !strings.Contains(out[0], "potentially dangerous") {
		t.Fatalf("without -y: %q", out)
	}
	if out := srv.Console("rg migrateheights -y"); !slices.Contains(out, "Migration complete!") {
		t.Fatalf("migrateheights: %q", out)
	}
	w := alice.Handle().World()
	m, _ = wg.Regions(w)
	if tall, _ = m.Region("tall"); tall.Min().Y != w.MinY() || tall.Max().Y != w.MinY()+w.Height()-1 {
		t.Fatalf("tall spans %v to %v", tall.Min(), tall.Max())
	}
}

func TestUsageAndPermissions(t *testing.T) {
	srv, _ := startCommands(t)
	eve := srv.Join("eve")
	eve.Run("/rg")
	if msgs := eve.Messages(2); len(msgs) != 1 || !strings.HasPrefix(msgs[0], "/rg <define|redefine|claim|select|info|list|flag|") {
		t.Errorf("/rg: %q", msgs)
	}
	eve.Run("/region bogus")
	if msgs := eve.Messages(2); len(msgs) != 1 || !strings.HasPrefix(msgs[0], "/region <define|") {
		t.Errorf("/region bogus: %q", msgs)
	}
	eve.Run("/rg define")
	if msgs := eve.Messages(2); len(msgs) != 2 || msgs[0] != "Too few arguments." ||
		msgs[1] != "/rg define [-ng] [-w <world>] <id> [<owner1> [<owner2> [<owners...>]]]" {
		t.Errorf("/rg define: %q", msgs)
	}
	eve.Run("/rg define -x a")
	if msgs := eve.Messages(2); len(msgs) == 0 || msgs[0] != "Unknown flag: x" {
		t.Errorf("/rg define -x: %q", msgs)
	}
	eve.Run("/rg define spawn")
	if msgs := eve.Messages(2); len(msgs) != 1 || msgs[0] != "You don't have permission." {
		t.Errorf("no permission: %q", msgs)
	}
	eve.Run("/rg list")
	if msgs := eve.Messages(2); len(msgs) != 1 || msgs[0] != "You don't have permission." {
		t.Errorf("list: %q", msgs)
	}
	if out := strings.Join(srv.Console("rg info spawn"), "\n"); !strings.Contains(out, "Please specify the world with -w world_name.") {
		t.Errorf("console: %s", out)
	}
}

// grants is a permission plugin granting a fixed set of nodes, wildcards
// included.
type grants struct {
	mu    sync.Mutex
	nodes map[string]bool
}

func (g *grants) Has(_ plugin.CommandSender, node string) (bool, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	v, ok := g.nodes[node]
	return v, ok
}

func TestClaimAndPatternPermissions(t *testing.T) {
	srv, wg := startCommands(t)
	plugin.Provide[plugin.PermissionProvider](srv.Host(), &grants{nodes: map[string]bool{
		"worldguard.region.claim": true, "worldguard.region.info.own.*": true, "worldguard.region.flag.*": true,
	}}, 0)
	dave := srv.Join("dave")
	selectCuboid(t, srv, dave, plugin.BlockPos{X: 0, Y: 60, Z: 0}, plugin.BlockPos{X: 100, Y: 70, Z: 100})
	dave.Run("/rg claim home")
	await(t, dave, "Max. volume: 30000, your volume: 112211")

	selectCuboid(t, srv, dave, plugin.BlockPos{X: 0, Y: 60, Z: 0}, plugin.BlockPos{X: 9, Y: 69, Z: 9})
	dave.Run("/rg claim home")
	await(t, dave, "A new region has been claimed named 'home'.")
	m, _ := wg.Regions(dave.Handle().World())
	home, _ := m.Region("home")
	if !home.Owners().ContainsUUID(dave.Handle().UUID()) {
		t.Fatal("dave does not own home")
	}

	dave.Run("/rg info home")
	await(t, dave, "Region: home (type=cuboid, priority=0)")
	dave.Run("/rg flag home greeting hi")
	await(t, dave, "Region flag greeting set on 'home' to 'hi'.")

	other, _ := worldguard.NewCuboidRegion("other", plugin.BlockPos{X: 50}, plugin.BlockPos{X: 60, Y: 10, Z: 10})
	m.AddRegion(other)
	dave.Run("/rg info other")
	await(t, dave, "You don't have permission.")
}

func TestSelectAndTeleport(t *testing.T) {
	srv, wg := startCommands(t, plugintest.Ops("alice"))
	alice := srv.Join("alice")
	h := alice.Handle()
	m, _ := wg.Regions(h.World())
	pts := []worldguard.Point2D{{X: 10, Z: 10}, {X: 20, Z: 10}, {X: 20, Z: 20}}
	field, _ := worldguard.NewPolygonRegion("field", pts, 50, 80)
	m.AddRegion(field)

	alice.Run("/rg select field")
	await(t, alice, "Region selected as poly2d")
	we, _ := plugin.Service[worldedit.API](srv.Host())
	sel, err := we.Selection(h)
	poly, ok := sel.(*weregion.Polygon2D)
	if err != nil || !ok || len(poly.Points) != 3 || poly.MinY != 50 || poly.MaxY != 80 {
		t.Fatalf("selection %#v, %v", sel, err)
	}

	alice.Run("/rg tp field")
	await(t, alice, "The region has no teleport point associated.")
	alice.Run("/rg flag field teleport 15,64,15")
	await(t, alice, "Region flag teleport set on 'field'")
	alice.Run("/rg tp field")
	await(t, alice, "Teleported you to the region 'field'.")
	if pos := alice.Handle().Location().Pos; pos.X != 15 || pos.Z != 15 {
		t.Fatalf("at %v", pos)
	}

	alice.Run("/rg bypass")
	await(t, alice, "You are no longer bypassing region protection.")
	if wg.HasBypass(h, h.World()) {
		t.Fatal("bypass still on")
	}
	alice.Run("/rg bypass on")
	await(t, alice, "You are now bypassing region protection (as long as you have permission).")
}

func TestParseArgs(t *testing.T) {
	c, err := parseCmdArgs(strings.Split(`-w "my world" spawn -g owners  "a b" -e`, " "), "g:w:eh:")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := c.value('w'); v != "my world" || !c.has('e') || strings.Join(c.args, "|") != "spawn|a b" {
		t.Fatalf("parsed %+v", c)
	}
	if g, _ := c.value('g'); g != "owners" {
		t.Fatalf("group %q", g)
	}
	if _, err := parseCmdArgs([]string{"-w"}, "w:"); err == nil || err.Error() != "No value specified for the '-w' flag." {
		t.Fatalf("hanging flag: %v", err)
	}
	if _, err := parseCmdArgs([]string{"-w", "a", "-w", "b"}, "w:"); err == nil || err.Error() != "Value flag 'w' already given" {
		t.Fatalf("twice: %v", err)
	}
}

func TestCompletion(t *testing.T) {
	_, wg := startCommands(t)
	g := regionCommands()
	console := wg.Server.Console()
	if got := g.suggest(wg, console, "def"); !slices.Equal(got, []string{"define"}) {
		t.Errorf("sub-commands: %q", got)
	}
	world := wg.WorldName(wg.Server.Overworld())
	m, _ := wg.Regions(wg.Server.Overworld())
	r, _ := worldguard.NewCuboidRegion("arena", plugin.BlockPos{}, plugin.BlockPos{X: 1, Y: 1, Z: 1})
	m.AddRegion(r)
	if got := g.suggest(wg, console, "flag -w "+world+" ar"); !slices.Equal(got, []string{"flag -w " + world + " arena"}) {
		t.Errorf("region ids: %q", got)
	}
	if got := g.suggest(wg, console, "flag -w "+world+" arena pv"); !slices.Equal(got, []string{"flag -w " + world + " arena pvp"}) {
		t.Errorf("flag names: %q", got)
	}
	if got := g.suggest(wg, console, "flag -w "+world+" arena pvp "); !slices.Equal(got, []string{
		"flag -w " + world + " arena pvp allow", "flag -w " + world + " arena pvp deny", "flag -w " + world + " arena pvp none"}) {
		t.Errorf("flag values: %q", got)
	}
}

func TestDefineWarnsAboutSpawnProtection(t *testing.T) {
	srv, _ := startCommands(t, plugintest.Ops("alice"), plugintest.WithSpawnProtection(16))
	alice := srv.Join("alice")
	spawn := alice.Handle().World().SpawnPoint().Pos.Block()
	selectCuboid(t, srv, alice, spawn.Offset(plugin.North, 2), spawn.Offset(plugin.South, 2))
	alice.Run("/rg define nearspawn")
	await(t, alice, "This region overlaps vanilla's spawn protection.")

	far := spawn.Offset(plugin.East, 100)
	selectCuboid(t, srv, alice, far, far.Offset(plugin.East, 3))
	alice.Run("/rg define farfromspawn")
	if msgs := await(t, alice, "A new region has been made named 'farfromspawn'."); said(msgs, "spawn protection") {
		t.Errorf("a region far from spawn was reported to overlap it: %q", msgs)
	}
}
