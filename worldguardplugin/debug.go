package worldguardplugin

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard/internal/chat"
)

// maxTraceDistance is how far, in blocks, /wg debug looks for a block or
// an entity. WorldGuard: BukkitDebugHandler.MAX_TRACE_DISTANCE
const maxTraceDistance = 20

// debugCommands is /wg debug. WorldGuard: DebuggingCommands
func debugCommands() *commandGroup {
	test := func(alias, desc string, run func(*WorldGuard, *cmdArgs, plugin.Player) error) *wgCommand {
		return &wgCommand{aliases: []string{alias}, usage: "[player]", desc: desc, flags: "ts", min: 1, max: 1,
			perm: "worldguard.debug.event", run: func(wg *WorldGuard, a *cmdArgs) error {
				target, err := wg.matchSinglePlayer(a.sender, a.args[0])
				if err != nil {
					return err
				}
				return run(wg, a, target)
			}}
	}
	return &commandGroup{aliases: []string{"debug"}, desc: "Debugging commands", subs: []*wgCommand{
		test("testbreak", "Simulate a block break", testBreak),
		test("testplace", "Simulate a block place", testPlace),
		test("testinteract", "Simulate a block interact", testInteract),
		test("testdamage", "Simulate an entity damage", testDamage),
	}}
}

// debugSource answers whose view the test traces from: the target's with
// -t, else the sender's. WorldGuard: BukkitDebugHandler.getSource
func debugSource(a *cmdArgs, target plugin.Player) (plugin.Player, error) {
	if a.has('t') {
		return target, nil
	}
	if p, ok := a.sender.(plugin.Player); ok {
		return p, nil
	}
	return nil, cmdError("If this command is not to be used in-game, use -t to run the test from the viewpoint of the given player rather than yourself.")
}

func traceBlock(a *cmdArgs, target plugin.Player) (plugin.Block, error) {
	src, err := debugSource(a, target)
	if err != nil {
		return plugin.Block{}, err
	}
	if b, ok := src.TargetBlock(maxTraceDistance); ok {
		return b, nil
	}
	return plugin.Block{}, cmdError("Not currently looking at a block that is close enough.")
}

// traceEntity answers the first entity other than target within √10
// blocks of a block along the line of sight. WorldGuard:
// BukkitDebugHandler.traceEntity
func traceEntity(a *cmdArgs, target plugin.Player) (plugin.Entity, error) {
	src, err := debugSource(a, target)
	if err != nil {
		return nil, err
	}
	for _, b := range src.LineOfSight(maxTraceDistance, func(plugin.BlockState) bool { return true }) {
		c := plugin.Vec3{X: float64(b.Pos.X), Y: float64(b.Pos.Y), Z: float64(b.Pos.Z)}
		box := plugin.Box{Min: c.Add(plugin.Vec3{X: -4, Y: -4, Z: -4}), Max: c.Add(plugin.Vec3{X: 4, Y: 4, Z: 4})}
		for _, e := range b.World.EntitiesIn(box) {
			if e.UUID() != target.UUID() && e.Location().Pos.DistanceSq(c) < 10 {
				return e, nil
			}
		}
	}
	return nil, cmdError("Not currently looking at an entity that is close enough.")
}

func describeBlock(b plugin.Block) string {
	return fmt.Sprintf("%s at %d, %d, %d in %s", b.State().Block().Name(), b.Pos.X, b.Pos.Y, b.Pos.Z, b.World.Key())
}

func announceTest(a *cmdArgs, what, at string) {
	a.sender.SendMessage(plugin.Plain("Testing " + what + " at ").Color("aqua").Append(plugin.Plain(at).Color("dark_aqua")))
}

func testBreak(wg *WorldGuard, a *cmdArgs, target plugin.Player) error {
	b, err := traceBlock(a, target)
	if err != nil {
		return err
	}
	announceTest(a, "BLOCK BREAK", describeBlock(b))
	wg.testEvent(a, target, &plugin.BlockBreakEvent{Player: target, Block: b, State: b.State(), DropItems: true})
	return nil
}

func testPlace(wg *WorldGuard, a *cmdArgs, target plugin.Player) error {
	b, err := traceBlock(a, target)
	if err != nil {
		return err
	}
	announceTest(a, "BLOCK PLACE", describeBlock(b))
	s := b.State()
	wg.testEvent(a, target, &plugin.BlockPlaceEvent{Player: target, Block: b, Placed: s, Replaced: s,
		Against: plugin.Block{World: b.World, Pos: b.Pos.Offset(plugin.Down, 1)}, Hand: plugin.MainHand,
		Item: target.Equipment().Get(plugin.SlotMainHand), Blocks: []plugin.BlockPos{b.Pos}})
	return nil
}

func testInteract(wg *WorldGuard, a *cmdArgs, target plugin.Player) error {
	b, err := traceBlock(a, target)
	if err != nil {
		return err
	}
	announceTest(a, "BLOCK INTERACT", describeBlock(b))
	wg.testEvent(a, target, &plugin.PlayerInteractEvent{Player: target, Action: plugin.RightClickBlock,
		Hand: plugin.MainHand, Item: target.Equipment().Get(plugin.SlotMainHand), Block: &b, Face: plugin.South})
	return nil
}

func testDamage(wg *WorldGuard, a *cmdArgs, target plugin.Player) error {
	e, err := traceEntity(a, target)
	if err != nil {
		return err
	}
	p := e.Location().Pos
	announceTest(a, "ENTITY DAMAGE", fmt.Sprintf("%s at %.1f, %.1f, %.1f", e.Type().Name(), p.X, p.Y, p.Z))
	wg.testEvent(a, target, &plugin.EntityDamageEvent{Entity: e, Amount: 1, Final: 1,
		Source: plugin.DamageSource{Type: registry.DamageTypePlayerAttack, Causing: target, Direct: target}})
	return nil
}

// cancelAttempt is a change of an event's cancelled state, seen after the
// listeners of one priority ran. WorldGuard: CancelAttempt
type cancelAttempt struct {
	after    bool
	priority plugin.Priority
}

var priorityNames = map[plugin.Priority]string{plugin.Lowest: "LOWEST", plugin.Low: "LOW",
	plugin.Normal: "NORMAL", plugin.High: "HIGH", plugin.Highest: "HIGHEST", plugin.Monitor: "MONITOR"}

// fireLogged fires e through every plugin's listeners and answers each
// change of its cancelled state. golem does not say which listener made a
// change, so an observer after each priority's listeners notes the state:
// listeners registered later at a priority run after earlier ones.
func (wg *WorldGuard) fireLogged(e cancellable) []cancelAttempt {
	var cancels []cancelAttempt
	last := false
	var observers []plugin.Listener
	for _, pri := range []plugin.Priority{plugin.Lowest, plugin.Low, plugin.Normal, plugin.High, plugin.Highest, plugin.Monitor} {
		observe := func(ev plugin.Event) {
			if ev != e {
				return
			}
			if now := e.Cancelled(); now != last {
				cancels = append(cancels, cancelAttempt{after: now, priority: pri})
				last = now
			}
		}
		observers = append(observers, wg.Server.Events().Register(reflect.TypeOf(e), observe, plugin.ListenOptions{Priority: pri}))
	}
	defer func() {
		for _, o := range observers {
			o.Unregister()
		}
	}()
	wg.Server.Events().Fire(e)
	return cancels
}

// cancelReport answers the report on an event's cancellations.
// WorldGuard: CancelReport.toString
func cancelReport(cancelled bool, cancels []cancelAttempt) string {
	if len(cancels) == 0 {
		return "No plugins cancelled the event. Other causes for cancellation: " +
			"(1) golem may be using a different event for the action " +
			" (example: buckets have their own bucket events); or " +
			"(2) Minecraft's spawn protection has not been disabled."
	}
	var b strings.Builder
	b.WriteString("Was the action blocked? ")
	if cancelled {
		b.WriteString("YES\n")
	} else {
		b.WriteString("NO\n")
	}
	if len(cancels) != 1 {
		b.WriteString("Entry #1 had the last word.\n")
	}
	for i, c := range slices.Backward(cancels) {

		verdict := "ALLOWED"
		if c.after {
			verdict = "BLOCKED"
		}
		fmt.Fprintf(&b, "#%d %s by a listener at priority %s (the plugin is not known)\n",
			len(cancels)-i, verdict, priorityNames[c.priority])
	}
	return b.String()
}

// testEvent fires e as target and reports who cancelled it. WorldGuard:
// BukkitDebugHandler.testEvent
func (wg *WorldGuard) testEvent(a *cmdArgs, target plugin.Player, e cancellable) {
	_, isConsole := a.sender.(plugin.Player)
	isConsole = !isConsole
	if !isSender(target, a.sender) {
		if !isConsole {
			wg.Log.Info(displayName(a.sender) + " is simulating an event on " + target.Name())
		}
		target.SendMessage(chat.Error("(Please ignore any messages that may immediately follow.)"))
	}
	cancels := wg.fireLogged(e)
	result := cancelReport(e.Cancelled(), cancels)
	if a.has('s') {
		a.sender.SendMessage(chat.Debug("The report was printed to console."))
		wg.Log.Info("Event report for " + displayName(a.sender) + ":\n\n" + result)
		return
	}
	for line := range strings.SplitSeq(strings.TrimRight(result, "\n"), "\n") {
		a.sender.SendMessage(plugin.Plain(line).Color("aqua"))
	}
	if len(result) >= 500 && !isConsole {
		a.sender.SendMessage(chat.Debug("The report was also printed to console."))
		wg.Log.Info("Event report for " + displayName(a.sender) + ":\n\n" + result)
	}
}
