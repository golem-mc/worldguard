package worldguardplugin

import (
	"fmt"
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
)

// regionLister lists a world's regions for /rg list. WorldGuard:
// RegionLister
type regionLister struct {
	wg     *WorldGuard
	sender plugin.CommandSender
	m      *worldguard.RegionManager
	world  string
	page   int
	// owner filters by an owner or member: by name only, or by the
	// player's UUID (ownerID) too.
	owner    string
	nameOnly bool
	ownerID  *plugin.UUID
	idFilter *string
	// intersecting filters by overlap with the sender's selection.
	intersecting *worldguard.Region
}

type listEntry struct {
	region          *worldguard.Region
	owner, isMember bool
}

func (l *regionLister) contains(d *worldguard.Domain) bool {
	if d.ContainsName(l.owner) {
		return true
	}
	return l.ownerID != nil && d.ContainsUUID(*l.ownerID)
}

func (l *regionLister) matches(e *listEntry) bool {
	if l.idFilter != nil && !strings.Contains(e.region.ID(), *l.idFilter) {
		return false
	}
	if l.owner == "" {
		return true
	}
	if e.owner = l.contains(e.region.Owners()); e.owner {
		return true
	}
	e.isMember = l.contains(e.region.Members())
	return e.isMember
}

func compareEntries(a, b *listEntry) int {
	if a.owner != b.owner {
		if a.owner {
			return -1
		}
		return 1
	}
	if a.isMember != b.isMember {
		if a.isMember {
			return -1
		}
		return 1
	}
	return strings.Compare(a.region.ID(), b.region.ID())
}

// text answers the page of the list. RegionLister.call
func (l *regionLister) text() (plugin.Text, error) {
	regions := l.m.Regions()
	all := make([]*worldguard.Region, 0, len(regions))
	for _, r := range regions {
		all = append(all, r)
	}
	if l.intersecting != nil {
		all = l.intersecting.IntersectingRegions(all)
	}
	var entries []*listEntry
	for _, r := range all {
		if r.ID() == worldguard.GlobalRegionID {
			continue
		}
		if e := (&listEntry{region: r}); l.matches(e) {
			entries = append(entries, e)
		}
	}
	if l.owner == "" {
		slices.SortFunc(entries, compareEntries)
	}
	if g, ok := regions[worldguard.GlobalRegionID]; ok {
		if e := (&listEntry{region: g}); l.matches(e) {
			entries = append([]*listEntry{e}, entries...)
		}
	}
	if l.owner != "" {
		slices.SortStableFunc(entries, compareEntries)
	}
	var perms *permModel
	if _, ok := l.sender.(plugin.Player); ok {
		perms = l.wg.perms(l.sender)
	}
	title := "Regions"
	cmd := fmt.Sprintf("/rg list -w \"%s\"", l.world)
	if l.owner != "" {
		title = "Regions for " + l.owner
		cmd += " -p " + l.owner
	}
	if l.nameOnly {
		cmd += " -n"
	}
	if l.intersecting != nil {
		cmd += " -s"
	}
	if l.idFilter != nil {
		cmd += " -i " + *l.idFilter
	}
	cmd += " %page%"
	return paginate(title, cmd, len(entries), 8, l.page, func(i int) plugin.Text {
		e := entries[i]
		parts := []plugin.Text{txt(fmt.Sprintf("%d.", i+1), "light_purple")}
		switch {
		case e.owner:
			parts = append(parts, plugin.Plain(" "), txt("+", "dark_aqua").OnHover(txt("Region Owner", "gold")))
		case e.isMember:
			parts = append(parts, plugin.Plain(" "), txt("-", "aqua").OnHover(txt("Region Member", "gold")))
		}
		parts = append(parts, plugin.Plain(" "), txt(e.region.ID(), "gold"))
		if perms != nil && perms.mayLookup(e.region) {
			parts = append(parts, plugin.Plain(" "), runs(txt("[Info]", "gray"),
				fmt.Sprintf("/rg info -w \"%s\" %s", l.world, e.region.ID()), "Click for info"))
		}
		var subject worldguard.RegionAssociable
		if perms != nil {
			subject = perms.player
		}
		_, tele := worldguard.EffectiveFlag(e.region, worldguard.FlagTeleport, subject).(worldguard.Location)
		switch {
		case perms != nil && tele && perms.mayTeleportTo(e.region):
			parts = append(parts, plugin.Plain(" "), runs(txt("[TP]", "gray"),
				fmt.Sprintf("/rg tp -w \"%s\" %s", l.world, e.region.ID()), "Click to teleport"))
		case perms != nil && perms.mayTeleportToCenter(e.region) && e.region.IsPhysical():
			parts = append(parts, plugin.Plain(" "), runs(txt("[TP-Center]", "gray"),
				fmt.Sprintf("/rg tp -c -w \"%s\" %s", l.world, e.region.ID()), "Click to teleport to the center"))
		}
		t := line(parts...)
		t.Style.Color = "light_purple"
		return t
	})
}
