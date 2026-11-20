package worldguard

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/google/uuid"
)

// testPlayer is a LocalPlayer with fixed groups. WorldGuard: TestPlayer
type testPlayer struct {
	id     UUID
	name   string
	groups []string
}

func newTestPlayer(name string, groups ...string) *testPlayer {
	return &testPlayer{id: uuid.NewSHA1(uuid.NameSpaceOID, []byte(name)), name: name, groups: groups}
}

func (p *testPlayer) UUID() UUID   { return p.id }
func (p *testPlayer) Name() string { return p.name }
func (p *testPlayer) HasGroup(g string) bool {
	return slices.ContainsFunc(p.groups, func(x string) bool { return strings.EqualFold(x, g) })
}
func (p *testPlayer) Association(regions []*Region) Association { return PlayerAssociation(p, regions) }

// mock builds a list of regions at one place. WorldGuard:
// MockApplicableRegionSet
type mock struct {
	regions []*Region
	glob    *Region
	id, pl  int
}

func newMock() *mock { return &mock{} }

func (m *mock) createPlayer() *testPlayer {
	m.pl++
	return newTestPlayer(fmt.Sprintf("#PLAYER_%d", m.pl))
}

func (m *mock) global() *Region {
	r, err := NewGlobalRegion(GlobalRegionID)
	if err != nil {
		panic(err)
	}
	m.glob = r
	return r
}

func (m *mock) createOutside(priority int) *Region {
	m.id++
	r, err := NewCuboidRegion(fmt.Sprintf("REGION_%d", m.id), plugin.BlockPos{}, plugin.BlockPos{X: 1, Y: 1, Z: 1})
	if err != nil {
		panic(err)
	}
	r.SetPriority(priority)
	return r
}

func (m *mock) add(priority int) *Region {
	r := m.createOutside(priority)
	m.regions = append(m.regions, r)
	return r
}

func (m *mock) addChild(priority int, parent *Region) *Region {
	r := m.add(priority)
	if err := r.SetParent(parent); err != nil {
		panic(err)
	}
	return r
}

func (m *mock) calc() *FlagValueCalculator {
	return NewFlagValueCalculator(NormativeSort(m.regions), m.glob)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func eq(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v (%T), want %v (%T)", got, got, want, want)
	}
}

func isNil(t *testing.T, got any) {
	t.Helper()
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func isTrue(t *testing.T, v bool) {
	t.Helper()
	if !v {
		t.Fatal("want true")
	}
}

func isFalse(t *testing.T, v bool) {
	t.Helper()
	if v {
		t.Fatal("want false")
	}
}

// setOf answers values as a set of their strings.
func setOf[T any](values ...T) map[string]bool {
	out := map[string]bool{}
	for _, v := range values {
		out[fmt.Sprint(v)] = true
	}
	return out
}
