package worldguardplugin

import (
	"os"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/golem/registry"
)

func TestBlacklistDeniesAndTells(t *testing.T) {
	srv, wg := start(t, plugintest.WithGameMode("creative"))
	bob := srv.Join("Bob")
	cfg := wg.WorldConfig(wg.Server.Overworld())
	if err := os.WriteFile(cfg.BlacklistFile, []byte("[stone]\non-break=deny,tell\n[dirt]\non-place=deny,tell\nmessage=&cNo dirt!\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wg.loadBlacklist(cfg)

	bob.StartBreak(2, 63, 2)
	if msgs := bob.Messages(2); !said(msgs, "You're not allowed to break") {
		t.Errorf("Bob was told %q", msgs)
	}
	if blockAt(wg, 2, 63, 2) != registry.BlockStone {
		t.Fatal("Bob broke blacklisted stone")
	}
	bob.Handle().Inventory().SetItem(0, plugin.NewStack(registry.ItemDirt, 8))
	bob.PlaceBlock(3, 63, 3, 1)
	if msgs := bob.Messages(2); !said(msgs, "No dirt!") {
		t.Errorf("Bob was told %q", msgs)
	}
	if blockAt(wg, 3, 64, 3) != registry.BlockAir {
		t.Fatal("Bob placed blacklisted dirt")
	}
	bob.Handle().Inventory().SetItem(0, plugin.NewStack(registry.ItemGravel, 8))
	bob.PlaceBlock(4, 63, 4, 1)
	bob.Read(2)
	if blockAt(wg, 4, 64, 4) != registry.BlockGravel {
		t.Fatal("Bob could not place gravel, which is not blacklisted")
	}
}
