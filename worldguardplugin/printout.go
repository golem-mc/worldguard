package worldguardplugin

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
)

// chatWidth is how many characters fit a chat line without wrapping.
// WorldEdit: MessageBox.GUARANTEED_NO_WRAP_CHAT_PAGE_WIDTH
const chatWidth = 47

// textLen answers the length of t's text.
func textLen(t plugin.Text) int { return len([]rune(t.PlainString())) }

// centerAndBorder centers t on a line of struck-through dashes.
// WorldEdit: MessageBox.centerAndBorder
func centerAndBorder(t plugin.Text) plugin.Text {
	side := (chatWidth - textLen(t)) / 2
	border := func() plugin.Text { return strike(txt(strings.Repeat("-", side-1), "yellow")) }
	var parts []plugin.Text
	if side > 0 {
		if side > 1 {
			parts = append(parts, border())
		}
		parts = append(parts, plugin.Plain(" "))
	}
	parts = append(parts, t)
	if side > 0 {
		parts = append(parts, plugin.Plain(" "))
		if side > 1 {
			parts = append(parts, border())
		}
	}
	return line(parts...)
}

// box builds a message: lines joined by newlines.
type box struct{ lines [][]plugin.Text }

func (b *box) add(parts ...plugin.Text) {
	if len(b.lines) == 0 {
		b.lines = append(b.lines, nil)
	}
	b.lines[len(b.lines)-1] = append(b.lines[len(b.lines)-1], parts...)
}

func (b *box) newline() { b.lines = append(b.lines, nil) }

// text answers the box as one component; trailing empty lines dropped.
func (b *box) text() plugin.Text {
	lines := b.lines
	for len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	var parts []plugin.Text
	for i, l := range lines {
		if i > 0 {
			parts = append(parts, plugin.Plain("\n"))
		}
		parts = append(parts, l...)
	}
	return line(parts...)
}

// messageBox answers contents under a centered title. WorldEdit: MessageBox
func messageBox(title string, contents plugin.Text) plugin.Text {
	return line(centerAndBorder(plugin.Plain(title)), plugin.Plain("\n"), contents)
}

// paginate answers page (from 1) of n entries, perPage a page, under title,
// with arrows running cmd with %page% replaced. WorldEdit: PaginationBox
func paginate(title, cmd string, n, perPage, page int, entry func(i int) plugin.Text) (plugin.Text, error) {
	if page == 1 && n == 0 {
		return txt("There are no results.", "red"), nil
	}
	pages := int(math.Ceil(float64(n) / float64(perPage)))
	if page < 1 || page > pages {
		return plugin.Text{}, errorf("Invalid page number.")
	}
	var b box
	last := min(page*perPage, n)
	for i := (page - 1) * perPage; i < last; i++ {
		b.add(entry(i))
		if i+1 != last {
			b.newline()
		}
	}
	nav := []plugin.Text{txt("Page ", "yellow"), txt(strconv.Itoa(page), "gold"), plugin.Plain(" of "), txt(strconv.Itoa(pages), "gold")}
	nav[2] = nav[2].Color("yellow")
	if page > 1 {
		nav = append([]plugin.Text{runs(txt("<<< ", "gold"), strings.ReplaceAll(cmd, "%page%", strconv.Itoa(page-1)), "Click to navigate")}, nav...)
	}
	if page < pages {
		nav = append(nav, runs(txt(" >>>", "gold"), strings.ReplaceAll(cmd, "%page%", strconv.Itoa(page+1)), "Click to navigate"))
	}
	b.newline()
	b.add(centerAndBorder(line(nav...)))
	return messageBox(title, b.text()), nil
}

// printout describes a region. WorldGuard: RegionPrintoutBuilder
type printout struct {
	wg     *WorldGuard
	world  string
	region *worldguard.Region
	// names answers a UUID's last known name; nil shows UUIDs.
	names func(plugin.UUID) (string, bool)
	// perms is nil for the console, which gets no clickable parts.
	perms *permModel
	b     box
}

func (wg *WorldGuard) newPrintout(world string, r *worldguard.Region, withNames bool, s plugin.CommandSender) *printout {
	p := &printout{wg: wg, world: world, region: r}
	if withNames {
		p.names = wg.lastKnownName
	}
	if _, ok := s.(plugin.Player); ok {
		p.perms = wg.perms(s)
	}
	return p
}

func (p *printout) cmd(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

func (p *printout) priority(r *worldguard.Region) plugin.Text {
	s := strconv.Itoa(r.Priority())
	if p.perms != nil && p.perms.maySetPriority(r) {
		return suggests(txt(s, "gold"), p.cmd("/rg setpriority -w \"%s\" %s ", p.world, r.ID()), "Click to change")
	}
	return txt(s, "white")
}

func (p *printout) basics() {
	r := p.region
	p.b.add(txt("Region: ", "blue"), runs(txt(r.ID(), "yellow"), p.cmd("/rg info -w \"%s\" %s", p.world, r.ID()), ""),
		txt(" (type=", "gray"), plugin.Plain(r.Type().String()), txt(", priority=", "gray"), p.priority(r), txt(")", "gray"))
	p.b.newline()
}

// dangerZone are the flags whose setting overrides protection.
// WorldGuard: FlagHelperBox.DANGER_ZONE
func dangerZone(f worldguard.Flag) bool {
	return f == worldguard.FlagBuild || f == worldguard.FlagPassthrough || f == worldguard.FlagBlockPlace || f == worldguard.FlagBlockBreak
}

// flagsList lists the region's flags. RegionPrintoutBuilder.appendFlagsList
func (p *printout) flagsList(colors bool) {
	r := p.region
	any := false
	for _, f := range worldguard.Flags.All() {
		v := r.Flag(f)
		if v == nil {
			continue
		}
		if any {
			p.b.add(plugin.Plain(", "))
		}
		name := f.Name() + ": "
		if g := f.GroupFlag(); g != nil {
			if gv := r.Flag(g); gv != nil {
				name = f.Name() + " -g " + fmt.Sprint(gv) + ": "
			}
		}
		color, valColor := "white", "white"
		if colors {
			valColor = "yellow"
			_, unknown := f.(*worldguard.UnknownFlag)
			switch {
			case dangerZone(f) && (r.ID() != worldguard.GlobalRegionID || f != worldguard.FlagPassthrough):
				color = "dark_red"
			case worldguard.IsBuiltinFlag(f.Name()):
				color = "gold"
			case unknown:
				color = "gray"
			default:
				color = "light_purple"
			}
		}
		t := line(txt(name, color), txt(worldguard.FormatFlagValue(f, v), valColor))
		t.Style.Color = color
		if p.perms != nil && p.perms.maySetFlag(r, f) {
			t = suggests(t, p.cmd("/rg flag -w \"%s\" %s %s ", p.world, r.ID(), f.Name()), "Click to set flag")
		}
		p.b.add(t)
		any = true
	}
	if !any {
		p.b.add(txt("(none)", map[bool]string{true: "red", false: "white"}[colors]))
	}
	if p.perms != nil && p.perms.maySetFlags(r) {
		p.b.add(plugin.Plain(" "), runs(txt("[Flags]", map[bool]string{true: "green", false: "gray"}[colors]),
			p.cmd("/rg flags -w \"%s\" %s", p.world, r.ID()), "Click to set a flag"))
	}
}

func (p *printout) flags() {
	p.b.add(txt("Flags: ", "blue"))
	p.flagsList(true)
	p.b.newline()
}

// parentTree lists the region's parents, the root first.
func (p *printout) parentTree(colors bool) {
	r := p.region
	if r.Parent() == nil {
		return
	}
	var chain []*worldguard.Region
	for q := r; q != nil; q = q.Parent() {
		chain = append(chain, q)
	}
	green := "white"
	if colors {
		green = "green"
	}
	var last *worldguard.Region
	for indent, cur := range slices.Backward(chain) {
		indent = len(chain) - 1 - indent
		prefix := ""
		if indent != 0 {
			prefix = strings.Repeat(" ", indent) + "⤷"
		}
		p.b.add(txt(prefix, green))
		id := txt(cur.ID(), green)
		if p.perms != nil && p.perms.mayLookup(cur) {
			id = runs(id, p.cmd("/rg info -w \"%s\" %s", p.world, cur.ID()), "Click for info")
		}
		p.b.add(id)
		if cur != r {
			gray := "white"
			if colors {
				gray = "gray"
			}
			p.b.add(txt(" (parent, priority=", gray), p.priority(cur), txt(")", gray))
		}
		if last != nil && cur == r && p.perms != nil && p.perms.maySetParent(cur, last) {
			p.b.add(plugin.Plain(" "), suggests(txt("[X]", "red"), p.cmd("/rg setparent -w \"%s\" %s", p.world, cur.ID()), "Click to unlink parent"))
		}
		last = cur
		p.b.newline()
	}
}

// domain shows the owners and members with buttons to change them.
func (p *printout) domain() {
	r := p.region
	add := func(d *worldguard.Domain, addCmd, remCmd string) {
		if d.Size() == 0 {
			p.b.add(txt("(none)", "red"))
		} else if p.perms != nil {
			p.b.add(p.domainText(d))
		} else {
			p.b.add(txt(d.String(p.names), "yellow"))
		}
		if addCmd != "" {
			p.b.add(plugin.Plain(" "), suggests(txt("[Add]", "green"),
				p.cmd("/rg %s -w \"%s\" %s ", addCmd, p.world, r.ID()), "Click to add a player or group"))
		}
		if remCmd != "" && d.Size() > 0 {
			p.b.add(plugin.Plain(" "), suggests(txt("[Rem]", "red"),
				p.cmd("/rg %s -w \"%s\" %s ", remCmd, p.world, r.ID()), "Click to remove a player or group"))
			p.b.add(plugin.Plain(" "), suggests(txt("[Clr]", "red"),
				p.cmd("/rg %s -w \"%s\" -a %s", remCmd, p.world, r.ID()), "Click to clear"))
		}
	}
	pick := func(ok bool, cmd string) string {
		if p.perms != nil && ok {
			return cmd
		}
		return ""
	}
	p.b.add(txt("Owners: ", "blue"))
	add(r.Owners(), pick(p.perms != nil && p.perms.mayAddOwners(r), "addowner"), pick(p.perms != nil && p.perms.mayRemoveOwners(r), "removeowner"))
	p.b.newline()
	p.b.add(txt("Members: ", "blue"))
	add(r.Members(), pick(p.perms != nil && p.perms.mayAddMembers(r), "addmember"), pick(p.perms != nil && p.perms.mayRemoveMembers(r), "removemember"))
	p.b.newline()
}

// domainText shows a domain's players and groups with hovers.
// WorldGuard: DefaultDomain.toUserFriendlyComponent
func (p *printout) domainText(d *worldguard.Domain) plugin.Text {
	byName := map[string]*plugin.UUID{}
	for _, n := range d.Names() {
		byName[n] = nil
	}
	var unknown []string
	for _, id := range d.UUIDs() {
		if p.names != nil {
			if n, ok := p.names(id); ok {
				byName[n] = &id
				continue
			}
		}
		unknown = append(unknown, id.String())
	}
	var parts []plugin.Text
	for i, name := range slices.Sorted(maps.Keys(byName)) {
		if i > 0 {
			parts = append(parts, plugin.Plain(", "))
		}
		t := txt(name, "yellow")
		if id := byName[name]; id == nil {
			t = t.OnHover(line(txt("Name only", "gray"), plugin.Plain("\nClick to copy"))).OnClick(plugin.CopyToClipboard, name)
		} else {
			t = t.OnHover(line(txt("Last known name of uuid: ", "gray"), txt(id.String(), "white"), plugin.Plain("\nClick to copy"))).
				OnClick(plugin.CopyToClipboard, id.String())
		}
		parts = append(parts, t)
	}
	if len(unknown) > 0 {
		if len(parts) > 0 {
			parts = append(parts, plugin.Plain(", "))
		}
		label := fmt.Sprintf("%d unknown uuid", len(unknown))
		if len(unknown) != 1 {
			label += "s"
		}
		parts = append(parts, txt(label, "gray").
			OnHover(line(txt("Unable to resolve the name for:\n", "gray"), txt(strings.Join(unknown, "\n"), "white"), plugin.Plain("\nClick to copy"))).
			OnClick(plugin.CopyToClipboard, strings.Join(unknown, ",")))
	}
	if groups := d.Groups(); len(groups) > 0 {
		if len(parts) > 0 {
			parts = append(parts, plugin.Plain("; "))
		}
		var g []plugin.Text
		for i, name := range groups {
			if i > 0 {
				g = append(g, plugin.Plain(", "))
			}
			g = append(g, txt("g:", "gray"), txt(name, "gold"))
		}
		parts = append(parts, line(g...).OnHover(plugin.Plain("Groups")))
	}
	return line(parts...)
}

// teleportFlag answers the region's effective teleport location for the
// viewer.
func (p *printout) teleportFlag(r *worldguard.Region) (worldguard.Location, bool) {
	var subject worldguard.RegionAssociable
	if p.perms != nil && p.perms.player != nil {
		subject = p.perms.player
	}
	loc, ok := worldguard.EffectiveFlag(r, worldguard.FlagTeleport, subject).(worldguard.Location)
	return loc, ok
}

func (p *printout) bounds() {
	r := p.region
	lo, hi := r.Min(), r.Max()
	p.b.add(txt("Bounds:", "blue"))
	bound := txt(fmt.Sprintf(" (%d, %d, %d) -> (%d, %d, %d)", lo.X, lo.Y, lo.Z, hi.X, hi.Y, hi.Z), "yellow")
	if p.perms != nil && p.perms.maySelect(r) {
		bound = runs(bound, "/rg select "+r.ID(), "Click to select")
	}
	p.b.add(bound)
	tele, ok := p.teleportFlag(r)
	switch {
	case ok && p.perms != nil && p.perms.mayTeleportTo(r):
		hover := line(plugin.Plain("Click to teleport\n"), plugin.Plain(fmt.Sprintf("%d, %d, %d",
			int(math.Floor(tele.Pos.X)), int(math.Floor(tele.Pos.Y)), int(math.Floor(tele.Pos.Z)))))
		p.b.add(plugin.Plain(" "), txt("[Teleport]", "gray").OnHover(hover).
			OnClick(plugin.RunCommand, p.cmd("/rg tp -w \"%s\" %s", p.world, r.ID())))
	case p.perms != nil && p.perms.mayTeleportToCenter(r) && r.IsPhysical():
		p.b.add(plugin.Plain(" "), runs(txt("[Center Teleport]", "gray"),
			p.cmd("/rg tp -c -w \"%s\" %s", p.world, r.ID()), "Click to teleport to the center of the region"))
	}
	p.b.newline()
}

// info answers the whole /rg info box.
func (p *printout) info() plugin.Text {
	p.basics()
	p.flags()
	p.parentTree(true)
	p.domain()
	p.bounds()
	if p.names != nil && p.perms == nil {
		p.b.add(txt("Any names suffixed by * are 'last seen names' and may not be up to date.", "gray"))
	}
	return messageBox("Region Info", p.b.text())
}
