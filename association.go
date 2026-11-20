package worldguard

import "slices"

import "strings"

// Association is how a subject relates to a region. WorldGuard: Association
type Association uint8

const (
	AssociationOwner Association = iota
	AssociationMember
	AssociationNonMember
)

// RegionAssociable is something a flag can be queried for: a player, or a
// block or entity judged by the regions it is in. WorldGuard:
// RegionAssociable
type RegionAssociable interface {
	// Association answers the subject's association with regions, a region
	// and its parents walked so far.
	Association(regions []*Region) Association
}

// ConstantAssociation is a subject with one association to every region.
// WorldGuard: Associables.constant
type ConstantAssociation Association

func (c ConstantAssociation) Association([]*Region) Association { return Association(c) }

// RegionGroup is a set of associations a flag value applies to.
// WorldGuard: RegionGroup
type RegionGroup uint8

const (
	RegionGroupMembers RegionGroup = iota
	RegionGroupOwners
	RegionGroupNonMembers
	RegionGroupNonOwners
	RegionGroupAll
	RegionGroupNone
)

var regionGroupNames = [...]string{"MEMBERS", "OWNERS", "NON_MEMBERS", "NON_OWNERS", "ALL", "NONE"}

// String answers the group's name as regions.yml stores it.
func (g RegionGroup) String() string { return regionGroupNames[g] }

// Contains reports whether the group holds a.
func (g RegionGroup) Contains(a Association) bool {
	switch g {
	case RegionGroupMembers:
		return a == AssociationMember || a == AssociationOwner
	case RegionGroupOwners:
		return a == AssociationOwner
	case RegionGroupNonMembers:
		return a == AssociationNonMember
	case RegionGroupNonOwners:
		return a == AssociationMember || a == AssociationNonMember
	case RegionGroupAll:
		return true
	}
	return false
}

// parseRegionGroup reads a group's name or one of the aliases /rg flag -g
// takes. WorldGuard: RegionGroupFlag.detectValue
func parseRegionGroup(s string) (RegionGroup, bool) {
	s = strings.TrimSpace(s)
	for i, n := range regionGroupNames {
		if strings.EqualFold(s, n) {
			return RegionGroup(i), true
		}
	}
	switch strings.ToLower(s) {
	case "members", "member":
		return RegionGroupMembers, true
	case "owners", "owner":
		return RegionGroupOwners, true
	case "nonowners", "nonowner":
		return RegionGroupNonOwners, true
	case "nonmembers", "nonmember":
		return RegionGroupNonMembers, true
	case "everyone", "anyone", "all":
		return RegionGroupAll, true
	case "none", "noone", "deny":
		return RegionGroupNone, true
	}
	return 0, false
}

// IsMemberOfGroup reports whether player belongs to group in region; a nil
// player is no one's member. WorldGuard: RegionGroupFlag.isMember
func IsMemberOfGroup(region *Region, group RegionGroup, player LocalPlayer) bool {
	switch group {
	case RegionGroupAll:
		return true
	case RegionGroupOwners:
		return player != nil && region.IsOwner(player)
	case RegionGroupMembers:
		return player != nil && region.IsMember(player)
	case RegionGroupNonOwners:
		return player == nil || !region.IsOwner(player)
	case RegionGroupNonMembers:
		return player == nil || !region.IsMember(player)
	}
	return false
}

// IsMemberOfGroupIn is IsMemberOfGroup over every region of set.
func IsMemberOfGroupIn(set ApplicableRegionSet, group RegionGroup, player LocalPlayer) bool {
	switch group {
	case RegionGroupAll:
		return true
	case RegionGroupOwners:
		return set.IsOwnerOfAll(player)
	case RegionGroupMembers:
		return set.IsMemberOfAll(player)
	case RegionGroupNonOwners:
		return !set.IsOwnerOfAll(player)
	case RegionGroupNonMembers:
		return !set.IsMemberOfAll(player)
	}
	return false
}

// LocalPlayer is a player as regions see one: an identity and the
// permission groups it is in. WorldGuard: LocalPlayer
type LocalPlayer interface {
	RegionAssociable
	UUID() UUID
	Name() string
	// HasGroup reports whether the player is in the permission group.
	HasGroup(group string) bool
}

// PlayerAssociation is a LocalPlayer's Association: owner of any region
// first, else member of any. WorldGuard: LocalPlayer.getAssociation
func PlayerAssociation(p LocalPlayer, regions []*Region) Association {
	member := false
	for _, r := range regions {
		if r.IsOwner(p) {
			return AssociationOwner
		}
		if !member && r.IsMember(p) {
			member = true
		}
	}
	if member {
		return AssociationMember
	}
	return AssociationNonMember
}

// RegionOverlapAssociation associates a subject with the regions it is in
// (Source), as owner of those, for a block or entity acting on another
// place. With MaxPriority set, only the highest priority source regions
// count. Source may be filled later by Lazy. WorldGuard:
// RegionOverlapAssociation, DelayedRegionOverlapAssociation
type RegionOverlapAssociation struct {
	Source      map[*Region]bool
	MaxPriority bool
	// Lazy, when Source is nil, fills it on the first query.
	Lazy func() []*Region

	maxPriority        int
	maxPriorityRegions map[*Region]bool
	computed           bool
}

// NewOverlapAssociation answers the association of a subject in regions.
func NewOverlapAssociation(regions []*Region, maxPriority bool) *RegionOverlapAssociation {
	a := &RegionOverlapAssociation{Source: map[*Region]bool{}, MaxPriority: maxPriority}
	for _, r := range regions {
		a.Source[r] = true
	}
	return a
}

func (a *RegionOverlapAssociation) prepare() {
	if a.computed {
		return
	}
	a.computed = true
	if a.Source == nil {
		a.Source = map[*Region]bool{}
		if a.Lazy != nil {
			for _, r := range a.Lazy() {
				a.Source[r] = true
			}
		}
	}
	best := 0
	bestRegions := map[*Region]bool{}
	for r := range a.Source {
		p := r.Priority()
		if p > best {
			best = p
			clear(bestRegions)
			bestRegions[r] = true
		} else if p == best {
			bestRegions[r] = true
		}
	}
	a.maxPriority, a.maxPriorityRegions = best, bestRegions
}

func (a *RegionOverlapAssociation) Association(regions []*Region) Association {
	a.prepare()
	for _, region := range regions {
		for ; region != nil; region = region.Parent() {
			if region.ID() == GlobalRegionID && len(a.Source) == 0 {
				return AssociationOwner
			}
			if a.Source[region] && (!a.MaxPriority || region.Priority() == a.maxPriority) {
				return AssociationOwner
			}
			source := a.Source
			if a.MaxPriority {
				source = a.maxPriorityRegions
			}
			domains, _ := EffectiveFlag(region, FlagNonPlayerProtectionDomains, a).([]any)
			if a.sharesDomain(source, domains) {
				return AssociationOwner
			}
		}
	}
	return AssociationNonMember
}

// sharesDomain reports whether a source region has one of domains in its
// nonplayer-protection-domains.
func (a *RegionOverlapAssociation) sharesDomain(source map[*Region]bool, domains []any) bool {
	if len(domains) == 0 {
		return false
	}
	for r := range source {
		own, _ := EffectiveFlag(r, FlagNonPlayerProtectionDomains, a).([]any)
		for _, d := range own {
			if slices.Contains(domains, d) {
				return true
			}
		}
	}
	return false
}
