package worldguard

import "math"

// Membership is the outcome of a membership test. WorldGuard:
// FlagValueCalculator.Result
type Membership uint8

const (
	MembershipNoRegions Membership = iota
	MembershipFail
	MembershipSuccess
)

// FlagValueCalculator resolves flag values over a priority-sorted list of
// regions and the global region. WorldGuard: FlagValueCalculator
type FlagValueCalculator struct {
	applicable []*Region
}

// NewFlagValueCalculator answers a calculator over regions, sorted by
// NormativeSort, and global (nil for none), which is considered last.
func NewFlagValueCalculator(regions []*Region, global *Region) *FlagValueCalculator {
	applicable := regions
	if global != nil {
		applicable = append(append(make([]*Region, 0, len(regions)+1), regions...), global)
	}
	return &FlagValueCalculator{applicable: applicable}
}

// priorityOf answers a region's priority, the lowest possible for the
// global region.
func priorityOf(r *Region) int {
	if r.ID() == GlobalRegionID {
		return math.MinInt
	}
	return r.Priority()
}

func addParents(ignored map[*Region]bool, r *Region) {
	for p := r.Parent(); p != nil; p = p.Parent() {
		ignored[p] = true
	}
}

// Membership answers whether subject is a member of every region of the
// highest priority that does not let everyone pass through.
func (c *FlagValueCalculator) Membership(subject RegionAssociable) Membership {
	minPriority := math.MinInt
	result := MembershipNoRegions
	ignored := map[*Region]bool{}
	for _, r := range c.applicable {
		p := priorityOf(r)
		if p < minPriority {
			break
		}
		if EffectiveFlag(r, FlagPassthrough, subject) == StateAllow || ignored[r] {
			continue
		}
		minPriority = p
		if !RegionGroupMembers.Contains(subject.Association([]*Region{r})) {
			return MembershipFail
		}
		result = MembershipSuccess
		addParents(ignored, r)
	}
	return result
}

// QueryState answers the combined value of the state flags: deny if any
// denies, allow if any allows.
func (c *FlagValueCalculator) QueryState(subject RegionAssociable, flags ...*StateFlag) State {
	s := StateNone
	for _, f := range flags {
		s = CombineStates(s, stateOf(c.QueryValue(subject, f)))
		if s == StateDeny {
			break
		}
	}
	return s
}

// QueryValue answers the effective value of f, nil for none.
func (c *FlagValueCalculator) QueryValue(subject RegionAssociable, f Flag) any {
	return f.Choose(c.queryAll(subject, f, true))
}

// QueryAllValues answers every value of f of the highest priority set.
func (c *FlagValueCalculator) QueryAllValues(subject RegionAssociable, f Flag) []any {
	return c.queryAll(subject, f, false)
}

func (c *FlagValueCalculator) queryAll(subject RegionAssociable, f Flag, acceptOne bool) []any {
	if f.HasConflictStrategy() {
		acceptOne = false
	}
	if subject == nil && f.RequiresSubject() {
		subject = ConstantAssociation(AssociationNonMember)
	}
	minPriority := math.MinInt
	var considered []any
	ignored := map[*Region]bool{}
	for _, r := range c.applicable {
		p := priorityOf(r)
		if p < minPriority {
			break
		}
		if ignored[r] {
			continue
		}
		if v := EffectiveFlag(r, f, subject); v != nil {
			minPriority = p
			if acceptOne {
				return []any{v}
			}
			considered = append(considered, v)
		}
		addParents(ignored, r)
		if p != minPriority && f.ImplicitlySetWithMembership() &&
			EffectiveFlag(r, FlagPassthrough, subject) != StateAllow {
			minPriority = p
		}
	}
	if f.UsesMembershipAsDefault() && len(considered) == 0 {
		switch c.Membership(subject) {
		case MembershipFail:
			return nil
		case MembershipSuccess:
			return []any{StateAllow}
		}
	}
	if len(considered) == 0 {
		if d := f.Default(); d != nil {
			return []any{d}
		}
		return nil
	}
	return considered
}

// QueryMapValue answers the value under key of the map flag f, falling back
// to the flag fallback (nil for none) in regions whose map lacks the key.
func (c *FlagValueCalculator) QueryMapValue(subject RegionAssociable, f *MapFlag, key any, fallback Flag) any {
	minPriority := math.MinInt
	var considered, fallbacks []any
	ignored := map[*Region]bool{}
	for _, r := range c.applicable {
		p := priorityOf(r)
		if p < minPriority {
			break
		}
		if ignored[r] {
			continue
		}
		if v := EffectiveMapValue(r, f, key, subject); v != nil {
			minPriority = p
			considered = append(considered, v)
		} else if fallback != nil {
			if v := EffectiveFlag(r, fallback, subject); v != nil {
				minPriority = p
				fallbacks = append(fallbacks, v)
			}
		}
		addParents(ignored, r)
	}
	if len(considered) == 0 {
		if fallback != nil && len(fallbacks) > 0 {
			return fallback.Choose(fallbacks)
		}
		if d := f.value.Default(); d != nil {
			return d
		}
		if fallback != nil {
			return fallback.Default()
		}
		return nil
	}
	return f.value.Choose(considered)
}

// groupAllows reports whether the region group flag of f, as set on
// region or defaulted, lets subject use a value seen on the chain.
func groupAllows(region *Region, f Flag, subject RegionAssociable, seen []*Region) bool {
	gf := f.GroupFlag()
	if gf == nil {
		return true
	}
	group := gf.def
	if g, ok := region.Flag(gf).(RegionGroup); ok {
		group = g
	}
	if subject == nil {
		return group.Contains(AssociationNonMember)
	}
	return group.Contains(subject.Association(seen))
}

// EffectiveFlag answers the value of f on region or its nearest parent that
// sets it for subject, nil for none. On the global region passthrough is
// allow unless it is set to deny or the region has members, and the build
// flag's allow is ignored. WorldGuard: FlagValueCalculator.getEffectiveFlagOf
func EffectiveFlag(region *Region, f Flag, subject RegionAssociable) any {
	if region.ID() == GlobalRegionID {
		if f == Flag(FlagPassthrough) {
			pt := stateOf(region.Flag(FlagPassthrough))
			if pt == StateDeny || pt != StateAllow && region.HasMembersOrOwners() {
				return nil
			}
			return StateAllow
		}
		if sf, ok := f.(*StateFlag); ok && sf.PreventsAllowOnGlobal() {
			if v := stateOf(region.Flag(sf)); v != StateAllow && v != StateNone {
				return v
			}
			return nil
		}
	}
	var seen []*Region
	for cur := region; cur != nil; cur = cur.Parent() {
		seen = append(seen, cur)
		if v := cur.Flag(f); v != nil && groupAllows(cur, f, subject, seen) {
			return v
		}
	}
	return nil
}

// EffectiveMapValue answers the value under key of the map flag on region
// or its nearest parent that has it for subject, nil for none.
func EffectiveMapValue(region *Region, f *MapFlag, key any, subject RegionAssociable) any {
	var seen []*Region
	for cur := region; cur != nil; cur = cur.Parent() {
		seen = append(seen, cur)
		m, _ := cur.Flag(f).(map[any]any)
		if v, ok := m[key]; ok && groupAllows(cur, f, subject, seen) {
			return v
		}
	}
	return nil
}
