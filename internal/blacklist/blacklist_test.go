package blacklist

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

type fakePlayer struct {
	name   string
	groups []string
	perms  map[string]bool
	told   []string
	kicked []string
	banned bool
}

func (p *fakePlayer) Name() string                   { return p.name }
func (p *fakePlayer) Groups() []string               { return p.groups }
func (p *fakePlayer) HasPermission(node string) bool { return p.perms[node] }
func (p *fakePlayer) SendMessage(t plugin.Text)      { p.told = append(p.told, t.PlainString()) }
func (p *fakePlayer) Kick(t plugin.Text)             { p.kicked = append(p.kicked, t.PlainString()) }
func (p *fakePlayer) Ban(t plugin.Text)              { p.banned = true; p.Kick(t) }

func load(t *testing.T, text string, whitelist bool) (*Blacklist, []string) {
	t.Helper()
	var warnings []string
	b, err := Load(strings.NewReader(text), "blacklist.txt", whitelist, func(s string) { warnings = append(warnings, s) })
	if err != nil {
		t.Fatal(err)
	}
	return b, warnings
}

func breakEvent(p Player, b registry.Block) Event {
	return Event{Type: Break, Player: p, Pos: plugin.BlockPos{X: 1, Y: 2, Z: 3}, Target: BlockTarget(b)}
}

func TestLoadHeadingsAndOptions(t *testing.T) {
	b, warnings := load(t, `
# comment
; also a comment
on-break=deny
[stone, minecraft:diamond_ore, 35:14, lava_bucket, nonsense]
on-break=deny,tell,bogus
message=&cNo %s here
frobnicate=1
ignore-perms
`, false)
	if b.Len() != 4 {
		t.Errorf("Len = %d, want 4", b.Len())
	}
	for _, want := range []string{
		"Found option with no heading blacklist.txt for 'on-break=deny'",
		"Could not parse a block/item heading: Unknown block or item name: nonsense",
		"Unknown blacklist action: bogus",
		"Unknown option 'frobnicate' in blacklist.txt for 'frobnicate=1'",
		"Found option with no value blacklist.txt for 'ignore-perms'",
	} {
		found := false
		for _, w := range warnings {
			found = found || w == want
		}
		if !found {
			t.Errorf("no warning %q in %q", want, warnings)
		}
	}
	if _, ok := b.entries["minecraft:red_wool"]; !ok {
		t.Error("the legacy id 35:14 is not red wool")
	}
	p := &fakePlayer{name: "Bob"}
	if b.Check(breakEvent(p, registry.BlockStone), false, false) {
		t.Error("breaking stone was allowed")
	}
	if len(p.told) != 1 || p.told[0] != "No block.minecraft.stone here" {
		t.Errorf("told %q", p.told)
	}
	if !b.Check(breakEvent(p, registry.BlockDirt), false, false) {
		t.Error("breaking dirt was denied")
	}
}

func TestRepeatsAreQuietAndIgnoresApply(t *testing.T) {
	b, _ := load(t, "[stone]\non-break=deny,tell\nignore-groups=Admins\n[dirt]\non-break=deny,kick\nignore-perms=my.perm\n", false)
	p := &fakePlayer{name: "Bob"}
	b.Check(breakEvent(p, registry.BlockStone), false, false)
	b.Check(breakEvent(p, registry.BlockStone), false, false)
	if len(p.told) != 1 || p.told[0] != "You're not allowed to break block.minecraft.stone." {
		t.Errorf("told %q, want one message", p.told)
	}
	b.Check(breakEvent(p, registry.BlockStone), true, false)
	if len(p.told) != 2 {
		t.Error("forceRepeat did not tell again")
	}
	if b.Check(breakEvent(p, registry.BlockStone), false, true) || len(p.told) != 2 {
		t.Error("a silent check told the player or allowed the break")
	}
	if b.Check(breakEvent(p, registry.BlockDirt), false, false) || len(p.kicked) != 1 || p.kicked[0] != "You can't break block.minecraft.dirt" {
		t.Errorf("kicked %q", p.kicked)
	}
	admin := &fakePlayer{name: "Ann", groups: []string{"admins"}, perms: map[string]bool{"my.perm": true}}
	if !b.Check(breakEvent(admin, registry.BlockStone), false, false) || !b.Check(breakEvent(admin, registry.BlockDirt), false, false) {
		t.Error("ignore-groups or ignore-perms did not let Ann through")
	}
}

func TestWhitelist(t *testing.T) {
	b, _ := load(t, "[stone]\non-break=allow\n[dirt]\non-place=allow\n", true)
	p := &fakePlayer{name: "Bob"}
	if !b.Check(breakEvent(p, registry.BlockStone), false, false) {
		t.Error("whitelisted stone was denied")
	}
	if b.Check(breakEvent(p, registry.BlockDirt), false, false) {
		t.Error("dirt, listed without on-break, was allowed")
	}
	if !b.Check(breakEvent(p, registry.BlockGravel), false, false) {
		t.Error("gravel, not listed, was denied")
	}
}

func TestItemTargetPrefersBlock(t *testing.T) {
	if got := ItemTarget(registry.ItemStone); got.ID != "minecraft:stone" || got.Name.Translate != "block.minecraft.stone" {
		t.Errorf("stone = %+v", got)
	}
	if got := ItemTarget(registry.ItemLavaBucket); got.ID != "minecraft:lava_bucket" || got.Name.Translate != "item.minecraft.lava_bucket" {
		t.Errorf("lava bucket = %+v", got)
	}
}

func TestFormat(t *testing.T) {
	if got := format("100%% no %s, %s", "x"); got != "100% no x, %s" {
		t.Errorf("format = %q", got)
	}
}

func TestFileLogger(t *testing.T) {
	dir := t.TempDir()
	f := &FileLogger{Pattern: filepath.Join(dir, "%w", "%u-%Y.log"), CacheSize: 1, World: "My World"}
	p := &fakePlayer{name: "Bob"}
	f.Log(breakEvent(p, registry.BlockStone), "ore", true)
	f.Log(Event{Type: Dispense, Pos: plugin.BlockPos{}, Target: BlockTarget(registry.BlockStone)}, "", false)
	f.Close()
	data, err := os.ReadFile(filepath.Join(dir, "my_world", "bob-"+time.Now().Format("2006")+".log"))
	if err != nil {
		t.Fatal(err)
	}
	if line := string(data); !strings.HasSuffix(line, "] Bob: Tried to break block.minecraft.stone @1,2,3 (ore)\r\n") {
		t.Errorf("logged %q", line)
	}
}

func TestJavaWeekOfYear(t *testing.T) {
	for date, want := range map[string]int{
		"2026-01-01": 1, "2026-01-03": 1, "2026-01-04": 2, "2026-12-26": 52, "2026-12-27": 1, "2026-12-31": 1,
		"2022-12-31": 53, "2023-01-01": 1,
	} {
		d, _ := time.Parse("2006-01-02", date)
		if got := javaWeekOfYear(d); got != want {
			t.Errorf("javaWeekOfYear(%s) = %d, want %d", date, got, want)
		}
	}
}
