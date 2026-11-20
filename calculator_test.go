// Code translated from WorldGuard's FlagValueCalculatorTest.

package worldguard

import (
	"math"
	"slices"
	"testing"
)

func TestGetMembershipWilderness(t *testing.T) {
	mock := newMock()
	player := mock.createPlayer()
	result := mock.calc()
	eq(t, result.Membership(player), MembershipNoRegions)
}

func TestGetMembershipWildernessWithGlobalRegion(t *testing.T) {
	mock := newMock()
	player := mock.createPlayer()
	global := mock.global()
	_ = global
	result := mock.calc()
	eq(t, result.Membership(player), MembershipNoRegions)
}

func TestGetMembershipGlobalRegion(t *testing.T) {
	mock := newMock()
	global := mock.global()
	_ = global
	player := mock.createPlayer()
	result := mock.calc()
	eq(t, result.Membership(player), MembershipNoRegions)
}

func TestGetMembershipGlobalRegionAndRegion(t *testing.T) {
	mock := newMock()
	global := mock.global()
	_ = global
	region := mock.add(0)
	_ = region
	player := mock.createPlayer()
	result := mock.calc()
	eq(t, result.Membership(player), MembershipFail)
}

func TestGetMembershipPassthroughRegions(t *testing.T) {
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(FlagPassthrough, StateAllow)
	region = mock.add(0)
	region.SetFlag(FlagPassthrough, StateAllow)
	player := mock.createPlayer()
	result := mock.calc()
	eq(t, result.Membership(player), MembershipNoRegions)
}

func TestGetMembershipPassthroughAndRegion(t *testing.T) {
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(FlagPassthrough, StateAllow)
	region = mock.add(0)
	player := mock.createPlayer()
	result := mock.calc()
	eq(t, result.Membership(player), MembershipFail)
}

func TestGetMembershipPassthroughAndRegionMemberOf(t *testing.T) {
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(FlagPassthrough, StateAllow)
	region = mock.add(0)
	player := mock.createPlayer()
	region.Members().AddPlayer(player.UUID())
	result := mock.calc()
	eq(t, result.Membership(player), MembershipSuccess)
}

func TestGetMembershipPassthroughAndRegionMemberOfAndAnotherNot(t *testing.T) {
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(FlagPassthrough, StateAllow)
	region = mock.add(0)
	player := mock.createPlayer()
	region.Members().AddPlayer(player.UUID())
	region = mock.add(0)
	result := mock.calc()
	eq(t, result.Membership(player), MembershipFail)
	region.Members().AddPlayer(mock.createPlayer().UUID())
	result = mock.calc()
	eq(t, result.Membership(player), MembershipFail)
}

func TestGetMembershipPassthroughAndRegionMemberOfAndAnotherNotWithHigherPriority(t *testing.T) {
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(FlagPassthrough, StateAllow)
	region = mock.add(0)
	player := mock.createPlayer()
	region.Members().AddPlayer(player.UUID())
	region = mock.add(10)
	result := mock.calc()
	eq(t, result.Membership(player), MembershipFail)
}

func TestGetMembershipPassthroughAndRegionMemberOfWithHigherPriorityAndAnotherNot(t *testing.T) {
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(FlagPassthrough, StateAllow)
	region = mock.add(10)
	player := mock.createPlayer()
	region.Members().AddPlayer(player.UUID())
	region = mock.add(0)
	result := mock.calc()
	eq(t, result.Membership(player), MembershipSuccess)
}

func TestGetMembershipPassthroughAndRegionMemberOfWithAnotherParent(t *testing.T) {
	mock := newMock()
	passthrough := mock.add(0)
	passthrough.SetFlag(FlagPassthrough, StateAllow)
	parent := mock.add(0)
	region := mock.add(0)
	must(t, region.SetParent(parent))
	player := mock.createPlayer()
	region.Members().AddPlayer(player.UUID())
	result := mock.calc()
	eq(t, result.Membership(player), MembershipSuccess)
}

func TestGetMembershipPassthroughAndRegionMemberOfWithAnotherChild(t *testing.T) {
	mock := newMock()
	passthrough := mock.add(0)
	passthrough.SetFlag(FlagPassthrough, StateAllow)
	parent := mock.add(0)
	region := mock.add(0)
	must(t, region.SetParent(parent))
	player := mock.createPlayer()
	parent.Members().AddPlayer(player.UUID())
	result := mock.calc()
	eq(t, result.Membership(player), MembershipSuccess)
}

func TestGetMembershipPassthroughAndRegionMemberOfWithAnotherChildAndAnother(t *testing.T) {
	mock := newMock()
	passthrough := mock.add(0)
	passthrough.SetFlag(FlagPassthrough, StateAllow)
	parent := mock.add(0)
	region := mock.add(0)
	must(t, region.SetParent(parent))
	region = mock.add(0)
	player := mock.createPlayer()
	parent.Members().AddPlayer(player.UUID())
	result := mock.calc()
	eq(t, result.Membership(player), MembershipFail)
}

func TestGetMembershipThirdPriorityLower(t *testing.T) {
	mock := newMock()
	passthrough := mock.add(0)
	passthrough.SetFlag(FlagPassthrough, StateAllow)
	parent := mock.add(0)
	region := mock.add(0)
	must(t, region.SetParent(parent))
	region = mock.add(0)
	region.SetPriority(-5)
	player := mock.createPlayer()
	parent.Members().AddPlayer(player.UUID())
	result := mock.calc()
	eq(t, result.Membership(player), MembershipSuccess)
}

func TestQueryStateWilderness(t *testing.T) {
	mock := newMock()
	flag1 := NewStateFlag("test1", false)
	flag2 := NewStateFlag("test2", true)
	result := mock.calc()
	eq(t, result.QueryState(nil, flag1), StateNone)
	eq(t, result.QueryState(nil, flag2), StateAllow)
}

func TestQueryValueSingleRegion(t *testing.T) {
	mock := newMock()
	flag1 := NewStateFlag("test1", false)
	flag2 := NewStateFlag("test2", false)
	region := mock.add(0)
	region.SetFlag(flag2, StateDeny)
	result := mock.calc()
	isNil(t, result.QueryValue(nil, flag1))
	eq(t, result.QueryValue(nil, flag2), StateDeny)
}

func TestQueryValueDenyOverridesAllow(t *testing.T) {
	mock := newMock()
	flag1 := NewStateFlag("test1", false)
	flag2 := NewStateFlag("test2", false)
	region := mock.add(0)
	region.SetFlag(flag2, StateDeny)
	region = mock.add(0)
	region.SetFlag(flag2, StateAllow)
	result := mock.calc()
	isNil(t, result.QueryValue(nil, flag1))
	eq(t, result.QueryValue(nil, flag2), StateDeny)
}

func TestQueryValueAllowOverridesNone(t *testing.T) {
	mock := newMock()
	region := mock.add(0)
	flag1 := NewStateFlag("test1", false)
	flag2 := NewStateFlag("test2", false)
	region = mock.add(0)
	region.SetFlag(flag2, StateAllow)
	result := mock.calc()
	isNil(t, result.QueryValue(nil, flag1))
	eq(t, result.QueryValue(nil, flag2), StateAllow)
}

func TestQueryValueMultipleFlags(t *testing.T) {
	mock := newMock()
	flag1 := NewStateFlag("test1", false)
	flag2 := NewStateFlag("test2", false)
	flag3 := NewStateFlag("test3", false)
	region := mock.add(0)
	region.SetFlag(flag1, StateDeny)
	region.SetFlag(flag2, StateAllow)
	region = mock.add(0)
	region.SetFlag(flag2, StateDeny)
	result := mock.calc()
	eq(t, result.QueryValue(nil, flag1), StateDeny)
	eq(t, result.QueryValue(nil, flag2), StateDeny)
	isNil(t, result.QueryValue(nil, flag3))
}

func TestQueryValueFlagsWithRegionGroupsAndInheritance(t *testing.T) {
	mock := newMock()
	flag1 := NewStateFlag("test1", false)
	nonMember := mock.createPlayer()
	member := mock.createPlayer()
	parent := mock.add(0)
	parent.SetFlag(flag1, StateDeny)
	parent.SetFlag(flag1.GroupFlag(), RegionGroupNonMembers)
	region := mock.add(0)
	region.Members().AddPlayer(member.UUID())
	must(t, region.SetParent(parent))
	result := mock.calc()
	eq(t, result.QueryValue(nonMember, flag1), StateDeny)
	isNil(t, result.QueryValue(member, flag1))
}

func TestQueryValueFlagsWithRegionGroupsAndInheritanceAndParentMember(t *testing.T) {
	mock := newMock()
	flag1 := NewStateFlag("test1", false)
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberTwo := mock.createPlayer()
	parent := mock.add(0)
	parent.Members().AddPlayer(memberOne.UUID())
	parent.SetFlag(flag1, StateDeny)
	parent.SetFlag(flag1.GroupFlag(), RegionGroupNonMembers)
	region := mock.add(0)
	region.Members().AddPlayer(memberOne.UUID())
	must(t, region.SetParent(parent))
	result := mock.calc()
	eq(t, result.QueryValue(nonMember, flag1), StateDeny)
	isNil(t, result.QueryValue(memberOne, flag1))
	eq(t, result.QueryValue(memberTwo, flag1), StateDeny)
}

func TestQueryValueFlagsWithRegionGroupsAndPriority(t *testing.T) {
	mock := newMock()
	flag1 := NewStateFlag("test1", false)
	nonMember := mock.createPlayer()
	member := mock.createPlayer()
	lower := mock.add(-1)
	lower.SetFlag(flag1, StateDeny)
	lower.SetFlag(flag1.GroupFlag(), RegionGroupNonMembers)
	region := mock.add(0)
	region.Members().AddPlayer(member.UUID())
	result := mock.calc()
	eq(t, result.QueryValue(nonMember, flag1), StateDeny)
	eq(t, result.QueryValue(member, flag1), StateDeny)
}

func TestQueryValueFlagsWithRegionGroupsAndPriorityAndOveride(t *testing.T) {
	mock := newMock()
	flag1 := NewStateFlag("test1", false)
	nonMember := mock.createPlayer()
	member := mock.createPlayer()
	lower := mock.add(-1)
	lower.SetFlag(flag1, StateDeny)
	lower.SetFlag(flag1.GroupFlag(), RegionGroupNonMembers)
	region := mock.add(0)
	region.SetFlag(flag1, StateAllow)
	region.SetFlag(flag1.GroupFlag(), RegionGroupMembers)
	region.Members().AddPlayer(member.UUID())
	result := mock.calc()
	eq(t, result.QueryValue(nonMember, flag1), StateDeny)
	eq(t, result.QueryValue(member, flag1), StateAllow)
}

func TestQueryValueStringFlag(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	flag1 := NewStateFlag("test1", false)
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region = mock.add(0)
	region.SetFlag(stringFlag1, "test2")
	result := mock.calc()
	isTrue(t, slices.Contains([]any{"test1", "test2"}, result.QueryValue(nil, stringFlag1)))
	isNil(t, result.QueryValue(nil, stringFlag2))
	isNil(t, result.QueryValue(nil, flag1))
}

func TestQueryValueEmptyGlobalRegion(t *testing.T) {
	mock := newMock()
	flag := NewStateFlag("test", true)
	global := mock.global()
	_ = global
	result := mock.calc()
	eq(t, result.QueryValue(nil, flag), StateAllow)
}

func TestQueryValueGlobalRegionAllow(t *testing.T) {
	mock := newMock()
	flag := NewStateFlag("test", true)
	global := mock.global()
	global.SetFlag(flag, StateAllow)
	result := mock.calc()
	eq(t, result.QueryValue(nil, flag), StateAllow)
}

func TestQueryValueGlobalRegionDeny(t *testing.T) {
	mock := newMock()
	flag := NewStateFlag("test", true)
	global := mock.global()
	global.SetFlag(flag, StateDeny)
	result := mock.calc()
	eq(t, result.QueryValue(nil, flag), StateDeny)
}

func TestQueryValueStringFlagWithGlobalRegion(t *testing.T) {
	mock := newMock()
	flag := NewStringFlag("test", "")
	global := mock.global()
	_ = global
	result := mock.calc()
	isNil(t, result.QueryValue(nil, flag))
}

func TestQueryValueStringFlagWithGlobalRegionValueSet(t *testing.T) {
	mock := newMock()
	flag := NewStringFlag("test", "")
	global := mock.global()
	global.SetFlag(flag, "hello")
	result := mock.calc()
	eq(t, result.QueryValue(nil, flag), "hello")
}

func TestQueryValueStringFlagWithGlobalRegionAndRegion(t *testing.T) {
	mock := newMock()
	flag := NewStringFlag("test", "")
	global := mock.global()
	global.SetFlag(flag, "hello")
	region := mock.add(0)
	_ = region
	result := mock.calc()
	eq(t, result.QueryValue(nil, flag), "hello")
}

func TestQueryValueStringFlagWithGlobalRegionAndRegionOverride(t *testing.T) {
	mock := newMock()
	flag := NewStringFlag("test", "")
	global := mock.global()
	global.SetFlag(flag, "hello")
	region := mock.add(0)
	region.SetFlag(flag, "beep")
	result := mock.calc()
	eq(t, result.QueryValue(nil, flag), "beep")
}

func TestQueryValueStringFlagWithEverything(t *testing.T) {
	mock := newMock()
	flag := NewStringFlag("test", "", WithGroup(RegionGroupAll))
	global := mock.global()
	global.SetFlag(flag, "hello")
	parent := mock.add(0)
	parent.SetFlag(flag, "ello there")
	region := mock.add(0)
	region.SetFlag(flag, "beep beep")
	region.SetFlag(flag.GroupFlag(), RegionGroupMembers)
	must(t, region.SetParent(parent))
	nonMember := mock.createPlayer()
	member := mock.createPlayer()
	region.Members().AddPlayer(member.UUID())
	result := mock.calc()
	eq(t, result.QueryValue(nil, flag), "ello there")
	eq(t, result.QueryValue(nonMember, flag), "ello there")
	eq(t, result.QueryValue(member, flag), "beep beep")
}

func TestQueryValueBuildFlagWilderness(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	result := mock.calc()
	eq(t, result.QueryValue(nonMember, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagWildernessAndGlobalRegion(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	global := mock.global()
	_ = global
	result := mock.calc()
	eq(t, result.QueryValue(nonMember, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagWildernessAndGlobalRegionDeny(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	global := mock.global()
	global.SetFlag(FlagBuild, StateDeny)
	result := mock.calc()
	eq(t, result.QueryValue(nonMember, FlagBuild), StateDeny)
}

func TestQueryValueBuildFlagWildernessAndGlobalRegionAllow(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	global := mock.global()
	global.SetFlag(FlagBuild, StateAllow)
	result := mock.calc()
	eq(t, result.QueryValue(nonMember, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagWildernessAndGlobalRegionMembership(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	member := mock.createPlayer()
	global := mock.global()
	global.Members().AddPlayer(member.UUID())
	result := mock.calc()
	eq(t, result.QueryValue(member, FlagBuild), StateAllow)
	isNil(t, result.QueryValue(nonMember, FlagBuild))
}

func TestQueryValueBuildFlagWildernessAndGlobalRegionMembershipAndDeny(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	member := mock.createPlayer()
	global := mock.global()
	global.Members().AddPlayer(member.UUID())
	global.SetFlag(FlagBuild, StateDeny)
	result := mock.calc()
	eq(t, result.QueryValue(member, FlagBuild), StateDeny)
	eq(t, result.QueryValue(nonMember, FlagBuild), StateDeny)
}

func TestQueryValueBuildFlagWildernessAndGlobalRegionMembershipAndAllow(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	member := mock.createPlayer()
	global := mock.global()
	global.Members().AddPlayer(member.UUID())
	global.SetFlag(FlagBuild, StateAllow)
	result := mock.calc()
	eq(t, result.QueryValue(member, FlagBuild), StateAllow)
	isNil(t, result.QueryValue(nonMember, FlagBuild))
}

func TestQueryValueBuildFlagRegion(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	member := mock.createPlayer()
	region := mock.add(0)
	region.Members().AddPlayer(member.UUID())
	result := mock.calc()
	isNil(t, result.QueryValue(nonMember, FlagBuild))
	eq(t, result.QueryValue(member, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagRegionsOverlapping(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberBoth := mock.createPlayer()
	region := mock.add(0)
	region.Members().AddPlayer(memberOne.UUID())
	region.Members().AddPlayer(memberBoth.UUID())
	region = mock.add(0)
	region.Members().AddPlayer(memberBoth.UUID())
	result := mock.calc()
	isNil(t, result.QueryValue(nonMember, FlagBuild))
	isNil(t, result.QueryValue(memberOne, FlagBuild))
	eq(t, result.QueryValue(memberBoth, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagRegionsOverlappingDifferingPriority(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberBoth := mock.createPlayer()
	region := mock.add(0)
	region.Members().AddPlayer(memberOne.UUID())
	region.Members().AddPlayer(memberBoth.UUID())
	region.SetPriority(10)
	region = mock.add(0)
	region.Members().AddPlayer(memberBoth.UUID())
	result := mock.calc()
	isNil(t, result.QueryValue(nonMember, FlagBuild))
	eq(t, result.QueryValue(memberOne, FlagBuild), StateAllow)
	eq(t, result.QueryValue(memberBoth, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagRegionsOverlappingInheritanceFromParent(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberBoth := mock.createPlayer()
	parent := mock.add(0)
	parent.Members().AddPlayer(memberOne.UUID())
	parent.Members().AddPlayer(memberBoth.UUID())
	region := mock.add(0)
	region.Members().AddPlayer(memberBoth.UUID())
	must(t, region.SetParent(parent))
	result := mock.calc()
	isNil(t, result.QueryValue(nonMember, FlagBuild))
	eq(t, result.QueryValue(memberOne, FlagBuild), StateAllow)
	eq(t, result.QueryValue(memberBoth, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagRegionsOverlappingInheritanceFromChild(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberBoth := mock.createPlayer()
	parent := mock.add(0)
	parent.Members().AddPlayer(memberBoth.UUID())
	region := mock.add(0)
	region.Members().AddPlayer(memberBoth.UUID())
	region.Members().AddPlayer(memberOne.UUID())
	must(t, region.SetParent(parent))
	result := mock.calc()
	isNil(t, result.QueryValue(nonMember, FlagBuild))
	eq(t, result.QueryValue(memberOne, FlagBuild), StateAllow)
	eq(t, result.QueryValue(memberBoth, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagRegionsOverlappingInheritanceFromChildAndPriority(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberBoth := mock.createPlayer()
	parent := mock.add(0)
	parent.Members().AddPlayer(memberBoth.UUID())
	region := mock.add(0)
	region.Members().AddPlayer(memberBoth.UUID())
	region.Members().AddPlayer(memberOne.UUID())
	must(t, region.SetParent(parent))
	priority := mock.add(10)
	_ = priority
	result := mock.calc()
	isNil(t, result.QueryValue(nonMember, FlagBuild))
	isNil(t, result.QueryValue(memberOne, FlagBuild))
	isNil(t, result.QueryValue(memberBoth, FlagBuild))
}

func TestQueryValueBuildFlagRegionsOverlappingInheritanceFromChildAndPriorityPassthrough(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberBoth := mock.createPlayer()
	parent := mock.add(0)
	parent.Members().AddPlayer(memberBoth.UUID())
	region := mock.add(0)
	region.Members().AddPlayer(memberBoth.UUID())
	region.Members().AddPlayer(memberOne.UUID())
	must(t, region.SetParent(parent))
	priority := mock.add(10)
	priority.SetFlag(FlagPassthrough, StateAllow)
	result := mock.calc()
	isNil(t, result.QueryValue(nonMember, FlagBuild))
	eq(t, result.QueryValue(memberOne, FlagBuild), StateAllow)
	eq(t, result.QueryValue(memberBoth, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagRegionsOverlappingAndGlobalRegion(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberBoth := mock.createPlayer()
	global := mock.global()
	_ = global
	region := mock.add(0)
	region.Members().AddPlayer(memberOne.UUID())
	region.Members().AddPlayer(memberBoth.UUID())
	region = mock.add(0)
	region.Members().AddPlayer(memberBoth.UUID())
	result := mock.calc()
	isNil(t, result.QueryValue(nonMember, FlagBuild))
	isNil(t, result.QueryValue(memberOne, FlagBuild))
	eq(t, result.QueryValue(memberBoth, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagRegionsOverlappingAndGlobalRegionDenyRegionOverride(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberBoth := mock.createPlayer()
	global := mock.global()
	global.SetFlag(FlagBuild, StateDeny)
	region := mock.add(0)
	region.Members().AddPlayer(memberOne.UUID())
	region.Members().AddPlayer(memberBoth.UUID())
	region = mock.add(0)
	region.Members().AddPlayer(memberBoth.UUID())
	region.SetFlag(FlagBuild, StateAllow)
	result := mock.calc()
	eq(t, result.QueryValue(nonMember, FlagBuild), StateAllow)
	eq(t, result.QueryValue(memberOne, FlagBuild), StateAllow)
	eq(t, result.QueryValue(memberBoth, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagRegionsOverlappingAndGlobalRegionDenyRegionOverrideDenyAndAllow(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberBoth := mock.createPlayer()
	global := mock.global()
	global.SetFlag(FlagBuild, StateDeny)
	region := mock.add(0)
	region.Members().AddPlayer(memberOne.UUID())
	region.Members().AddPlayer(memberBoth.UUID())
	region.SetFlag(FlagBuild, StateDeny)
	region = mock.add(0)
	region.Members().AddPlayer(memberBoth.UUID())
	region.SetFlag(FlagBuild, StateAllow)
	result := mock.calc()
	eq(t, result.QueryValue(nonMember, FlagBuild), StateDeny)
	eq(t, result.QueryValue(memberOne, FlagBuild), StateDeny)
	eq(t, result.QueryValue(memberBoth, FlagBuild), StateDeny)
}

func TestQueryValueBuildFlagRegionsOverlappingAndGlobalRegionAllow(t *testing.T) {
	mock := newMock()
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberBoth := mock.createPlayer()
	global := mock.global()
	global.SetFlag(FlagBuild, StateAllow)
	region := mock.add(0)
	region.Members().AddPlayer(memberOne.UUID())
	region.Members().AddPlayer(memberBoth.UUID())
	region = mock.add(0)
	region.Members().AddPlayer(memberBoth.UUID())
	result := mock.calc()
	isNil(t, result.QueryValue(nonMember, FlagBuild))
	isNil(t, result.QueryValue(memberOne, FlagBuild))
	eq(t, result.QueryValue(memberBoth, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagRegionsOverlappingAndGlobalRegionMembership(t *testing.T) {
	mock := newMock()
	globalMember := mock.createPlayer()
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberBoth := mock.createPlayer()
	global := mock.global()
	global.Members().AddPlayer(globalMember.UUID())
	region := mock.add(0)
	region.Members().AddPlayer(memberOne.UUID())
	region.Members().AddPlayer(memberBoth.UUID())
	region = mock.add(0)
	region.Members().AddPlayer(memberBoth.UUID())
	result := mock.calc()
	isNil(t, result.QueryValue(globalMember, FlagBuild))
	isNil(t, result.QueryValue(nonMember, FlagBuild))
	isNil(t, result.QueryValue(memberOne, FlagBuild))
	eq(t, result.QueryValue(memberBoth, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagRegionsOverlappingAndGlobalRegionMembershipAndGlobalDeny(t *testing.T) {
	mock := newMock()
	globalMember := mock.createPlayer()
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberBoth := mock.createPlayer()
	global := mock.global()
	global.Members().AddPlayer(globalMember.UUID())
	global.SetFlag(FlagBuild, StateDeny)
	region := mock.add(0)
	region.Members().AddPlayer(memberOne.UUID())
	region.Members().AddPlayer(memberBoth.UUID())
	region = mock.add(0)
	region.Members().AddPlayer(memberBoth.UUID())
	result := mock.calc()
	isNil(t, result.QueryValue(globalMember, FlagBuild))
	isNil(t, result.QueryValue(nonMember, FlagBuild))
	isNil(t, result.QueryValue(memberOne, FlagBuild))
	eq(t, result.QueryValue(memberBoth, FlagBuild), StateAllow)
}

func TestQueryValueBuildFlagRegionsOverlappingAndGlobalRegionMembershipAndGlobalAllow(t *testing.T) {
	mock := newMock()
	globalMember := mock.createPlayer()
	nonMember := mock.createPlayer()
	memberOne := mock.createPlayer()
	memberBoth := mock.createPlayer()
	global := mock.global()
	global.Members().AddPlayer(globalMember.UUID())
	global.SetFlag(FlagBuild, StateAllow)
	region := mock.add(0)
	region.Members().AddPlayer(memberOne.UUID())
	region.Members().AddPlayer(memberBoth.UUID())
	region = mock.add(0)
	region.Members().AddPlayer(memberBoth.UUID())
	result := mock.calc()
	isNil(t, result.QueryValue(globalMember, FlagBuild))
	isNil(t, result.QueryValue(nonMember, FlagBuild))
	isNil(t, result.QueryValue(memberOne, FlagBuild))
	eq(t, result.QueryValue(memberBoth, FlagBuild), StateAllow)
}

func TestQueryAllValuesTwoWithSamePriority(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region = mock.add(0)
	region.SetFlag(stringFlag1, "test2")
	result := mock.calc()
	eq(t, setOf(result.QueryAllValues(nil, stringFlag1)...), setOf("test1", "test2"))
	isTrue(t, len(result.QueryAllValues(nil, stringFlag2)) == 0)
}

func TestQueryAllValuesTwoWithDuplicateFlagValues(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test")
	region = mock.add(0)
	region.SetFlag(stringFlag1, "test")
	result := mock.calc()
	eq(t, setOf(result.QueryAllValues(nil, stringFlag1)...), setOf("test"))
	isTrue(t, len(result.QueryAllValues(nil, stringFlag2)) == 0)
}

func TestQueryAllValuesWithHigherPriority(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	region := mock.add(10)
	region.SetFlag(stringFlag1, "test1")
	region = mock.add(0)
	region.SetFlag(stringFlag1, "test2")
	result := mock.calc()
	eq(t, setOf(result.QueryAllValues(nil, stringFlag1)...), setOf("test1"))
	isTrue(t, len(result.QueryAllValues(nil, stringFlag2)) == 0)
}

func TestQueryAllValuesWithTwoElevatedPriorities(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	region := mock.add(10)
	region.SetFlag(stringFlag1, "test3")
	region = mock.add(10)
	region.SetFlag(stringFlag1, "test1")
	region = mock.add(0)
	region.SetFlag(stringFlag1, "test2")
	result := mock.calc()
	eq(t, setOf(result.QueryAllValues(nil, stringFlag1)...), setOf("test1", "test3"))
	isTrue(t, len(result.QueryAllValues(nil, stringFlag2)) == 0)
}

func TestQueryAllValuesParentChildWithSamePriority(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	parent1 := mock.add(10)
	parent1.SetFlag(stringFlag1, "test3")
	region := mock.add(10)
	region.SetFlag(stringFlag1, "test1")
	must(t, region.SetParent(parent1))
	region = mock.add(0)
	region.SetFlag(stringFlag1, "test2")
	result := mock.calc()
	eq(t, setOf(result.QueryAllValues(nil, stringFlag1)...), setOf("test1"))
	isTrue(t, len(result.QueryAllValues(nil, stringFlag2)) == 0)
}

func TestQueryAllValuesParentWithHigherPriority(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	parent1 := mock.add(20)
	parent1.SetFlag(stringFlag1, "test3")
	region := mock.add(10)
	region.SetFlag(stringFlag1, "test1")
	must(t, region.SetParent(parent1))
	region = mock.add(0)
	region.SetFlag(stringFlag1, "test2")
	result := mock.calc()
	eq(t, setOf(result.QueryAllValues(nil, stringFlag1)...), setOf("test3"))
	isTrue(t, len(result.QueryAllValues(nil, stringFlag2)) == 0)
}

func TestQueryAllValuesParentWithLowerPriority(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	parent1 := mock.add(5)
	parent1.SetFlag(stringFlag1, "test3")
	region := mock.add(10)
	region.SetFlag(stringFlag1, "test1")
	must(t, region.SetParent(parent1))
	region = mock.add(0)
	region.SetFlag(stringFlag1, "test2")
	result := mock.calc()
	eq(t, setOf(result.QueryAllValues(nil, stringFlag1)...), setOf("test1"))
	isTrue(t, len(result.QueryAllValues(nil, stringFlag2)) == 0)
}

func TestQueryAllValuesThirdRegionWithHigherPriorityThanParentChild(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	parent1 := mock.add(5)
	parent1.SetFlag(stringFlag1, "test3")
	region := mock.add(10)
	region.SetFlag(stringFlag1, "test1")
	must(t, region.SetParent(parent1))
	region = mock.add(20)
	region.SetFlag(stringFlag1, "test2")
	result := mock.calc()
	eq(t, setOf(result.QueryAllValues(nil, stringFlag1)...), setOf("test2"))
	isTrue(t, len(result.QueryAllValues(nil, stringFlag2)) == 0)
}

func TestQueryAllValuesParentsAndInheritance(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	parent1 := mock.add(5)
	parent1.SetFlag(stringFlag1, "test1")
	region := mock.add(20)
	region.SetFlag(stringFlag1, "test2")
	must(t, region.SetParent(parent1))
	parent2 := mock.add(6)
	parent2.SetFlag(stringFlag1, "test3")
	region = mock.add(20)
	must(t, region.SetParent(parent2))
	result := mock.calc()
	eq(t, setOf(result.QueryAllValues(nil, stringFlag1)...), setOf("test2", "test3"))
	isTrue(t, len(result.QueryAllValues(nil, stringFlag2)) == 0)
}

func TestQueryAllValuesParentsAndInheritanceHighPriorityAndNoFlag(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	flag1 := NewStateFlag("test1", false)
	_ = flag1
	parent1 := mock.add(5)
	parent1.SetFlag(stringFlag1, "test1")
	region := mock.add(20)
	region.SetFlag(stringFlag1, "test2")
	must(t, region.SetParent(parent1))
	parent2 := mock.add(6)
	parent2.SetFlag(stringFlag1, "test3")
	region = mock.add(30)
	result := mock.calc()
	eq(t, setOf(result.QueryAllValues(nil, stringFlag1)...), setOf("test2"))
	isTrue(t, len(result.QueryAllValues(nil, stringFlag2)) == 0)
}

func TestQueryAllValuesParentWithSamePriorityAsHighest(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	flag1 := NewStateFlag("test1", false)
	_ = flag1
	parent1 := mock.add(30)
	parent1.SetFlag(stringFlag1, "test1")
	region := mock.add(20)
	region.SetFlag(stringFlag1, "test2")
	must(t, region.SetParent(parent1))
	parent2 := mock.add(6)
	parent2.SetFlag(stringFlag1, "test3")
	region = mock.add(30)
	result := mock.calc()
	eq(t, setOf(result.QueryAllValues(nil, stringFlag1)...), setOf("test1"))
	isTrue(t, len(result.QueryAllValues(nil, stringFlag2)) == 0)
}

func TestGetEffectivePriority(t *testing.T) {
	mock := newMock()
	region := mock.add(30)
	result := mock.calc()
	_ = result
	eq(t, priorityOf(region), 30)
}

func TestGetEffectivePriorityGlobalRegion(t *testing.T) {
	mock := newMock()
	region := mock.global()
	result := mock.calc()
	_ = result
	eq(t, priorityOf(region), math.MinInt)
}

func TestGetEffectiveFlagSingleRegion(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupAll)
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, nil), "test1")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagWithALLGroupAndNonMember(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupAll)
	player := mock.createPlayer()
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, player), "test1")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagWithALLGroupAndNull(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupAll)
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, nil), "test1")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagNONMEMBERSGroupNonMember(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupNonMembers)
	player := mock.createPlayer()
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, player), "test1")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagNONMEMBERSGroupNull(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupNonMembers)
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, nil), "test1")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagNONOWNERSGroupNonMember(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupNonOwners)
	player := mock.createPlayer()
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, player), "test1")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagMEMBERSGroupNonMember(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupMembers)
	player := mock.createPlayer()
	result := mock.calc()
	_ = result
	isNil(t, EffectiveFlag(region, stringFlag1, player))
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagMEMBERSGroupNull(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupMembers)
	result := mock.calc()
	_ = result
	isNil(t, EffectiveFlag(region, stringFlag1, nil))
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagMEMBERSGroupMember(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupMembers)
	player := mock.createPlayer()
	region.Members().AddPlayer(player.UUID())
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, player), "test1")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagMEMBERSGroupOwner(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupMembers)
	player := mock.createPlayer()
	region.Owners().AddPlayer(player.UUID())
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, player), "test1")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagOWNERSGroupOwner(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupOwners)
	player := mock.createPlayer()
	region.Owners().AddPlayer(player.UUID())
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, player), "test1")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagOWNERSGroupMember(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupOwners)
	player := mock.createPlayer()
	region.Members().AddPlayer(player.UUID())
	result := mock.calc()
	_ = result
	isNil(t, EffectiveFlag(region, stringFlag1, player))
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagNONOWNERSGroupOwner(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupNonOwners)
	player := mock.createPlayer()
	region.Owners().AddPlayer(player.UUID())
	result := mock.calc()
	_ = result
	isNil(t, EffectiveFlag(region, stringFlag1, player))
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagNONMEMBERSGroupOwner(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupNonMembers)
	player := mock.createPlayer()
	region.Owners().AddPlayer(player.UUID())
	result := mock.calc()
	_ = result
	isNil(t, EffectiveFlag(region, stringFlag1, player))
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagNONOWNERSGroupMember(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupNonOwners)
	player := mock.createPlayer()
	region.Members().AddPlayer(player.UUID())
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, player), "test1")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagNONOWNERSNonMember(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	region := mock.add(0)
	region.SetFlag(stringFlag1, "test1")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupNonOwners)
	player := mock.createPlayer()
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, player), "test1")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagThreeInheritance(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	parent1 := mock.add(0)
	parent1.SetFlag(stringFlag1, "test1")
	parent1.SetFlag(stringFlag1.GroupFlag(), RegionGroupAll)
	parent2 := mock.add(0)
	must(t, parent2.SetParent(parent1))
	region := mock.add(0)
	must(t, region.SetParent(parent2))
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, nil), "test1")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagThreeInheritanceMiddleOverride(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	parent1 := mock.add(0)
	parent1.SetFlag(stringFlag1, "test1")
	parent1.SetFlag(stringFlag1.GroupFlag(), RegionGroupAll)
	parent2 := mock.add(0)
	must(t, parent2.SetParent(parent1))
	parent2.SetFlag(stringFlag1, "test2")
	parent1.SetFlag(stringFlag1.GroupFlag(), RegionGroupAll)
	region := mock.add(0)
	must(t, region.SetParent(parent2))
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, nil), "test2")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagThreeInheritanceLastOverride(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	parent1 := mock.add(0)
	parent1.SetFlag(stringFlag1, "test1")
	parent1.SetFlag(stringFlag1.GroupFlag(), RegionGroupAll)
	parent2 := mock.add(0)
	must(t, parent2.SetParent(parent1))
	region := mock.add(0)
	must(t, region.SetParent(parent2))
	region.SetFlag(stringFlag1, "test3")
	parent1.SetFlag(stringFlag1.GroupFlag(), RegionGroupAll)
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, nil), "test3")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagInheritanceAndDifferingGroups(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	parent1 := mock.add(0)
	parent1.SetFlag(stringFlag1, "everyone")
	parent1.SetFlag(stringFlag1.GroupFlag(), RegionGroupAll)
	parent2 := mock.add(0)
	parent2.SetFlag(stringFlag1, "members")
	parent2.SetFlag(stringFlag1.GroupFlag(), RegionGroupMembers)
	must(t, parent2.SetParent(parent1))
	region := mock.add(0)
	must(t, region.SetParent(parent2))
	player1 := mock.createPlayer()
	parent2.Members().AddPlayer(player1.UUID())
	player2 := mock.createPlayer()
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, player1), "members")
	eq(t, EffectiveFlag(region, stringFlag1, player2), "everyone")
	eq(t, EffectiveFlag(region, stringFlag1, nil), "everyone")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagInheritanceAndDifferingGroupsMemberOnChild(t *testing.T) {
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	mock := newMock()
	parent1 := mock.add(0)
	parent1.SetFlag(stringFlag1, "everyone")
	parent1.SetFlag(stringFlag1.GroupFlag(), RegionGroupAll)
	parent2 := mock.add(0)
	parent2.SetFlag(stringFlag1, "members")
	parent2.SetFlag(stringFlag1.GroupFlag(), RegionGroupMembers)
	must(t, parent2.SetParent(parent1))
	region := mock.add(0)
	must(t, region.SetParent(parent2))
	player1 := mock.createPlayer()
	region.Members().AddPlayer(player1.UUID())
	player2 := mock.createPlayer()
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, player1), "members")
	eq(t, EffectiveFlag(region, stringFlag1, player2), "everyone")
	eq(t, EffectiveFlag(region, stringFlag1, nil), "everyone")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagInheritanceAndDifferingGroupsMemberOnParent(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	parent1 := mock.add(0)
	parent1.SetFlag(stringFlag1, "everyone")
	parent1.SetFlag(stringFlag1.GroupFlag(), RegionGroupAll)
	parent2 := mock.add(0)
	parent2.SetFlag(stringFlag1, "members")
	parent2.SetFlag(stringFlag1.GroupFlag(), RegionGroupMembers)
	must(t, parent2.SetParent(parent1))
	region := mock.add(0)
	must(t, region.SetParent(parent2))
	player1 := mock.createPlayer()
	parent1.Members().AddPlayer(player1.UUID())
	player2 := mock.createPlayer()
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, player1), "members")
	eq(t, EffectiveFlag(region, stringFlag1, player2), "everyone")
	eq(t, EffectiveFlag(region, stringFlag1, nil), "everyone")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagInheritanceAndDifferingGroupsMemberOnParentFlagOnBottom(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	parent1 := mock.add(0)
	parent1.SetFlag(stringFlag1, "everyone")
	parent1.SetFlag(stringFlag1.GroupFlag(), RegionGroupAll)
	parent2 := mock.add(0)
	must(t, parent2.SetParent(parent1))
	region := mock.add(0)
	region.SetFlag(stringFlag1, "members")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupMembers)
	must(t, region.SetParent(parent2))
	player1 := mock.createPlayer()
	parent1.Members().AddPlayer(player1.UUID())
	player2 := mock.createPlayer()
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, player1), "members")
	eq(t, EffectiveFlag(region, stringFlag1, player2), "everyone")
	eq(t, EffectiveFlag(region, stringFlag1, nil), "everyone")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagInheritanceAndDifferingGroupsMemberOnParentFlagOnBottomGroupOutside(t *testing.T) {
	mock := newMock()
	stringFlag1 := NewStringFlag("string1", "")
	stringFlag2 := NewStringFlag("string2", "")
	parent1 := mock.createOutside(0)
	parent1.SetFlag(stringFlag1, "everyone")
	parent1.SetFlag(stringFlag1.GroupFlag(), RegionGroupAll)
	parent2 := mock.add(0)
	must(t, parent2.SetParent(parent1))
	region := mock.add(0)
	region.SetFlag(stringFlag1, "members")
	region.SetFlag(stringFlag1.GroupFlag(), RegionGroupMembers)
	must(t, region.SetParent(parent2))
	player1 := mock.createPlayer()
	parent1.Members().AddPlayer(player1.UUID())
	player2 := mock.createPlayer()
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(region, stringFlag1, player1), "members")
	eq(t, EffectiveFlag(region, stringFlag1, player2), "everyone")
	eq(t, EffectiveFlag(region, stringFlag1, nil), "everyone")
	isNil(t, EffectiveFlag(region, stringFlag2, nil))
}

func TestGetEffectiveFlagGlobalRegionBuild(t *testing.T) {
	mock := newMock()
	global := mock.global()
	result := mock.calc()
	_ = result
	isNil(t, EffectiveFlag(global, FlagBuild, nil))
}

func TestGetEffectiveFlagGlobalRegionBuildDeny(t *testing.T) {
	mock := newMock()
	global := mock.global()
	global.SetFlag(FlagBuild, StateDeny)
	result := mock.calc()
	_ = result
	eq(t, EffectiveFlag(global, FlagBuild, nil), StateDeny)
}

func TestGetEffectiveFlagGlobalRegionBuildAllow(t *testing.T) {
	mock := newMock()
	global := mock.global()
	global.SetFlag(FlagBuild, StateAllow)
	result := mock.calc()
	_ = result
	isNil(t, EffectiveFlag(global, FlagBuild, nil))
}
