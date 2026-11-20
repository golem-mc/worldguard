package worldguardplugin

import (
	"fmt"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
)

// memberTarget picks the owners or the members of a region.
type memberTarget struct {
	word    string // "members" or "owners"
	domain  func(*worldguard.Region) *worldguard.Domain
	mayAdd  func(*permModel, *worldguard.Region) bool
	mayRem  func(*permModel, *worldguard.Region) bool
	addFail string
	remFail string
}

var (
	membersTarget = memberTarget{word: "members", domain: (*worldguard.Region).Members,
		mayAdd: (*permModel).mayAddMembers, mayRem: (*permModel).mayRemoveMembers,
		addFail: "Failed to add new members", remFail: "Failed to remove members"}
	ownersTarget = memberTarget{word: "owners", domain: (*worldguard.Region).Owners,
		mayAdd: (*permModel).mayAddOwners, mayRem: (*permModel).mayRemoveOwners,
		addFail: "Failed to add new owners", remFail: "Failed to remove owners"}
)

func (wg *WorldGuard) cmdAddMember(c *cmdArgs) error { return wg.addTo(c, membersTarget) }
func (wg *WorldGuard) cmdAddOwner(c *cmdArgs) error  { return wg.addTo(c, ownersTarget) }

// addTo runs /rg addmember and /rg addowner. MemberCommands.addMember,
// addOwner
func (wg *WorldGuard) addTo(c *cmdArgs, t memberTarget) error {
	wg.warnAboutSaveFailures(c.sender)
	_, m, err := c.worldRegions()
	if err != nil {
		return err
	}
	r, err := checkExistingRegion(m, c.arg(0), true)
	if err != nil {
		return err
	}
	if !t.mayAdd(wg.perms(c.sender), r) {
		return errNoPermission
	}
	in := wg.resolveLocal(c.args[1:], policyOf(c, uuidOnly))
	async(wg, c.sender, "", t.addFail, in.resolve, func(d *worldguard.Domain) {
		if t.word == "owners" {
			if err := wg.checkAddOwners(c, m, r, d); err != nil {
				printError(c.sender, t.addFail+": "+err.Error())
				return
			}
		}
		t.domain(r).AddAll(d)
		printInfo(c.sender, fmt.Sprintf("Region '%s' updated with new %s.", r.ID(), t.word))
	})
	return nil
}

// checkAddOwners refuses owners that would exceed the sender's region
// limit, and an owner of an unclaimed region without
// worldguard.region.addowner.unclaimed.<id>. MemberCommands.checkedAddOwners
func (wg *WorldGuard) checkAddOwners(c *cmdArgs, m *worldguard.RegionManager, r *worldguard.Region, owners *worldguard.Domain) error {
	if p := wg.perms(c.sender); p.player != nil {
		if owners.Contains(p.player) && !wg.HasPermission(c.sender, "worldguard.region.unlimited") {
			pl, _ := checkPlayer(c.sender)
			most := wg.WorldConfig(pl.World()).MaxRegionCount(wg.Groups(pl))
			if most >= 0 && m.RegionCountOfPlayer(p.player) >= most {
				return errorf("You already own the maximum allowed amount of regions.")
			}
		}
	}
	if r.Owners().Size() == 0 {
		for q := r.Parent(); q != nil; q = q.Parent() {
			if q.Owners().Size() > 0 {
				return nil
			}
		}
		if !wg.HasPermission(c.sender, "worldguard.region.addowner.unclaimed."+strings.ToLower(r.ID())) {
			return errNoPermission
		}
	}
	return nil
}

func (wg *WorldGuard) cmdRemoveMember(c *cmdArgs) error { return wg.removeFrom(c, membersTarget) }
func (wg *WorldGuard) cmdRemoveOwner(c *cmdArgs) error  { return wg.removeFrom(c, ownersTarget) }

// removeFrom runs /rg removemember and /rg removeowner.
// MemberCommands.removeMember, removeOwner
func (wg *WorldGuard) removeFrom(c *cmdArgs, t memberTarget) error {
	wg.warnAboutSaveFailures(c.sender)
	_, m, err := c.worldRegions()
	if err != nil {
		return err
	}
	r, err := checkExistingRegion(m, c.arg(0), true)
	if err != nil {
		return err
	}
	if !t.mayRem(wg.perms(c.sender), r) {
		return errNoPermission
	}
	done := func(d *worldguard.Domain) {
		if d == nil {
			t.domain(r).Clear()
		} else {
			t.domain(r).RemoveAll(d)
		}
		printInfo(c.sender, fmt.Sprintf("Region '%s' updated with %s removed.", r.ID(), t.word))
	}
	if c.has('a') {
		done(nil)
		return nil
	}
	if len(c.args) < 2 {
		return errorf("List some names to remove, or use -a to remove all.")
	}
	in := wg.resolveLocal(c.args[1:], policyOf(c, uuidAndName))
	async(wg, c.sender, "(Please wait... querying player names...)", t.remFail, in.resolve, done)
	return nil
}

// completeRegionIDs completes the ids of the regions in the -w world or
// the sender's.
func completeRegionIDs(wg *WorldGuard, a *cmdArgs, i int) []string {
	if i > 0 {
		return nil
	}
	return wg.regionIDs(a)
}

func (wg *WorldGuard) regionIDs(c *cmdArgs) []string {
	var w plugin.World
	if name, ok := c.value('w'); ok {
		w, _ = wg.matchWorld(c.sender, name)
	} else if p, ok := c.sender.(plugin.Player); ok {
		w = p.World()
	}
	if w == nil {
		return nil
	}
	m, ok := wg.Regions(w)
	if !ok {
		return nil
	}
	var out []string
	for id := range m.Regions() {
		out = append(out, id)
	}
	return out
}

// completeRegionIDsTwice completes a child and a parent id.
func completeRegionIDsTwice(wg *WorldGuard, a *cmdArgs, i int) []string {
	if i > 1 {
		return nil
	}
	return wg.regionIDs(a)
}

// completeIDThenPlayers completes a region id, then player names.
func completeIDThenPlayers(wg *WorldGuard, a *cmdArgs, i int) []string {
	if i == 0 {
		return wg.regionIDs(a)
	}
	return wg.playerNames()
}

func (wg *WorldGuard) playerNames() []string {
	var out []string
	for _, p := range wg.Server.Players() {
		out = append(out, p.Name())
	}
	return out
}

// playersAfter completes player names from argument n on.
func playersAfter(n int) func(*WorldGuard, *cmdArgs, int) []string {
	return func(wg *WorldGuard, _ *cmdArgs, i int) []string {
		if i < n {
			return nil
		}
		return wg.playerNames()
	}
}

// words completes fixed words.
func words(w ...string) func(*WorldGuard, *cmdArgs, int) []string {
	return func(*WorldGuard, *cmdArgs, int) []string { return w }
}

// completeFlag completes /rg flag: a region id, a flag name, then the
// flag's values.
func completeFlag(wg *WorldGuard, c *cmdArgs, i int) []string {
	switch i {
	case 0:
		return wg.regionIDs(c)
	case 1:
		var out []string
		for _, f := range worldguard.Flags.All() {
			out = append(out, f.Name())
		}
		return out
	case 2:
		f, ok := worldguard.Flags.FuzzyMatch(c.args[1])
		if !ok {
			return nil
		}
		return flagValues(f)
	}
	return nil
}

// flagValues answers the values a flag suggests.
func flagValues(f worldguard.Flag) []string {
	switch f := f.(type) {
	case *worldguard.StateFlag:
		return []string{"allow", "deny", "none"}
	case *worldguard.BooleanFlag:
		return []string{"true", "false"}
	case *worldguard.RegistryFlag:
		return f.Registry().Keys()
	case *worldguard.SetFlag:
		return flagValues(f.Sub())
	case *worldguard.LocationFlag:
		return []string{"here"}
	case *worldguard.EnumFlag:
		return f.Values()
	case worldguard.NumberFlag:
		return f.Suggestions()
	}
	return nil
}
