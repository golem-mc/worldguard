package worldguardplugin

import (
	"regexp"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
)

// permModel decides the region permissions of a sender. WorldGuard:
// RegionPermissionModel
type permModel struct {
	wg     *WorldGuard
	sender plugin.CommandSender
	// player is the sender as a region player; nil for the console, which
	// has every pattern permission.
	player worldguard.LocalPlayer
}

func (wg *WorldGuard) perms(s plugin.CommandSender) *permModel {
	m := &permModel{wg: wg, sender: s}
	if p, ok := s.(plugin.Player); ok {
		m.player = wg.WrapPlayer(p)
	}
	return m
}

func (m *permModel) has(perm string) bool { return m.wg.HasPermission(m.sender, "worldguard."+perm) }

// pattern checks region.<perm>.<id>, or .own.<id> and .member.<id> for an
// owner and .member.<id> for a member.
func (m *permModel) pattern(perm string, r *worldguard.Region) bool {
	if m.player == nil {
		return true
	}
	id := strings.ToLower(r.ID())
	switch {
	case r.IsOwner(m.player):
		return m.has("region."+perm+".own."+id) || m.has("region."+perm+".member."+id)
	case r.IsMember(m.player):
		return m.has("region." + perm + ".member." + id)
	}
	return m.has("region." + perm + "." + id)
}

func (m *permModel) mayForceLoadRegions() bool     { return m.has("region.load") }
func (m *permModel) mayForceSaveRegions() bool     { return m.has("region.save") }
func (m *permModel) mayMigrateRegionStore() bool   { return m.has("region.migratedb") }
func (m *permModel) mayMigrateRegionNames() bool   { return m.has("region.migrateuuid") }
func (m *permModel) mayMigrateRegionHeights() bool { return m.has("region.migrateheights") }
func (m *permModel) mayDefine() bool               { return m.has("region.define") }
func (m *permModel) mayClaim() bool                { return m.has("region.claim") }
func (m *permModel) mayClaimUnbounded() bool       { return m.has("region.unlimited") }

func (m *permModel) mayRedefine(r *worldguard.Region) bool { return m.pattern("redefine", r) }
func (m *permModel) mayDelete(r *worldguard.Region) bool   { return m.pattern("remove", r) }
func (m *permModel) maySetPriority(r *worldguard.Region) bool {
	return m.pattern("setpriority", r)
}
func (m *permModel) maySelect(r *worldguard.Region) bool     { return m.pattern("select", r) }
func (m *permModel) mayLookup(r *worldguard.Region) bool     { return m.pattern("info", r) }
func (m *permModel) mayTeleportTo(r *worldguard.Region) bool { return m.pattern("teleport", r) }
func (m *permModel) mayTeleportToCenter(r *worldguard.Region) bool {
	return m.pattern("teleportcenter", r)
}
func (m *permModel) mayOverrideLocationBounds(r *worldguard.Region) bool {
	return m.pattern("locationoverride", r)
}
func (m *permModel) mayAddMembers(r *worldguard.Region) bool { return m.pattern("addmember", r) }
func (m *permModel) mayAddOwners(r *worldguard.Region) bool  { return m.pattern("addowner", r) }
func (m *permModel) mayRemoveMembers(r *worldguard.Region) bool {
	return m.pattern("removemember", r)
}
func (m *permModel) mayRemoveOwners(r *worldguard.Region) bool { return m.pattern("removeowner", r) }

func (m *permModel) maySetParent(child, parent *worldguard.Region) bool {
	return m.pattern("setparent", child) && (parent == nil || m.pattern("setparent", parent))
}

// mayList decides /rg list of target's regions, every region for "".
func (m *permModel) mayList(target string) bool {
	if target != "" && strings.EqualFold(target, m.sender.Name()) {
		return m.has("region.list.own")
	}
	return m.has("region.list")
}

func (m *permModel) maySetFlags(r *worldguard.Region) bool { return m.pattern("flag.regions", r) }

func (m *permModel) maySetFlag(r *worldguard.Region, f worldguard.Flag) bool {
	return m.pattern("flag.flags."+strings.ToLower(f.Name()), r)
}

var notPermChar = regexp.MustCompile(`[^a-z0-9]`)

// maySetFlagValue checks flag.flags.<flag>.<value>, the value lower case
// without other characters than letters and digits, at most 20 long, or
// "unset" for removing it.
func (m *permModel) maySetFlagValue(r *worldguard.Region, f worldguard.Flag, value *string) bool {
	v := "unset"
	if value != nil {
		v = notPermChar.ReplaceAllString(strings.ToLower(strings.TrimSpace(*value)), "")
		if len(v) > 20 {
			v = v[:20]
		}
	}
	return m.pattern("flag.flags."+strings.ToLower(f.Name())+"."+v, r)
}
