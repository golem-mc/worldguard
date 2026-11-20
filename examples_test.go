package worldguard_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
)

// The README's Go examples are copies of the declarations below, so they
// compile; TestReadmeExamplesCompile holds the two together.

// canFight reports whether attacker may hurt someone at loc.
func canFight(s plugin.Server, attacker plugin.Player, loc plugin.Location) bool {
	wg, ok := worldguard.Lookup(s)
	if !ok {
		return true // WorldGuard is not installed
	}
	who := wg.WrapPlayer(attacker)
	if wg.HasBypass(attacker, loc.World) {
		return true
	}
	return wg.Query().QueryState(loc, who, worldguard.FlagPVP) != worldguard.StateDeny
}

// regionsAt lists the ids of the regions at loc, highest priority first.
func regionsAt(s plugin.Server, loc plugin.Location) []string {
	wg, ok := worldguard.Lookup(s)
	if !ok {
		return nil
	}
	var ids []string
	for _, r := range wg.Query().ApplicableRegions(loc).Regions() {
		ids = append(ids, r.ID())
	}
	return ids
}

// greetingAt answers the greeting flag's value at loc for p.
func greetingAt(s plugin.Server, p plugin.Player, loc plugin.Location) string {
	wg, ok := worldguard.Lookup(s)
	if !ok {
		return ""
	}
	set := wg.Query().ApplicableRegions(loc)
	greeting, _ := worldguard.Value[string](set, wg.WrapPlayer(p), worldguard.FlagGreeting)
	return greeting
}

// FlagAllowFlight is a custom flag: "/rg flag <region> allow-flight deny".
var FlagAllowFlight = worldguard.NewStateFlag("allow-flight", true)

func init() {
	// Register flags before WorldGuard loads the regions: in init or Load.
	if err := worldguard.Flags.Register(FlagAllowFlight); err != nil {
		panic(err)
	}
}

func mayFly(s plugin.Server, p plugin.Player) bool {
	wg, ok := worldguard.Lookup(s)
	if !ok {
		return true
	}
	return wg.Query().TestState(p.Location(), wg.WrapPlayer(p), FlagAllowFlight)
}

// claimPlot makes a region owned by p.
func claimPlot(s plugin.Server, p plugin.Player, id string, a, b plugin.BlockPos) error {
	wg, ok := worldguard.Lookup(s)
	if !ok {
		return nil
	}
	regions, ok := wg.RegionManager(p.World())
	if !ok {
		return nil // regions are off in this world
	}
	plot, err := worldguard.NewCuboidRegion(id, a, b)
	if err != nil {
		return err
	}
	plot.Owners().AddPlayer(p.UUID())
	plot.SetFlag(worldguard.FlagGreeting, "Welcome to "+p.Name()+"'s plot")
	regions.AddRegion(plot) // saved in the background
	return nil
}

var _ = []any{canFight, regionsAt, greetingAt, mayFly, claimPlot}

func TestReadmeExamplesCompile(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.ReadFile("examples_test.go")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```go\n(.*?)```").FindAllStringSubmatch(string(readme), -1)
	if len(blocks) == 0 {
		t.Fatal("the README has no Go examples")
	}
	for _, b := range blocks {
		if !strings.Contains(string(self), b[1]) {
			t.Errorf("README example is not in examples_test.go:\n%s", b[1])
		}
	}
}
