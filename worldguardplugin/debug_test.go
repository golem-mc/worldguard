package worldguardplugin

import (
	"testing"

	"github.com/golem-mc/golem/plugin"
)

func TestDebugSimulatesEvents(t *testing.T) {
	srv, wg := start(t)
	alice, bob := srv.Join("Alice"), srv.Join("Bob")
	addRegion(t, wg, "home", plugin.BlockPos{X: 0, Y: 0, Z: 0}, plugin.BlockPos{X: 10, Y: 100, Z: 10}, alice.Handle())
	look := func(x float64) {
		h := bob.Handle()
		h.Teleport(plugin.Location{World: h.World(), Pos: plugin.Vec3{X: x + .5, Y: 64, Z: x + .5}, Pitch: 90}, plugin.TeleportPlugin)
	}

	contains(t, srv.Console("wg debug testbreak Bob"), "use -t to run the test from the viewpoint of the given player")
	look(3)
	out := srv.Console("wg debug testbreak -t Bob")
	contains(t, out, "Testing BLOCK BREAK at minecraft:")
	contains(t, out, "Was the action blocked? YES")
	contains(t, out, "#1 BLOCKED by a listener at priority")
	contains(t, bob.Messages(1), "(Please ignore any messages that may immediately follow.)")
	contains(t, srv.Console("wg debug testplace -t Bob"), "Was the action blocked? YES")

	look(-3)
	contains(t, srv.Console("wg debug testinteract -t Bob"), "No plugins cancelled the event.")
	contains(t, srv.Console("wg debug testdamage -t Bob"), "Not currently looking at an entity that is close enough.")
	contains(t, srv.Console("wg debug"), "/wg debug <testbreak|testplace|testinteract|testdamage>")
}
