package worldguard

import (
	"cmp"
	"slices"
)

// ApplicableRegionSet is the regions that apply at a place, and the flag
// queries over them. WorldGuard: ApplicableRegionSet
type ApplicableRegionSet interface {
	// Virtual reports a set that stands for no real regions: regions are
	// off in the world, or their data failed to load.
	Virtual() bool
	// TestState reports whether the state flags combine to allow.
	TestState(subject RegionAssociable, flags ...*StateFlag) bool
	QueryState(subject RegionAssociable, flags ...*StateFlag) State
	QueryValue(subject RegionAssociable, f Flag) any
	QueryMapValue(subject RegionAssociable, f *MapFlag, key any, fallback Flag) any
	QueryAllValues(subject RegionAssociable, f Flag) []any
	IsOwnerOfAll(p LocalPlayer) bool
	IsMemberOfAll(p LocalPlayer) bool
	// Regions answers the regions, highest priority first, the global
	// region not included.
	Regions() []*Region
	Len() int
}

// RegionResultSet is the set of a query's regions. WorldGuard:
// RegionResultSet
type RegionResultSet struct {
	regions []*Region
	calc    *FlagValueCalculator
}

// NewRegionResultSet answers the set of regions (sorted here) and the
// world's global region (nil for none).
func NewRegionResultSet(regions []*Region, global *Region) *RegionResultSet {
	sorted := NormativeSort(regions)
	return &RegionResultSet{regions: sorted, calc: NewFlagValueCalculator(sorted, global)}
}

func (s *RegionResultSet) Virtual() bool { return false }
func (s *RegionResultSet) TestState(subject RegionAssociable, flags ...*StateFlag) bool {
	return s.QueryState(subject, flags...) == StateAllow
}
func (s *RegionResultSet) QueryState(subject RegionAssociable, flags ...*StateFlag) State {
	return s.calc.QueryState(subject, flags...)
}
func (s *RegionResultSet) QueryValue(subject RegionAssociable, f Flag) any {
	return s.calc.QueryValue(subject, f)
}
func (s *RegionResultSet) QueryMapValue(subject RegionAssociable, f *MapFlag, key any, fallback Flag) any {
	return s.calc.QueryMapValue(subject, f, key, fallback)
}
func (s *RegionResultSet) QueryAllValues(subject RegionAssociable, f Flag) []any {
	return s.calc.QueryAllValues(subject, f)
}
func (s *RegionResultSet) IsOwnerOfAll(p LocalPlayer) bool {
	for _, r := range s.regions {
		if !r.IsOwner(p) {
			return false
		}
	}
	return true
}
func (s *RegionResultSet) IsMemberOfAll(p LocalPlayer) bool {
	for _, r := range s.regions {
		if !r.IsMember(p) {
			return false
		}
	}
	return true
}
func (s *RegionResultSet) Regions() []*Region { return slices.Clone(s.regions) }
func (s *RegionResultSet) Len() int           { return len(s.regions) }

// NormativeSort answers regions with every child before its parents, then
// stably sorted by priority, highest first. Unrelated regions of one
// priority are in id order. WorldGuard: NormativeOrders.sort
func NormativeSort(regions []*Region) []*Region {
	in := map[*Region]bool{}
	for _, r := range regions {
		in[r] = true
	}
	type node struct {
		r    *Region
		next *node
	}
	nodes := map[*Region]*node{}
	var roots []*node
	var add func(r *Region) *node
	add = func(r *Region) *node {
		if n, ok := nodes[r]; ok {
			return n
		}
		n := &node{r: r}
		nodes[r] = n
		if p := r.Parent(); p != nil {
			pn := add(p)
			n.next, pn.next = pn.next, n
		} else {
			roots = append(roots, n)
		}
		return n
	}
	byID := slices.SortedFunc(slices.Values(regions), func(a, b *Region) int { return cmp.Compare(a.ID(), b.ID()) })
	for _, r := range byID {
		add(r)
	}
	out := make([]*Region, len(in))
	i := len(out) - 1
	for _, n := range roots {
		for ; n != nil; n = n.next {
			if in[n.r] && i >= 0 {
				out[i] = n.r
				in[n.r] = false
				i--
			}
		}
	}
	slices.SortStableFunc(out, func(a, b *Region) int { return cmp.Compare(b.Priority(), a.Priority()) })
	return out
}

// virtualSet is a set with no regions where build is denied: regions off
// (permissive, everyone is member) or failed to load (no one is).
type virtualSet struct{ member bool }

// PermissiveRegionSet stands for a world with regions off. WorldGuard:
// PermissiveRegionSet
var PermissiveRegionSet ApplicableRegionSet = virtualSet{member: true}

// FailedLoadRegionSet stands for a world whose region data failed to load:
// everything is protected. WorldGuard: FailedLoadRegionSet
var FailedLoadRegionSet ApplicableRegionSet = virtualSet{member: false}

// FailedLoadMessage is the deny message of FailedLoadRegionSet.
const FailedLoadMessage = "Region data for WorldGuard failed to load for this world, so " +
	"everything has been protected as a precaution. Please inform a server administrator."

func (v virtualSet) Virtual() bool { return true }
func (v virtualSet) TestState(subject RegionAssociable, flags ...*StateFlag) bool {
	return v.QueryState(subject, flags...) == StateAllow
}
func (v virtualSet) QueryState(subject RegionAssociable, flags ...*StateFlag) State {
	s := StateNone
	for _, f := range flags {
		s = CombineStates(s, stateOf(v.QueryValue(subject, f)))
	}
	return s
}
func (v virtualSet) QueryValue(_ RegionAssociable, f Flag) any {
	if f == Flag(FlagBuild) {
		return StateDeny
	}
	if !v.member && f == Flag(FlagDenyMessage) {
		return FailedLoadMessage
	}
	return f.Default()
}
func (v virtualSet) QueryMapValue(_ RegionAssociable, f *MapFlag, key any, fallback Flag) any {
	if m, ok := f.Default().(map[any]any); ok {
		return m[key]
	}
	if fallback != nil {
		return fallback.Default()
	}
	return nil
}
func (v virtualSet) QueryAllValues(subject RegionAssociable, f Flag) []any {
	if val := v.QueryValue(subject, f); val != nil {
		return []any{val}
	}
	return nil
}
func (v virtualSet) IsOwnerOfAll(LocalPlayer) bool  { return v.member }
func (v virtualSet) IsMemberOfAll(LocalPlayer) bool { return v.member }
func (v virtualSet) Regions() []*Region             { return nil }
func (v virtualSet) Len() int                       { return 0 }
