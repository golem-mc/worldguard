package worldguardplugin

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldedit"
	weregion "github.com/golem-mc/worldedit/region"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/internal/chat"
)

// checkWorld answers the world of the -f flag, else the sender's.
// WorldGuard: RegionCommandsBase.checkWorld
func (c *cmdArgs) checkWorld(f byte) (plugin.World, error) {
	if name, ok := c.value(f); ok {
		return c.wg.matchWorld(c.sender, name)
	}
	if w := c.wg.worldOverride(c.sender); w != nil {
		return w, nil
	}
	if p, ok := c.sender.(plugin.Player); ok {
		return p.World(), nil
	}
	return nil, errorf("Please specify the world with -%c world_name.", f)
}

func checkRegionID(id string, allowGlobal bool) (string, error) {
	if !worldguard.ValidRegionID(id) {
		return "", errorf("The region name of '%s' contains characters that are not allowed.", id)
	}
	if !allowGlobal && strings.EqualFold(id, worldguard.GlobalRegionID) {
		return "", errorf("Sorry, you can't use __global__ here.")
	}
	return id, nil
}

// checkExistingRegion answers the region id, making __global__ when it is
// asked for and missing.
func checkExistingRegion(m *worldguard.RegionManager, id string, allowGlobal bool) (*worldguard.Region, error) {
	if _, err := checkRegionID(id, allowGlobal); err != nil {
		return nil, err
	}
	if r, ok := m.Region(id); ok {
		return r, nil
	}
	if strings.EqualFold(id, worldguard.GlobalRegionID) {
		r, err := worldguard.NewGlobalRegion(id)
		if err != nil {
			return nil, err
		}
		m.AddRegion(r)
		return r, nil
	}
	return nil, errorf("No region could be found with the name of '%s'.", id)
}

// checkRegionStandingIn answers the one region the player stands in.
// rgCmd, with %id% for a region, is what clicking a listed region runs.
func (c *cmdArgs) checkRegionStandingIn(m *worldguard.RegionManager, p plugin.Player, allowGlobal bool,
	rgCmd string, allowed func(*worldguard.Region) bool) (*worldguard.Region, error) {
	set := m.ApplicableRegions(p.Location().Pos.Block(), worldguard.NoParents)
	regions := set.Regions()
	switch {
	case len(regions) == 0 && allowGlobal:
		global, err := checkExistingRegion(m, worldguard.GlobalRegionID, true)
		if err != nil {
			return nil, err
		}
		printDebug(p, "You're not standing in any regions. Using the global region for this world instead.")
		return global, nil
	case len(regions) == 0:
		return nil, errorf("You're not standing in a region. Specify an ID if you want to select a specific region.")
	case len(regions) > 1:
		shown := slices.DeleteFunc(slices.Clone(regions), func(r *worldguard.Region) bool { return !allowed(r) })
		hidden := len(regions) - len(shown)
		msg := []plugin.Text{txt("Current regions: ", "gold")}
		for i, r := range shown {
			if i > 0 {
				msg = append(msg, plugin.Plain(", "))
			}
			id := txt(r.ID(), "aqua")
			if strings.Contains(rgCmd, "%id%") {
				id = runs(id, strings.ReplaceAll(rgCmd, "%id%", r.ID()), "Click to pick this region")
			}
			msg = append(msg, id)
		}
		switch {
		case len(shown) > 0 && hidden > 0:
			msg = append(msg, txt(fmt.Sprintf(", and %d hidden regions", hidden), "gray"))
		case len(shown) == 0:
			msg = append(msg, txt(fmt.Sprintf("%d hidden regions", hidden), "gray"))
		}
		p.SendMessage(line(msg...))
		return nil, errorf("You're standing in several regions (please pick one).")
	}
	return regions[0], nil
}

// worldOverride answers the sender's WorldEdit //world, nil for none.
func (wg *WorldGuard) worldOverride(s plugin.CommandSender) plugin.World {
	we, ok := wg.worldEdit()
	if !ok {
		return nil
	}
	sess := we.ConsoleSession()
	if p, ok := s.(plugin.Player); ok {
		sess = we.Session(p)
	}
	if sess == nil {
		return nil
	}
	w := sess.WorldOverride()
	if p, ok := s.(plugin.Player); ok && w != nil && w.Key() != p.World().Key() {
		printDebug(s, "Using //world override for region command: "+wg.WorldName(w))
	}
	return w
}

// worldEdit answers WorldEdit's API.
func (wg *WorldGuard) worldEdit() (worldedit.API, bool) {
	return plugin.Service[worldedit.API](wg.Server)
}

// checkSelection answers the sender's WorldEdit selection.
func (wg *WorldGuard) checkSelection(s plugin.CommandSender) (weregion.Region, error) {
	fail := errorf("Please select an area first. Use WorldEdit to make a selection! " +
		"(see: https://worldedit.enginehub.org/en/latest/usage/regions/selections/).")
	p, ok := s.(plugin.Player)
	we, found := wg.worldEdit()
	if !ok || !found {
		return nil, fail
	}
	sel, err := we.Selection(p)
	if err != nil {
		return nil, fail
	}
	return sel, nil
}

func checkRegionDoesNotExist(m *worldguard.RegionManager, id string, mayRedefine bool) error {
	if !m.HasRegion(id) {
		return nil
	}
	msg := "A region with that name already exists. Please choose another name."
	if mayRedefine {
		msg += " To change the shape, use /region redefine " + id + "."
	}
	return errorf("%s", msg)
}

// checkRegionManager answers the regions of w.
func (wg *WorldGuard) checkRegionManager(w plugin.World) (*worldguard.RegionManager, error) {
	if !wg.WorldConfig(w).UseRegions {
		return nil, errorf("Region support is disabled in the target world. " +
			"It can be enabled per-world in WorldGuard's configuration files. " +
			"However, you may need to restart your server afterwards.")
	}
	m, ok := wg.Regions(w)
	if !ok {
		return nil, errorf("Region data failed to load for this world. " +
			"Please ask a server administrator to read the logs to identify the reason.")
	}
	return m, nil
}

// regionFromSelection makes a region id of the sender's selection.
func (wg *WorldGuard) regionFromSelection(s plugin.CommandSender, id string) (*worldguard.Region, error) {
	sel, err := wg.checkSelection(s)
	if err != nil {
		return nil, err
	}
	switch sel := sel.(type) {
	case *weregion.Polygon2D:
		pts := make([]worldguard.Point2D, len(sel.Points))
		for i, p := range sel.Points {
			pts[i] = worldguard.Point2D{X: p.X, Z: p.Z}
		}
		return worldguard.NewPolygonRegion(id, pts, sel.Min().Y, sel.Max().Y)
	case *weregion.Cuboid:
		return worldguard.NewCuboidRegion(id, sel.Min(), sel.Max())
	}
	return nil, errorf("Sorry, you can only use cuboids and polygons for WorldGuard regions.")
}

// warnAboutSaveFailures tells the sender when background saves fail.
func (wg *WorldGuard) warnAboutSaveFailures(s plugin.CommandSender) {
	var failing []string
	for _, m := range wg.regions.loaded() {
		if wg.regions.saveFailed(m.Name()) {
			failing = append(failing, "'"+m.Name()+"'")
		}
	}
	if len(failing) > 0 {
		s.SendMessage(txt("(Warning: The background saving of region data is failing for these worlds: "+
			strings.Join(failing, ", ")+". Your changes are getting lost. See the server log for more information.)", "gold"))
	}
}

func warnAboutDimensions(s plugin.CommandSender, r *worldguard.Region) {
	if !r.IsPhysical() {
		return
	}
	if h := r.Max().Y - r.Min().Y; h <= 2 {
		printDebug(s, fmt.Sprintf("(Warning: The height of the region was %d block(s).)", h+1))
	}
}

func informNewUser(s plugin.CommandSender, m *worldguard.RegionManager, r *worldguard.Region) {
	if m.Len() <= 2 {
		s.SendMessage(line(txt("(This region is NOW PROTECTED from modification from others. Don't want that? Use ", "gray"),
			txt("/rg flag "+r.ID()+" passthrough allow", "aqua"), txt(")", "gray")))
	}
}

// setPlayerSelection makes r the sender's WorldEdit selection in w.
func (wg *WorldGuard) setPlayerSelection(s plugin.CommandSender, r *worldguard.Region, w plugin.World) error {
	p, err := checkPlayer(s)
	if err != nil {
		return err
	}
	we, ok := wg.worldEdit()
	if !ok || !r.IsPhysical() {
		return errorf("Can't select that region! The region type '%s' can't be selected.", r.Type())
	}
	sess := we.Session(p)
	switch r.Type() {
	case worldguard.RegionCuboid:
		sel := sess.SetSelectorType(w, weregion.SelectorCuboid).(*weregion.CuboidSelector)
		lo, hi := r.Min(), r.Max()
		sel.Pos1, sel.Pos2 = &lo, &hi
	default:
		sel := sess.SetSelectorType(w, weregion.SelectorPoly)
		pts := r.Points()
		sel.SelectPrimary(plugin.BlockPos{X: pts[0].X, Y: r.Min().Y, Z: pts[0].Z}, 0)
		for _, pt := range pts[1:] {
			sel.SelectSecondary(plugin.BlockPos{X: pt.X, Y: r.Max().Y, Z: pt.Z}, 0)
		}
	}
	we.Engine().DispatchCUI(p, sess)
	printInfo(s, "Region selected as "+r.Type().String())
	return nil
}

// setFlag parses value for f and sets it on r. RegionCommandsBase.setFlag
func (wg *WorldGuard) setFlag(r *worldguard.Region, f worldguard.Flag, s plugin.CommandSender, w plugin.World, value string) (any, error) {
	ctx := &worldguard.FlagContext{Sender: s, Input: value, World: wg.WorldName(w), Region: r,
		Warn: func(msg string) { printDebug(s, msg) }}
	if p, ok := s.(plugin.Player); ok {
		ctx.Player = p
		ctx.World = wg.WorldName(p.World())
		ctx.BoundedLocations = wg.WorldConfig(p.World()).BoundedLocationFlags
		ctx.MayOverrideBounds = wg.perms(s).mayOverrideLocationBounds(r)
	}
	v, err := f.Parse(ctx)
	if err != nil {
		return nil, err
	}
	r.SetFlag(f, v)
	return v, nil
}

// async runs task off the tick goroutine, then on the tick goroutine either
// done with its result or, on failure, "<failure>: <reason>" in red. A
// non-empty wait is shown when the task takes over a second. WorldEdit:
// AsyncCommandBuilder
func async[T any](wg *WorldGuard, s plugin.CommandSender, wait, failure string, task func() (T, error), done func(T)) {
	sched := wg.Server.Scheduler()
	finished := make(chan struct{})
	if wait != "" {
		sched.RunAsyncLater(time.Second, func() {
			select {
			case <-finished:
			default:
				sched.RunTask(func() { printDebug(s, wait) })
			}
		})
	}
	sched.RunAsync(func() {
		v, err := task()
		close(finished)
		sched.RunTask(func() {
			if err == nil {
				if done != nil {
					done(v)
				}
				return
			}
			if failure == "" {
				failure = "An error occurred"
			}
			if msg, ok := errors.AsType[cmdMsg](err); ok {
				printError(s, failure+": "+string(msg))
				return
			}
			wg.Log.Error("Uncaught exception occurred in task", "error", err)
			printError(s, failure+": Unknown error. Please see console.")
		})
	})
}

// txt answers s in color ("" for none).
func txt(s, color string) plugin.Text { return plugin.Plain(s).Color(color) }

// line joins components into one.
func line(parts ...plugin.Text) plugin.Text { return plugin.Text{}.Append(parts...) }

// strike answers t struck through.
func strike(t plugin.Text) plugin.Text { t.Style.Strikethrough = new(true); return t }

// runs and suggests make t run or suggest a command when clicked, with a
// hover text.
func runs(t plugin.Text, command, hover string) plugin.Text {
	t = t.OnClick(plugin.RunCommand, command)
	if hover != "" {
		t = t.OnHover(plugin.Plain(hover))
	}
	return t
}

func suggests(t plugin.Text, command, hover string) plugin.Text {
	t = t.OnClick(plugin.SuggestCommand, command)
	if hover != "" {
		t = t.OnHover(plugin.Plain(hover))
	}
	return t
}

// printLines sends msg line by line with a § prefix, so § codes in msg
// (a flag's value) apply: §d for print, §c for an error, §7 for debug
// output. WorldEdit: BukkitPlayer.print, printError, printDebug
func printLines(s plugin.CommandSender, prefix, msg string) {
	for part := range strings.SplitSeq(msg, "\n") {
		s.SendMessage(chat.Legacy(prefix + part))
	}
}

func printInfo(s plugin.CommandSender, msg string)  { printLines(s, "§d", msg) }
func printError(s plugin.CommandSender, msg string) { printLines(s, "§c", msg) }
func printDebug(s plugin.CommandSender, msg string) { printLines(s, "§7", msg) }
