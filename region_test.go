package worldguard

import (
	"testing"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

type memStore struct{ regions []*Region }

func (m *memStore) Name() string                             { return "test" }
func (m *memStore) LoadAll(*FlagRegistry) ([]*Region, error) { return m.regions, nil }
func (m *memStore) SaveAll(r []*Region) error                { m.regions = r; return nil }

func pos(x, y, z int) plugin.BlockPos { return plugin.BlockPos{X: x, Y: y, Z: z} }

func cuboid(t *testing.T, id string, a, b plugin.BlockPos) *Region {
	t.Helper()
	r, err := NewCuboidRegion(id, a, b)
	must(t, err)
	return r
}

func polygon(t *testing.T, id string, minY, maxY int, pts ...Point2D) *Region {
	t.Helper()
	r, err := NewPolygonRegion(id, pts, minY, maxY)
	must(t, err)
	return r
}

// overlapWorld is WorldGuard's RegionOverlapTest and RegionPriorityTest
// setup: a polygon courtyard owned by a group, a fountain in it.
type overlapWorld struct {
	m                         *RegionManager
	courtyard, fountain       *Region
	tetsu, alex               *testPlayer
	inFountain, inCourtyard   plugin.BlockPos
	outside, inNoFire         plugin.BlockPos
	courtyardPri, fountainPri int
}

func newOverlapWorld(t *testing.T, courtyardPri, fountainPri int) *overlapWorld {
	w := &overlapWorld{m: NewRegionManager(&memStore{}, nil),
		tetsu: newTestPlayer("tetsu", "member", "courtyard"), alex: newTestPlayer("alex", "member"),
		inFountain: pos(2, 2, 2), inCourtyard: pos(7, 7, 7), outside: pos(15, 15, 15), inNoFire: pos(150, 150, 150)}
	w.courtyard = polygon(t, "courtyard", 0, 10, Point2D{0, 0}, Point2D{10, 0}, Point2D{10, 10}, Point2D{0, 10})
	w.courtyard.Owners().AddGroup("courtyard")
	w.courtyard.SetPriority(courtyardPri)
	w.m.AddRegion(w.courtyard)
	w.fountain = cuboid(t, "fountain", pos(0, 0, 0), pos(5, 5, 5))
	w.fountain.Members().AddGroup("member")
	w.fountain.SetPriority(fountainPri)
	w.m.AddRegion(w.fountain)
	must(t, w.fountain.SetParent(w.courtyard))
	w.fountain.SetFlag(FlagFireSpread, StateDeny)
	noFire := cuboid(t, "nofire", pos(100, 100, 100), pos(200, 200, 200))
	w.m.AddRegion(noFire)
	noFire.SetFlag(FlagFireSpread, StateDeny)
	return w
}

func (w *overlapWorld) at(p plugin.BlockPos) ApplicableRegionSet {
	return w.m.ApplicableRegions(p, ComputeParents)
}

func TestRegionOverlap(t *testing.T) {
	w := newOverlapWorld(t, 5, 10)
	isTrue(t, w.at(w.outside).TestState(nil, FlagFireSpread))
	isTrue(t, w.at(w.inCourtyard).TestState(nil, FlagFireSpread))
	isFalse(t, w.at(w.inFountain).TestState(nil, FlagFireSpread))
	isFalse(t, w.at(w.inNoFire).TestState(nil, FlagFireSpread))

	for _, p := range []plugin.BlockPos{w.outside, w.inCourtyard, w.inFountain} {
		isTrue(t, w.at(p).TestState(w.tetsu, FlagBuild))
	}
	isTrue(t, w.at(w.outside).TestState(w.alex, FlagBuild))
	isFalse(t, w.at(w.inCourtyard).TestState(w.alex, FlagBuild))
	isTrue(t, w.at(w.inFountain).TestState(w.alex, FlagBuild))
}

func TestRegionOverlapNonPlayer(t *testing.T) {
	w := newOverlapWorld(t, 5, 10)
	for _, maxPri := range []bool{false, true} {
		one := NewOverlapAssociation([]*Region{w.courtyard}, maxPri)
		for _, p := range []plugin.BlockPos{w.outside, w.inCourtyard, w.inFountain} {
			isTrue(t, w.at(p).TestState(one, FlagBuild))
		}
		both := NewOverlapAssociation([]*Region{w.fountain, w.courtyard}, maxPri)
		isTrue(t, w.at(w.outside).TestState(both, FlagBuild))
		isTrue(t, maxPri != w.at(w.inCourtyard).TestState(both, FlagBuild))
		isTrue(t, w.at(w.inFountain).TestState(both, FlagBuild))
		none := NewOverlapAssociation(nil, maxPri)
		isTrue(t, w.at(w.outside).TestState(none, FlagBuild))
		isFalse(t, w.at(w.inCourtyard).TestState(none, FlagBuild))
		isFalse(t, w.at(w.inFountain).TestState(none, FlagBuild))
	}
}

func TestRegionPriority(t *testing.T) {
	w := newOverlapWorld(t, 0, 0)
	w.courtyard.SetFlag(FlagMobSpawning, StateDeny)
	w.fountain.SetFlag(FlagMobSpawning, StateAllow)
	isTrue(t, w.at(w.inCourtyard).TestState(nil, FlagFireSpread))
	isFalse(t, w.at(w.inCourtyard).TestState(nil, FlagMobSpawning))
	isFalse(t, w.at(w.inFountain).TestState(nil, FlagFireSpread))
	isTrue(t, w.at(w.inFountain).TestState(nil, FlagMobSpawning))

	w.courtyard.SetPriority(5)
	isTrue(t, w.at(w.inCourtyard).TestState(nil, FlagFireSpread))
	isFalse(t, w.at(w.inFountain).TestState(nil, FlagFireSpread))

	w.courtyard.SetPriority(0)
	w.fountain.SetPriority(5)
	isFalse(t, w.at(w.inCourtyard).TestState(nil, FlagMobSpawning))
	isTrue(t, w.at(w.inFountain).TestState(nil, FlagMobSpawning))
}

func nested(t *testing.T) *RegionManager {
	m := NewRegionManager(&memStore{}, nil)
	m.AddRegion(cuboid(t, "orphan", pos(0, 0, 0), pos(5, 5, 5)))
	var parent *Region
	for i := range 5 {
		r := cuboid(t, "nested_"+string(rune('0'+i)), pos(0, 0, 0), pos(0, 0, 0))
		if parent != nil {
			must(t, r.SetParent(parent))
		}
		parent = r
	}
	m.AddRegion(parent)
	return m
}

func TestRemoveRegionUnsetParent(t *testing.T) {
	m := nested(t)
	eq(t, m.Len(), 6)
	m.RemoveRegion("nested_0", UnsetParentInChildren)
	eq(t, m.Len(), 5)
	isFalse(t, m.HasRegion("nested_0"))
	first, _ := m.Region("nested_1")
	isTrue(t, first.Parent() == nil)
	second, _ := m.Region("nested_2")
	isTrue(t, second.Parent() == first)
}

func TestRemoveRegionWithChildren(t *testing.T) {
	m := nested(t)
	m.RemoveRegion("nested_1", RemoveChildren)
	eq(t, m.Len(), 2)
	isTrue(t, m.HasRegion("nested_0"))
	isTrue(t, m.HasRegion("orphan"))
}

func TestRegionIntersection(t *testing.T) {
	square := cuboid(t, "square", pos(100, 40, 0), pos(140, 128, 40))
	for _, c := range []struct {
		r    *Region
		want bool
	}{
		{cuboid(t, "normal", pos(80, 40, -20), pos(120, 128, 20)), true},
		{cuboid(t, "small", pos(98, 45, 20), pos(103, 50, 25)), true},
		{cuboid(t, "large", pos(-500, 0, -600), pos(1000, 128, 1000)), true},
		{cuboid(t, "short", pos(50, 40, -1), pos(150, 128, 2)), true},
		{cuboid(t, "long", pos(0, 40, 5), pos(1000, 128, 8)), true},
		{polygon(t, "triangleOverlap", 0, 128, Point2D{90, -10}, Point2D{120, -10}, Point2D{90, 20}), true},
		{polygon(t, "triangleNoOverlap", 0, 128, Point2D{90, -10}, Point2D{105, -10}, Point2D{90, 5}), false},
		{polygon(t, "triangleOverlapNoPoints", 60, 80, Point2D{100, -10}, Point2D{120, 50}, Point2D{140, -20}), true},
		{cuboid(t, "beside", pos(141, 40, 0), pos(150, 128, 40)), false},
		{cuboid(t, "above", pos(100, 129, 0), pos(140, 140, 40)), false},
	} {
		if got := len(square.IntersectingRegions([]*Region{c.r})) == 1; got != c.want {
			t.Errorf("%s: intersects = %v, want %v", c.r.ID(), got, c.want)
		}
	}
	// Two polygons sharing an edge touch but do not overlap.
	a := polygon(t, "a", 0, 10, Point2D{0, 0}, Point2D{10, 0}, Point2D{10, 10})
	b := polygon(t, "b", 0, 10, Point2D{0, 0}, Point2D{10, 10}, Point2D{0, 10})
	isFalse(t, a.Intersects(b))
	c := polygon(t, "c", 0, 10, Point2D{5, 0}, Point2D{15, 0}, Point2D{15, 10}, Point2D{5, 10})
	isTrue(t, a.Intersects(c))
}

func TestPolygonContains(t *testing.T) {
	r := polygon(t, "tri", 0, 10, Point2D{0, 0}, Point2D{10, 0}, Point2D{0, 10})
	isTrue(t, r.Contains(pos(0, 0, 0)))
	isTrue(t, r.Contains(pos(5, 5, 5)))
	isTrue(t, r.Contains(pos(2, 10, 2)))
	isFalse(t, r.Contains(pos(6, 5, 6)))
	isFalse(t, r.Contains(pos(2, 11, 2)))
	eq(t, r.Volume(), 0)
	eq(t, cuboid(t, "c", pos(0, 0, 0), pos(9, 9, 9)).Volume(), 1000)
}

func TestRegionIDs(t *testing.T) {
	isTrue(t, ValidRegionID("Spawn_1,'+-/"))
	isFalse(t, ValidRegionID("bad id"))
	r := cuboid(t, "MixedCase", pos(0, 0, 0), pos(1, 1, 1))
	eq(t, r.ID(), "mixedcase")
	must(t, nil)
	isTrue(t, r.SetParent(r) == ErrCircularInheritance)
}

func parse(t *testing.T, f Flag, in string) any {
	t.Helper()
	v, err := f.Parse(&FlagContext{Input: in, World: "world"})
	must(t, err)
	return v
}

func TestFlagParsing(t *testing.T) {
	eq(t, parse(t, FlagPVP, "deny"), StateDeny)
	eq(t, parse(t, FlagPVP, "ALLOW"), StateAllow)
	isNil(t, parse(t, FlagPVP, "none"))
	_, err := FlagPVP.Parse(&FlagContext{Input: "maybe"})
	eq(t, FlagErrorMessage(err), "Expected none/allow/deny but got 'maybe'")
	eq(t, parse(t, FlagGreeting, `Hi\nthere &aGreen `+"`"+`r`), "Hi\nthere §aGreen §c")
	eq(t, parse(t, FlagBlockedCmds, "spawn,/Home"), []any{"/spawn", "/home"})
	eq(t, parse(t, FlagHealAmount, "5"), 5)
	eq(t, parse(t, FlagHealMaxHealth, "7.5"), 7.5)
	eq(t, parse(t, FlagGameMode, "creative"), plugin.Creative)
	eq(t, parse(t, FlagWeatherLock, "thunder_storm"), plugin.WeatherThunder)
	eq(t, parse(t, FlagDenySpawn, "zombie,minecraft:creeper"), []any{registry.EntityTypeZombie, registry.EntityTypeCreeper})
	eq(t, parse(t, FlagExitOverride, "yes"), true)
	eq(t, parse(t, FlagPVP.GroupFlag(), "nonmembers"), RegionGroupNonMembers)
	_, err = FlagTeleport.Parse(&FlagContext{Input: "1,2,3"})
	eq(t, FlagErrorMessage(err), "Not a player")
}

func TestFlagMarshalling(t *testing.T) {
	loc := Location{World: "world", Pos: plugin.Vec3{X: 1.5, Y: 64, Z: -3}, Yaw: 90}
	values := map[Flag]any{
		FlagPVP: StateDeny, FlagPVP.GroupFlag(): RegionGroupMembers, FlagGreeting: "hi",
		FlagTeleport: loc, FlagDenySpawn: []any{registry.EntityTypeZombie}, FlagHealAmount: 3,
		FlagGameMode: plugin.Adventure,
	}
	raw := MarshalFlags(values)
	eq(t, raw["pvp"], "deny")
	eq(t, raw["pvp-group"], "MEMBERS")
	eq(t, raw["deny-spawn"], []any{"minecraft:zombie"})
	eq(t, raw["game-mode"], "adventure")
	// regions.yml hands back strings, ints, floats and string-keyed maps.
	raw["teleport"] = map[string]any{"world": "world", "x": 1.5, "y": 64, "z": -3, "yaw": 90.0, "pitch": 0.0}
	got := Flags.Unmarshal(raw, false, nil)
	eq(t, got, values)
	ts := NewTimestampFlag("ts")
	eq(t, ts.Marshal(time.Date(2024, 1, 2, 3, 4, 5, 500_000_000, time.UTC)), "2024-01-02T03:04:05.500Z")
}

func TestUnknownFlagsAreKept(t *testing.T) {
	got := Flags.Unmarshal(map[string]any{"some-plugin-flag": "x", "some-plugin-flag-group": "OWNERS"}, true, nil)
	eq(t, len(got), 2)
	f, ok := Flags.Get("some-plugin-flag")
	isTrue(t, ok)
	eq(t, got[f], "x")
	eq(t, MarshalFlags(got), map[string]any{"some-plugin-flag": "x", "some-plugin-flag-group": "OWNERS"})
}

func TestDomain(t *testing.T) {
	d := NewDomain()
	p := newTestPlayer("Bob", "builders")
	isFalse(t, d.Contains(p))
	d.AddGroup("Builders")
	isTrue(t, d.Contains(p))
	d.RemoveGroup("builders")
	d.AddPlayerName("BOB")
	isTrue(t, d.Contains(p))
	d.AddPlayer(p.UUID())
	d.AddGroup("admins")
	eq(t, d.String(nil), "name:bob, uuid:"+p.UUID().String()+"; g:admins")
	eq(t, d.String(func(UUID) (string, bool) { return "Bob", true }), "Bob*, name:bob; g:admins")
}
