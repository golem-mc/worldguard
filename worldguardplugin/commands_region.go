package worldguardplugin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
	weregion "github.com/golem-mc/worldedit/region"
	"github.com/golem-mc/worldguard"
)

// regionCommands is /region (/regions, /rg). WorldGuard:
// ProtectionCommands.region, RegionCommands, MemberCommands
func regionCommands() *commandGroup {
	ids := completeRegionIDs
	return &commandGroup{aliases: []string{"region", "regions", "rg"}, desc: "Region management commands", subs: []*wgCommand{
		{aliases: []string{"define", "def", "d", "create"}, usage: "[-w <world>] <id> [<owner1> [<owner2> [<owners...>]]]",
			flags: "ngw:", desc: "Defines a region", min: 1, max: -1, run: (*WorldGuard).cmdDefine, complete: playersAfter(1)},
		{aliases: []string{"redefine", "update", "move"}, usage: "[-w <world>] <id>", desc: "Re-defines the shape of a region",
			flags: "gw:", min: 1, max: 1, run: (*WorldGuard).cmdRedefine, complete: ids},
		{aliases: []string{"claim"}, usage: "<id>", desc: "Claim a region", min: 1, max: 1, run: (*WorldGuard).cmdClaim},
		{aliases: []string{"select", "sel", "s"}, usage: "[-w <world>] [id]", desc: "Load a region as a WorldEdit selection",
			flags: "w:", min: 0, max: 1, run: (*WorldGuard).cmdSelect, complete: ids},
		{aliases: []string{"info", "i"}, usage: "[id]", flags: "usw:", desc: "Get information about a region",
			min: 0, max: 1, run: (*WorldGuard).cmdInfo, complete: ids},
		{aliases: []string{"list"}, usage: "[-w world] [-p owner [-n]] [-s] [-i filter] [page]", desc: "Get a list of regions",
			flags: "np:w:i:s", min: 0, max: 1, run: (*WorldGuard).cmdList},
		{aliases: []string{"flag", "f"}, usage: "<id> <flag> [-w world] [-g group] [value]", flags: "g:w:eh:",
			desc: "Set flags", min: 2, max: -1, run: (*WorldGuard).cmdFlag, complete: completeFlag},
		{aliases: []string{"flags"}, usage: "[-p <page>] [id]", flags: "p:w:", desc: "View region flags",
			min: 0, max: 2, run: (*WorldGuard).cmdFlags, complete: ids},
		{aliases: []string{"setpriority", "priority", "pri"}, usage: "<id> <priority>", flags: "w:",
			desc: "Set the priority of a region", min: 2, max: 2, run: (*WorldGuard).cmdSetPriority, complete: ids},
		{aliases: []string{"setparent", "parent", "par"}, usage: "<id> [parent-id]", flags: "w:",
			desc: "Set the parent of a region", min: 1, max: 2, run: (*WorldGuard).cmdSetParent, complete: completeRegionIDsTwice},
		{aliases: []string{"remove", "delete", "del", "rem"}, usage: "<id>", flags: "fuw:", desc: "Remove a region",
			min: 1, max: 1, run: (*WorldGuard).cmdRemove, complete: ids},
		{aliases: []string{"load", "reload"}, usage: "[world]", desc: "Reload regions from file", flags: "w:",
			min: 0, max: -1, run: (*WorldGuard).cmdLoad},
		{aliases: []string{"save", "write"}, usage: "[world]", desc: "Re-save regions to file", flags: "w:",
			min: 0, max: -1, run: (*WorldGuard).cmdSave},
		{aliases: []string{"migratedb"}, usage: "<from> <to>", flags: "y",
			desc: "Migrate from one Protection Database to another.", min: 2, max: 2, run: (*WorldGuard).cmdMigrateDB,
			complete: words("yaml", "mysql")},
		{aliases: []string{"migrateuuid"}, desc: "Migrate loaded databases to use UUIDs", min: 0, max: 0, run: (*WorldGuard).cmdMigrateUUID},
		{aliases: []string{"migrateheights"}, usage: "[world]", max: 1, flags: "yw:",
			desc: "Migrate regions from old height limits to new height limits", run: (*WorldGuard).cmdMigrateHeights},
		{aliases: []string{"teleport", "tp"}, usage: "[-w world] [-c|s] <id>", flags: "csw:",
			desc: "Teleports you to the location associated with the region.", min: 1, max: 1, run: (*WorldGuard).cmdTeleport, complete: ids},
		{aliases: []string{"toggle-bypass", "bypass"}, usage: "[on|off]",
			desc: "Toggle region bypassing, effectively ignoring bypass permissions.", min: 0, max: -1, run: (*WorldGuard).cmdToggleBypass,
			complete: words("on", "off")},
		{aliases: []string{"addmember", "addmem", "am"}, usage: "<id> <members...>", flags: "nw:",
			desc: "Add a member to a region", min: 2, max: -1, run: (*WorldGuard).cmdAddMember, complete: completeIDThenPlayers},
		{aliases: []string{"addowner", "ao"}, usage: "<id> <owners...>", flags: "nw:",
			desc: "Add an owner to a region", min: 2, max: -1, run: (*WorldGuard).cmdAddOwner, complete: completeIDThenPlayers},
		{aliases: []string{"removemember", "remmember", "removemem", "remmem", "rm"}, usage: "<id> <owners...>", flags: "naw:",
			desc: "Remove an owner to a region", min: 1, max: -1, run: (*WorldGuard).cmdRemoveMember, complete: completeIDThenPlayers},
		{aliases: []string{"removeowner", "remowner", "ro"}, usage: "<id> <owners...>", flags: "naw:",
			desc: "Remove an owner to a region", min: 1, max: -1, run: (*WorldGuard).cmdRemoveOwner, complete: completeIDThenPlayers},
	}}
}

func (wg *WorldGuard) cmdDefine(c *cmdArgs) error {
	wg.warnAboutSaveFailures(c.sender)
	if !wg.perms(c.sender).mayDefine() {
		return errNoPermission
	}
	id, err := checkRegionID(c.arg(0), false)
	if err != nil {
		return err
	}
	w, m, err := c.worldRegions()
	if err != nil {
		return err
	}
	if err := checkRegionDoesNotExist(m, id, true); err != nil {
		return err
	}
	r, err := c.newRegion(id)
	if err != nil {
		return err
	}
	var owners *domainInput
	if len(c.args) >= 2 {
		owners = wg.resolveLocal(c.args[1:], policyOf(c, uuidOnly))
	}
	async(wg, c.sender, "", fmt.Sprintf("Failed to add the region '%s'", r.ID()), func() (*worldguard.Region, error) {
		if owners != nil {
			d, err := owners.resolve()
			if err != nil {
				return nil, err
			}
			r.Owners().AddAll(d)
		}
		m.AddRegion(r)
		return r, nil
	}, func(r *worldguard.Region) {
		printInfo(c.sender, fmt.Sprintf("A new region has been made named '%s'.", r.ID()))
		warnAboutDimensions(c.sender, r)
		informNewUser(c.sender, m, r)
		wg.checkSpawnOverlap(c.sender, w, r)
	})
	return nil
}

// policyOf answers name-only with -n, else def.
func policyOf(c *cmdArgs, def locatorPolicy) locatorPolicy {
	if c.has('n') {
		return nameOnly
	}
	return def
}

// worldRegions answers the -w world and its regions.
func (c *cmdArgs) worldRegions() (plugin.World, *worldguard.RegionManager, error) {
	w, err := c.checkWorld('w')
	if err != nil {
		return nil, nil, err
	}
	m, err := c.wg.checkRegionManager(w)
	return w, m, err
}

// newRegion makes a global region with -g, else one of the selection.
func (c *cmdArgs) newRegion(id string) (*worldguard.Region, error) {
	if c.has('g') {
		return worldguard.NewGlobalRegion(id)
	}
	return c.wg.regionFromSelection(c.sender, id)
}

func (wg *WorldGuard) cmdRedefine(c *cmdArgs) error {
	wg.warnAboutSaveFailures(c.sender)
	id, err := checkRegionID(c.arg(0), false)
	if err != nil {
		return err
	}
	w, m, err := c.worldRegions()
	if err != nil {
		return err
	}
	existing, err := checkExistingRegion(m, id, false)
	if err != nil {
		return err
	}
	if !wg.perms(c.sender).mayRedefine(existing) {
		return errNoPermission
	}
	r, err := c.newRegion(id)
	if err != nil {
		return err
	}
	r.CopyFrom(existing)
	desc := fmt.Sprintf("Updating region '%s'", r.ID())
	async(wg, c.sender, "(Please wait... "+desc+")", fmt.Sprintf("Failed to update the region '%s'", r.ID()),
		func() (*worldguard.Region, error) { m.AddRegion(r); return r, nil },
		func(r *worldguard.Region) {
			printInfo(c.sender, fmt.Sprintf("Region '%s' has been updated with a new area.", r.ID()))
			warnAboutDimensions(c.sender, r)
			informNewUser(c.sender, m, r)
			wg.checkSpawnOverlap(c.sender, w, r)
		})
	return nil
}

func (wg *WorldGuard) cmdClaim(c *cmdArgs) error {
	wg.warnAboutSaveFailures(c.sender)
	p, err := checkPlayer(c.sender)
	if err != nil {
		return err
	}
	perms := wg.perms(p)
	if !perms.mayClaim() {
		return errNoPermission
	}
	id, err := checkRegionID(c.arg(0), false)
	if err != nil {
		return err
	}
	m, err := wg.checkRegionManager(p.World())
	if err != nil {
		return err
	}
	if err := checkRegionDoesNotExist(m, id, false); err != nil {
		return err
	}
	r, err := wg.regionFromSelection(p, id)
	if err != nil {
		return err
	}
	cfg := wg.WorldConfig(p.World())
	lp := wg.WrapPlayer(p)
	if !perms.mayClaimUnbounded() {
		if most := cfg.MaxRegionCount(wg.Groups(p)); most >= 0 && m.RegionCountOfPlayer(lp) >= most {
			return errorf("You own too many regions, delete one first to claim a new one.")
		}
	}
	if existing, ok := m.Region(id); ok && !existing.Owners().Contains(lp) {
		return errorf("This region already exists and you don't own it.")
	}
	set := m.ApplicableRegionsFor(r, worldguard.ComputeParents)
	if set.Len() > 0 {
		if !set.IsOwnerOfAll(lp) {
			return errorf("This region overlaps with someone else's region.")
		}
	} else if cfg.ClaimOnlyInsideExisting {
		return errorf("You may only claim regions inside existing regions that you or your group own.")
	}
	if cfg.MaxClaimVolume >= math.MaxInt32 {
		return errorf("The maximum claim volume get in the configuration is higher than is supported. "+
			"Currently, it must be %d or smaller. Please contact a server administrator.", math.MaxInt32)
	}
	if !perms.mayClaimUnbounded() {
		if r.Type() == worldguard.RegionPolygon {
			return errorf("Polygons are currently not supported for /rg claim.")
		}
		if r.Volume() > cfg.MaxClaimVolume {
			printError(p, "This region is too large to claim.")
			printError(p, fmt.Sprintf("Max. volume: %d, your volume: %d", cfg.MaxClaimVolume, r.Volume()))
			return nil
		}
	}
	if cfg.SetParentOnClaim != "" {
		if template, ok := m.Region(cfg.SetParentOnClaim); ok {
			if err := r.SetParent(template); err != nil {
				return errorf("%s", err.Error())
			}
		}
	}
	r.Owners().AddPlayer(p.UUID())
	m.AddRegion(r)
	p.SendMessage(plugin.Plain(fmt.Sprintf("A new region has been claimed named '%s'.", id)))
	return nil
}

func (wg *WorldGuard) cmdSelect(c *cmdArgs) error {
	w, m, err := c.worldRegions()
	if err != nil {
		return err
	}
	perms := wg.perms(c.sender)
	var existing *worldguard.Region
	if len(c.args) == 0 {
		p, err := checkPlayer(c.sender)
		if err != nil {
			return err
		}
		if p.World() != w && p.World().Key() != w.Key() {
			return errorf("Please specify a region name.")
		}
		existing, err = c.checkRegionStandingIn(m, p, false,
			fmt.Sprintf("/rg select -w \"%s\" %%id%%", wg.WorldName(w)), perms.maySelect)
		if err != nil {
			return err
		}
	} else if existing, err = checkExistingRegion(m, c.arg(0), false); err != nil {
		return err
	}
	if !perms.maySelect(existing) {
		return errNoPermission
	}
	return wg.setPlayerSelection(c.sender, existing, w)
}

func (wg *WorldGuard) cmdInfo(c *cmdArgs) error {
	wg.warnAboutSaveFailures(c.sender)
	w, m, err := c.worldRegions()
	if err != nil {
		return err
	}
	perms := wg.perms(c.sender)
	var existing *worldguard.Region
	if len(c.args) == 0 {
		p, ok := c.sender.(plugin.Player)
		if !ok {
			return errorf("Please specify the region with /region info -w world_name region_name.")
		}
		cmd := fmt.Sprintf("/rg info -w \"%s\" %%id%%", wg.WorldName(w))
		if c.has('u') {
			cmd += " -u"
		}
		if c.has('s') {
			cmd += " -s"
		}
		if existing, err = c.checkRegionStandingIn(m, p, true, cmd, perms.mayLookup); err != nil {
			return err
		}
	} else if existing, err = checkExistingRegion(m, c.arg(0), true); err != nil {
		return err
	}
	if !perms.mayLookup(existing) {
		return errNoPermission
	}
	if c.has('s') {
		if !perms.maySelect(existing) {
			return errNoPermission
		}
		if err := wg.setPlayerSelection(c.sender, existing, w); err != nil {
			return err
		}
	}
	c.sender.SendMessage(wg.newPrintout(wg.WorldName(w), existing, !c.has('u'), c.sender).info())
	wg.checkSpawnOverlap(c.sender, w, existing)
	return nil
}

func (wg *WorldGuard) cmdList(c *cmdArgs) error {
	wg.warnAboutSaveFailures(c.sender)
	w, err := c.checkWorld('w')
	if err != nil {
		return err
	}
	page := 1
	if len(c.args) > 0 {
		if page, err = parseInt(c.arg(0)); err != nil {
			return err
		}
		page = max(page, 1)
	}
	owner, _ := c.value('p')
	perms := wg.perms(c.sender)
	if !perms.mayList(owner) {
		owner = c.sender.Name()
		if !perms.mayList(owner) {
			return errNoPermission
		}
	}
	m, err := wg.checkRegionManager(w)
	if err != nil {
		return err
	}
	l := &regionLister{wg: wg, sender: c.sender, m: m, world: wg.WorldName(w), page: page, owner: owner, nameOnly: c.has('n')}
	if c.has('s') {
		if l.intersecting, err = wg.regionFromSelection(c.sender, "tmp"); err != nil {
			return err
		}
	}
	if f, ok := c.value('i'); ok {
		l.idFilter = &f
	}
	var lookup *domainInput
	if owner != "" && !l.nameOnly {
		lookup = wg.resolveLocal([]string{owner}, uuidOnly)
	}
	async(wg, c.sender, "(Please wait... fetching region list...)", "Failed to fetch region list",
		func() (*regionLister, error) {
			if lookup != nil {
				d, err := lookup.resolve()
				if err != nil {
					return nil, errorf("A user by the name of '%s' does not seem to exist.", owner)
				}
				if ids := d.UUIDs(); len(ids) > 0 {
					l.ownerID = &ids[0]
				}
			}
			return l, nil
		}, func(l *regionLister) {
			t, err := l.text()
			if err != nil {
				printError(c.sender, err.Error())
				return
			}
			c.sender.SendMessage(t)
		})
	return nil
}

// passthroughWarning and buildWarning are shown when the flags that
// override protection are set. WorldGuard: RegionCommands
func passthroughWarning() plugin.Text {
	return line(txt("WARNING:", "red").Bold(), txt(" This flag is unrelated to moving through regions.", "red"),
		plugin.Plain("\nIt overrides build checks. If you're unsure what this means, see "),
		txt("[this documentation page]", "aqua").OnClick(plugin.OpenURL, "https://worldguard.enginehub.org/en/latest/regions/flags/#overrides"),
		plugin.Plain(" for more info."))
}

func buildWarning() plugin.Text {
	return line(txt("WARNING:", "red").Bold(), txt(" Setting this flag is not required for protection.", "red"),
		plugin.Plain("\nSetting this flag will completely override default protection, and apply"+
			" to members, non-members, pistons, sand physics, and everything else that can modify blocks."),
		plugin.Plain("\nOnly set this flag if you are sure you know what you are doing. See "),
		txt("[this documentation page]", "aqua").OnClick(plugin.OpenURL, "https://worldguard.enginehub.org/en/latest/regions/flags/#protection-related"),
		plugin.Plain(" for more info."))
}

func (wg *WorldGuard) cmdFlag(c *cmdArgs) error {
	wg.warnAboutSaveFailures(c.sender)
	w, err := c.checkWorld('w')
	if err != nil {
		return err
	}
	flagName := c.arg(1)
	var value *string
	if len(c.args) >= 3 {
		v := c.joined(2)
		value = &v
	}
	perms := wg.perms(c.sender)
	if c.has('e') {
		if value != nil {
			return errorf("You cannot use -e(mpty) with a flag value.")
		}
		empty := ""
		value = &empty
	}
	m, err := wg.checkRegionManager(w)
	if err != nil {
		return err
	}
	existing, err := checkExistingRegion(m, c.arg(0), true)
	if err != nil {
		return err
	}
	if !perms.maySetFlags(existing) {
		return errNoPermission
	}
	world := wg.WorldName(w)
	f, ok := worldguard.Flags.FuzzyMatch(flagName)
	if !ok {
		c.sender.SendMessage(wg.flagList(perms, existing, world, flagName))
		return nil
	}
	if value != nil {
		_, isPlayer := c.sender.(plugin.Player)
		switch f {
		case worldguard.FlagBuild, worldguard.FlagBlockBreak, worldguard.FlagBlockPlace:
			c.sender.SendMessage(buildWarning())
			if !isPlayer {
				c.sender.SendMessage(plugin.Plain("https://worldguard.enginehub.org/en/latest/regions/flags/#protection-related"))
			}
		case worldguard.FlagPassthrough:
			c.sender.SendMessage(passthroughWarning())
			if !isPlayer {
				c.sender.SendMessage(plugin.Plain("https://worldguard.enginehub.org/en/latest/regions/flags/#overrides"))
			}
		}
	}
	if !perms.maySetFlagValue(existing, f, value) {
		return errNoPermission
	}
	var groupValue any
	if g, ok := c.value('g'); ok {
		gf := f.GroupFlag()
		if gf == nil {
			return errorf("Region flag '%s' does not have a group flag!", f.Name())
		}
		if groupValue, err = gf.Parse(&worldguard.FlagContext{Sender: c.sender, Input: g, Region: existing}); err != nil {
			return errorf("%s", worldguard.FlagErrorMessage(err))
		}
	}
	if value != nil {
		v, err := wg.setFlag(existing, f, c.sender, w, *value)
		if err != nil {
			return errorf("%s", worldguard.FlagErrorMessage(err))
		}
		value = nil
		if v != nil {
			s := worldguard.FormatFlagValue(f, v)
			value = &s
		}
	}
	switch {
	case value != nil:
		if !c.has('h') {
			printInfo(c.sender, fmt.Sprintf("Region flag %s set on '%s' to '%s'.", f.Name(), existing.ID(), *value))
		}
	case !c.has('g'):
		existing.SetFlag(f, nil)
		if gf := f.GroupFlag(); gf != nil {
			existing.SetFlag(gf, nil)
		}
		if !c.has('h') {
			printInfo(c.sender, fmt.Sprintf("Region flag %s removed from '%s'. (Any -g(roups) were also removed.)", f.Name(), existing.ID()))
		}
	}
	if groupValue != nil {
		gf := f.GroupFlag()
		if groupValue == gf.Default() {
			existing.SetFlag(gf, nil)
			printInfo(c.sender, fmt.Sprintf("Region group flag for '%s' reset to default.", f.Name()))
		} else {
			existing.SetFlag(gf, groupValue)
			printInfo(c.sender, fmt.Sprintf("Region group flag for '%s' set.", f.Name()))
		}
	}
	if h, ok := c.value('h'); ok {
		page, err := parseInt(h)
		if err != nil {
			return err
		}
		return wg.sendFlagHelper(c.sender, w, existing, perms, page)
	}
	p := wg.newPrintout(world, existing, false, c.sender)
	p.b.add(txt("(Current flags: ", "gray"))
	p.flagsList(false)
	p.b.add(txt(")", "gray"))
	c.sender.SendMessage(p.b.text())
	wg.checkSpawnOverlap(c.sender, w, existing)
	return nil
}

// flagList answers the flags the sender may set, for an unknown flag
// name. RegionCommands.FlagListBuilder
func (wg *WorldGuard) flagList(perms *permModel, r *worldguard.Region, world, name string) plugin.Text {
	var names []string
	for _, f := range worldguard.Flags.All() {
		if perms.maySetFlag(r, f) {
			names = append(names, f.Name())
		}
	}
	slices.Sort(names)
	list := []plugin.Text{plugin.Plain("Available flags: ")}
	for i, n := range names {
		color := "white"
		if i%2 == 0 {
			color = "gray"
		}
		list = append(list, suggests(txt(n, color), fmt.Sprintf("/rg flag -w \"%s\" %s %s ", world, r.ID(), n), "Click to set"),
			plugin.Plain(", "))
	}
	out := txt("Unknown flag specified: "+name, "red").Append(plugin.Plain("\n"), line(list...))
	if perms.player != nil {
		out = out.Append(txt("Or use the command ", "light_purple").Append(
			txt("/rg flags "+r.ID(), "aqua").OnClick(plugin.RunCommand, fmt.Sprintf("/rg flags -w \"%s\" %s", world, r.ID()))))
	}
	return out
}

func (wg *WorldGuard) cmdFlags(c *cmdArgs) error {
	w, m, err := c.worldRegions()
	if err != nil {
		return err
	}
	perms := wg.perms(c.sender)
	var r *worldguard.Region
	if len(c.args) == 0 {
		p, ok := c.sender.(plugin.Player)
		if !ok {
			return errorf("Please specify the region with /region flags -w world_name region_name.")
		}
		if r, err = c.checkRegionStandingIn(m, p, true,
			fmt.Sprintf("/rg flags -w \"%s\" %%id%%", wg.WorldName(w)), perms.mayLookup); err != nil {
			return err
		}
	} else if r, err = checkExistingRegion(m, c.arg(0), true); err != nil {
		return err
	}
	if !perms.mayLookup(r) {
		return errNoPermission
	}
	page := 1
	if v, ok := c.value('p'); ok {
		if page, err = parseInt(v); err != nil {
			return err
		}
	}
	return wg.sendFlagHelper(c.sender, w, r, perms, page)
}

func (wg *WorldGuard) sendFlagHelper(s plugin.CommandSender, w plugin.World, r *worldguard.Region, perms *permModel, page int) error {
	h := &flagHelper{world: wg.WorldName(w), region: r, perms: perms}
	if _, ok := s.(plugin.Player); !ok {
		h.monoSpace = true
	}
	wg.checkSpawnOverlap(s, w, r)
	t, err := h.text(page)
	if err != nil {
		printError(s, "Failed to get region flags: "+err.Error())
		return nil
	}
	s.SendMessage(t)
	return nil
}

func (wg *WorldGuard) cmdSetPriority(c *cmdArgs) error {
	wg.warnAboutSaveFailures(c.sender)
	w, err := c.checkWorld('w')
	if err != nil {
		return err
	}
	priority, err := parseInt(c.arg(1))
	if err != nil {
		return err
	}
	m, err := wg.checkRegionManager(w)
	if err != nil {
		return err
	}
	existing, err := checkExistingRegion(m, c.arg(0), false)
	if err != nil {
		return err
	}
	if !wg.perms(c.sender).maySetPriority(existing) {
		return errNoPermission
	}
	existing.SetPriority(priority)
	printInfo(c.sender, fmt.Sprintf("Priority of '%s' set to %d (higher numbers override).", existing.ID(), priority))
	wg.checkSpawnOverlap(c.sender, w, existing)
	return nil
}

func (wg *WorldGuard) cmdSetParent(c *cmdArgs) error {
	wg.warnAboutSaveFailures(c.sender)
	w, m, err := c.worldRegions()
	if err != nil {
		return err
	}
	child, err := checkExistingRegion(m, c.arg(0), false)
	if err != nil {
		return err
	}
	var parent *worldguard.Region
	if len(c.args) == 2 {
		if parent, err = checkExistingRegion(m, c.arg(1), false); err != nil {
			return err
		}
	}
	if !wg.perms(c.sender).maySetParent(child, parent) {
		return errNoPermission
	}
	world := wg.WorldName(w)
	if err := child.SetParent(parent); errors.Is(err, worldguard.ErrCircularInheritance) {
		p := wg.newPrintout(world, parent, false, c.sender)
		p.b.add(txt(fmt.Sprintf("Uh oh! Setting '%s' to be the parent of '%s' would cause circular inheritance.", parent.ID(), child.ID()), "red"))
		p.b.newline()
		p.b.add(txt(fmt.Sprintf("(Current inheritance on '%s':", parent.ID()), "gray"))
		p.b.newline()
		p.parentTree(true)
		p.b.add(txt(")", "gray"))
		c.sender.SendMessage(p.b.text())
		return nil
	} else if err != nil {
		return err
	}
	p := wg.newPrintout(world, child, false, c.sender)
	p.b.add(txt(fmt.Sprintf("Inheritance set for region '%s'.", child.ID()), "light_purple"))
	if parent != nil {
		p.b.newline()
		p.b.add(txt("(Current inheritance:", "gray"))
		p.b.newline()
		p.parentTree(true)
		p.b.add(txt(")", "gray"))
	} else {
		p.b.add(txt(" Region is now orphaned.", "yellow"))
	}
	c.sender.SendMessage(p.b.text())
	return nil
}

func (wg *WorldGuard) cmdRemove(c *cmdArgs) error {
	wg.warnAboutSaveFailures(c.sender)
	_, m, err := c.worldRegions()
	if err != nil {
		return err
	}
	existing, err := checkExistingRegion(m, c.arg(0), true)
	if err != nil {
		return err
	}
	if !wg.perms(c.sender).mayDelete(existing) {
		return errNoPermission
	}
	children, unset := c.has('f'), c.has('u')
	if children && unset {
		return errorf("You cannot use both -u (unset parent) and -f (remove children) together.")
	}
	async(wg, c.sender, "Please wait... removing region.", "Failed to remove region", func() ([]*worldguard.Region, error) {
		switch {
		case children:
			return m.RemoveRegion(existing.ID(), worldguard.RemoveChildren), nil
		case unset:
		default:
			for _, r := range m.Regions() {
				if r.Parent() == existing {
					return nil, errorf("The region '%s' has child regions. Use -f to force removal of children "+
						"or -u to unset the parent value of these children.", existing.ID())
				}
			}
		}
		return m.RemoveRegion(existing.ID(), worldguard.UnsetParentInChildren), nil
	}, func(removed []*worldguard.Region) {
		ids := make([]string, len(removed))
		for i, r := range removed {
			ids[i] = r.ID()
		}
		c.sender.SendMessage(txt("Successfully removed "+strings.Join(ids, ", ")+".", "light_purple"))
	})
	return nil
}

// optionalWorld answers the -w world or the sender's, nil for the console
// without -w.
func (c *cmdArgs) optionalWorld() plugin.World {
	w, err := c.checkWorld('w')
	if err != nil {
		return nil
	}
	return w
}

func (wg *WorldGuard) cmdLoad(c *cmdArgs) error {
	return wg.loadOrSave(c, true)
}

func (wg *WorldGuard) cmdSave(c *cmdArgs) error {
	return wg.loadOrSave(c, false)
}

// loadOrSave runs /rg load and /rg save. RegionCommands.load, save
func (wg *WorldGuard) loadOrSave(c *cmdArgs, load bool) error {
	wg.warnAboutSaveFailures(c.sender)
	w := c.optionalWorld()
	perms := wg.perms(c.sender)
	if load && !perms.mayForceLoadRegions() || !load && !perms.mayForceSaveRegions() {
		return errNoPermission
	}
	run := func(m *worldguard.RegionManager) error {
		if load {
			return m.Load()
		}
		return m.Save()
	}
	if w != nil {
		m, err := wg.checkRegionManager(w)
		if err != nil {
			return err
		}
		name := wg.WorldName(w)
		desc, done, fail := "Loading region data for '%s'.", "Loaded region data for '%s'", "Failed to load region data for '%s'"
		if !load {
			desc, done, fail = "Saving region data for '%s'.", "Saving region data for '%s'", "Failed to save region data for '%s'"
		}
		async(wg, c.sender, "Please wait... "+fmt.Sprintf(desc, name), fmt.Sprintf(fail, name),
			func() (struct{}, error) { return struct{}{}, ioError(run(m)) },
			func(struct{}) { printInfo(c.sender, fmt.Sprintf(done, name)) })
		return nil
	}
	managers := wg.regions.loaded()
	wait, done, fail := "(Please wait... loading region data for all worlds...)", "Successfully load the region data for all worlds.", "Failed to load regions for all worlds"
	if !load {
		wait, done, fail = "(Please wait... saving region data for all worlds...)", "Successfully saved the region data for all worlds.", "Failed to save regions for all worlds"
	}
	async(wg, c.sender, wait, fail, func() (struct{}, error) {
		for _, m := range managers {
			if err := run(m); err != nil {
				return struct{}{}, ioError(err)
			}
		}
		return struct{}{}, nil
	}, func(struct{}) { printInfo(c.sender, done) })
	return nil
}

// ioError shows a storage failure's reason to the sender.
func ioError(err error) error {
	if err == nil {
		return nil
	}
	return errorf("%s", err.Error())
}

// regionDrivers are the region stores /rg migratedb names; only YAML is
// available. WorldGuard: DriverType
var regionDrivers = []string{"YAML", "MYSQL"}

func fuzzyDriver(s string) string {
	for _, d := range regionDrivers {
		if strings.EqualFold(d, strings.ReplaceAll(s, "_", "")) {
			return d
		}
	}
	return ""
}

func (wg *WorldGuard) cmdMigrateDB(c *cmdArgs) error {
	if !wg.perms(c.sender).mayMigrateRegionStore() {
		return errNoPermission
	}
	from, to := fuzzyDriver(c.arg(0)), fuzzyDriver(c.arg(1))
	switch {
	case from == "":
		return errorf("The value of 'from' is not a recognized type of region data database.")
	case to == "":
		return errorf("The value of 'to' is not a recognized type of region region data database.")
	case from == to:
		return errorf("It is not possible to migrate between the same types of region data databases.")
	case !c.has('y'):
		return errorf("This command is potentially dangerous.\n" +
			"Please ensure you have made a backup of your data, and then re-enter the command with -y tacked on at the end to proceed.")
	case from != "YAML":
		return errorf("The driver specified as 'from' does not seem to be supported in your version of WorldGuard.")
	}
	return errorf("The driver specified as 'to' does not seem to be supported in your version of WorldGuard.")
}

// chatLog is a log that also shows its records to a player in chat.
// WorldGuard: LoggerToChatHandler
type chatLog struct {
	slog.Handler
	to plugin.CommandSender
}

func (h chatLog) Handle(ctx context.Context, r slog.Record) error {
	level := "INFO"
	switch {
	case r.Level >= slog.LevelError:
		level = "SEVERE"
	case r.Level >= slog.LevelWarn:
		level = "WARNING"
	}
	printDebug(h.to, level+": "+r.Message)
	return h.Handler.Handle(ctx, r)
}

// migrationLog answers the log a migration run by s writes to.
func (wg *WorldGuard) migrationLog(s plugin.CommandSender) *slog.Logger {
	if _, ok := s.(plugin.Player); ok {
		return slog.New(chatLog{Handler: wg.Log.Handler(), to: s})
	}
	return wg.Log
}

func (wg *WorldGuard) cmdMigrateUUID(c *cmdArgs) error {
	if !wg.perms(c.sender).mayMigrateRegionNames() {
		return errNoPermission
	}
	log := wg.migrationLog(c.sender)
	printInfo(c.sender, "Now performing migration... this may take a while.")
	wg.regions.migrate(&uuidMigration{wg: wg, log: log, keepUnresolved: wg.Config.KeepUnresolvedNames}, log)
	printInfo(c.sender, "Migration complete!")
	return nil
}

func (wg *WorldGuard) cmdMigrateHeights(c *cmdArgs) error {
	if !wg.perms(c.sender).mayMigrateRegionHeights() {
		return errNoPermission
	}
	if !c.has('y') {
		return errorf("This command is potentially dangerous.\n" +
			"Please ensure you have made a backup of your data, and then re-enter the command with -y tacked on at the end to proceed.")
	}
	m := &heightMigration{wg: wg, log: wg.migrationLog(c.sender)}
	if w := c.optionalWorld(); w != nil {
		m.world = wg.WorldName(w)
	}
	wg.regions.migrate(m, m.log)
	printInfo(c.sender, "Migration complete!")
	return nil
}

func (wg *WorldGuard) cmdTeleport(c *cmdArgs) error {
	p, err := checkPlayer(c.sender)
	if err != nil {
		return err
	}
	w, m, err := c.worldRegions()
	if err != nil {
		return err
	}
	existing, err := checkExistingRegion(m, c.arg(0), true)
	if err != nil {
		return err
	}
	perms := wg.perms(p)
	if !perms.mayTeleportTo(existing) {
		return errNoPermission
	}
	lp := wg.WrapPlayer(p)
	var to plugin.Location
	switch {
	case c.has('s'):
		loc, ok := worldguard.EffectiveFlag(existing, worldguard.FlagSpawn, lp).(worldguard.Location)
		if !ok {
			return errorf("The region has no spawn point associated.")
		}
		if to, ok = wg.flagLocation(loc); !ok {
			printError(p, fmt.Sprintf("Unable to teleport to region '%s'.", existing.ID()))
			return nil
		}
	case c.has('c'):
		if !perms.mayTeleportToCenter(existing) {
			return errNoPermission
		}
		center, ok := regionCenter(existing)
		if !ok {
			return errorf("The region has no center point.")
		}
		if p.GameMode() != plugin.Spectator {
			return errorf("Center teleport is only available in Spectator gamemode.")
		}
		to = plugin.Location{World: w, Pos: center}
	default:
		loc, ok := worldguard.EffectiveFlag(existing, worldguard.FlagTeleport, lp).(worldguard.Location)
		if !ok {
			return errorf("The region has no teleport point associated.")
		}
		if to, ok = wg.flagLocation(loc); !ok {
			printError(p, fmt.Sprintf("Unable to teleport to region '%s'.", existing.ID()))
			return nil
		}
	}
	msg, ok := worldguard.EffectiveFlag(existing, worldguard.FlagTeleportMessage, lp).(string)
	if !ok {
		msg, _ = worldguard.FlagTeleportMessage.Default().(string)
	}
	if !p.Teleport(to, plugin.TeleportPlugin) {
		printError(p, fmt.Sprintf("Unable to teleport to region '%s'.", existing.ID()))
		return nil
	}
	if msg != "" {
		printInfo(p, strings.ReplaceAll(msg, "%id%", existing.ID()))
	}
	return nil
}

// flagLocation answers a location flag's value in its world.
func (wg *WorldGuard) flagLocation(l worldguard.Location) (plugin.Location, bool) {
	w, ok := wg.WorldByName(l.World)
	if !ok {
		return plugin.Location{}, false
	}
	return plugin.Location{World: w, Pos: l.Pos, Yaw: l.Yaw, Pitch: l.Pitch}, true
}

// regionCenter answers the center of a region's area as WorldEdit
// computes it. WorldEditRegionConverter.convertToRegion
func regionCenter(r *worldguard.Region) (plugin.Vec3, bool) {
	switch r.Type() {
	case worldguard.RegionCuboid:
		return weregion.NewCuboid(r.Min(), r.Max()).Center(), true
	case worldguard.RegionPolygon:
		pts := make([]weregion.Point2D, 0, len(r.Points()))
		for _, p := range r.Points() {
			pts = append(pts, weregion.Point2D{X: p.X, Z: p.Z})
		}
		return weregion.NewPolygon2D(pts, r.Min().Y, r.Max().Y).Center(), true
	}
	return plugin.Vec3{}, false
}

func (wg *WorldGuard) cmdToggleBypass(c *cmdArgs) error {
	p, err := checkPlayer(c.sender)
	if err != nil {
		return err
	}
	if !wg.HasPermission(p, "worldguard.region.toggle-bypass") {
		return errNoPermission
	}
	s := wg.sessions.get(p)
	enable := s.bypassDisabled
	if len(c.args) > 0 {
		arg := c.arg(0)
		if !strings.EqualFold(arg, "on") && !strings.EqualFold(arg, "off") {
			return errorf("Allowed optional arguments are: on, off")
		}
		enable = strings.EqualFold(arg, "on")
	}
	s.bypassDisabled = !enable
	if enable {
		printInfo(p, "You are now bypassing region protection (as long as you have permission).")
	} else {
		printInfo(p, "You are no longer bypassing region protection.")
	}
	return nil
}

// checkSpawnOverlap warns that r overlaps vanilla's spawn protection, which
// only the overworld has. RegionCommandsBase.checkSpawnOverlap,
// BukkitWorldGuardPlatform.getSpawnProtection
func (wg *WorldGuard) checkSpawnOverlap(s plugin.CommandSender, w plugin.World, r *worldguard.Region) bool {
	radius := wg.Server.SpawnProtection()
	if radius <= 0 || w.Key() != plugin.MinecraftKey("overworld") {
		return false
	}
	at := w.SpawnPoint().Pos.Block()
	spawn, err := worldguard.NewCuboidRegion("__spawn_protection__",
		plugin.BlockPos{X: at.X - radius, Y: w.MinY(), Z: at.Z - radius},
		plugin.BlockPos{X: at.X + radius, Y: w.MinY() + w.Height() - 1, Z: at.Z + radius})
	if err != nil || len(spawn.IntersectingRegions([]*worldguard.Region{r})) == 0 {
		return false
	}
	s.SendMessage(txt("Warning!", "red").Append(txt(" This region overlaps vanilla's spawn protection. WorldGuard cannot "+
		"override this, and only server operators will be able to interact with this area.", "white")))
	return true
}
