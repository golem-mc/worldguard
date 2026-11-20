package worldguard

import "github.com/golem-mc/golem/plugin"

// RegionQuery answers flag queries at locations, through the regions of
// the location's world. WorldGuard: RegionQuery
type RegionQuery struct {
	// Applicable answers the regions at loc, with their parents.
	Applicable func(loc plugin.Location) ApplicableRegionSet
}

// ApplicableRegions answers the regions at loc.
func (q RegionQuery) ApplicableRegions(loc plugin.Location) ApplicableRegionSet {
	return q.Applicable(loc)
}

// TestBuild reports whether subject may build at loc: the build flag,
// with flags (when given) able to allow what build alone does not, and deny.
func (q RegionQuery) TestBuild(loc plugin.Location, subject RegionAssociable, flags ...*StateFlag) bool {
	if len(flags) == 0 {
		return q.TestState(loc, subject, FlagBuild)
	}
	return TestStates(DenyToNone(q.QueryState(loc, subject, FlagBuild)), q.QueryState(loc, subject, flags...))
}

// TestBuildMap is TestBuild with a map flag's value under key (falling
// back to fallback) also able to allow or deny.
func (q RegionQuery) TestBuildMap(loc plugin.Location, subject RegionAssociable, f *MapFlag, key any,
	fallback *StateFlag, flags ...*StateFlag) bool {
	if f == nil {
		return q.TestBuild(loc, subject, flags...)
	}
	var fb Flag
	if fallback != nil {
		fb = fallback
	}
	states := []State{DenyToNone(q.QueryState(loc, subject, FlagBuild)), stateOf(q.QueryMapValue(loc, subject, f, key, fb))}
	if len(flags) > 0 {
		states = append(states, q.QueryState(loc, subject, flags...))
	}
	return TestStates(states...)
}

// TestState reports whether the flags combine to allow at loc.
func (q RegionQuery) TestState(loc plugin.Location, subject RegionAssociable, flags ...*StateFlag) bool {
	return q.QueryState(loc, subject, flags...) == StateAllow
}

// QueryState answers the flags' combined state at loc.
func (q RegionQuery) QueryState(loc plugin.Location, subject RegionAssociable, flags ...*StateFlag) State {
	return q.Applicable(loc).QueryState(subject, flags...)
}

// QueryValue answers the value of f at loc, nil for none.
func (q RegionQuery) QueryValue(loc plugin.Location, subject RegionAssociable, f Flag) any {
	return q.Applicable(loc).QueryValue(subject, f)
}

// QueryMapValue answers the map flag's value under key at loc.
func (q RegionQuery) QueryMapValue(loc plugin.Location, subject RegionAssociable, f *MapFlag, key any, fallback Flag) any {
	return q.Applicable(loc).QueryMapValue(subject, f, key, fallback)
}

// QueryAllValues answers every value of f at loc of the highest priority.
func (q RegionQuery) QueryAllValues(loc plugin.Location, subject RegionAssociable, f Flag) []any {
	return q.Applicable(loc).QueryAllValues(subject, f)
}

// Value answers f's value in set as a T, false when there is none.
func Value[T any](set ApplicableRegionSet, subject RegionAssociable, f Flag) (T, bool) {
	v, ok := set.QueryValue(subject, f).(T)
	return v, ok
}
