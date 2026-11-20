package worldguardplugin

import (
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard/internal/chat"
)

// label is WorldEdit's LabelFormat: yellow.
func label(s string) plugin.Text { return plugin.Plain(s).Color("yellow") }

// toggleWorld answers the world a fire toggle acts on: the sender's, or
// the one named.
func (wg *WorldGuard) toggleWorld(a *cmdArgs) (plugin.World, error) {
	if a.len() == 0 {
		p, err := checkPlayer(a.sender)
		if err != nil {
			return nil, err
		}
		return p.World(), nil
	}
	return wg.matchWorld(a.sender, a.args[0])
}

func worldArg(wg *WorldGuard, _ *cmdArgs, _ int) []string { return wg.worldNamesFor() }

// toggleCommands are /stopfire, /allowfire and /halt-activity.
// WorldGuard: ToggleCommands
func toggleCommands() []*wgCommand {
	return []*wgCommand{
		{aliases: []string{"stopfire"}, usage: "[<world>]", desc: "Disables all fire spread temporarily",
			max: 1, perm: "worldguard.fire-toggle.stop", complete: worldArg,
			run: func(wg *WorldGuard, a *cmdArgs) error {
				w, err := wg.toggleWorld(a)
				if err != nil {
					return err
				}
				cfg := wg.WorldConfig(w)
				if !cfg.FireSpreadDisableToggle {
					wg.notifyAll(label("Fire spread has been globally disabled for '" + wg.WorldName(w) +
						"' by " + displayName(a.sender) + "."))
				} else {
					a.sender.SendMessage(chat.Info("Fire spread was already globally disabled."))
				}
				cfg.FireSpreadDisableToggle = true
				return nil
			}},
		{aliases: []string{"allowfire"}, usage: "[<world>]", desc: "Allows all fire spread temporarily",
			max: 1, perm: "worldguard.fire-toggle.stop", complete: worldArg,
			run: func(wg *WorldGuard, a *cmdArgs) error {
				w, err := wg.toggleWorld(a)
				if err != nil {
					return err
				}
				cfg := wg.WorldConfig(w)
				if cfg.FireSpreadDisableToggle {
					wg.notifyAll(label("Fire spread has been globally for '" + wg.WorldName(w) +
						"' re-enabled by " + displayName(a.sender) + "."))
				} else {
					a.sender.SendMessage(chat.Info("Fire spread was already globally enabled."))
				}
				cfg.FireSpreadDisableToggle = false
				return nil
			}},
		{aliases: []string{"halt-activity", "stoplag", "haltactivity"}, usage: "[confirm]",
			desc: "Attempts to cease as much activity in order to stop lag", flags: "cis", max: 1,
			perm:     "worldguard.halt-activity",
			complete: func(*WorldGuard, *cmdArgs, int) []string { return []string{"confirm"} },
			run:      haltActivity},
	}
}

// haltActivity is /halt-activity. WorldGuard: ToggleCommands.stopLag
func haltActivity(wg *WorldGuard, a *cmdArgs) error {
	cfg := wg.Config
	_, isPlayer := a.sender.(plugin.Player)
	if a.has('i') {
		if cfg.ActivityHaltToggle {
			a.sender.SendMessage(chat.Info("ALL intensive server activity is not allowed."))
		} else {
			a.sender.SendMessage(chat.Info("ALL intensive server activity is allowed."))
		}
		return nil
	}
	halt := !a.has('c')
	if halt && (a.len() == 0 || !strings.EqualFold(a.args[0], "confirm")) {
		confirm := "/" + a.label + " confirm"
		a.sender.SendMessage(plugin.Plain("").Append(
			chat.Error("This command will "), chat.Error("PERMANENTLY").Bold(),
			chat.Error(" erase ALL animals in ALL loaded chunks in ALL loaded worlds. "),
			plugin.Plain("\n"),
			plugin.Plain("[Click]").Color("green").OnClick(plugin.RunCommand, confirm).
				OnHover(plugin.Plain("Click to confirm "+"/"+a.label)),
			chat.Error(" or type "),
			plugin.Plain(confirm).Color("aqua").OnClick(plugin.SuggestCommand, confirm),
			chat.Error(" to confirm.")))
		return nil
	}
	cfg.ActivityHaltToggle = halt
	if !halt {
		if !a.has('s') {
			wg.notifyAll(label("ALL intensive server activity is now allowed."))
			if !isPlayer {
				a.sender.SendMessage(chat.Info("ALL intensive server activity is now allowed."))
			}
		} else {
			a.sender.SendMessage(chat.Info("(Silent) ALL intensive server activity is now allowed."))
		}
		return nil
	}
	if !isPlayer {
		a.sender.SendMessage(chat.Info("ALL intensive server activity halted."))
	}
	if !a.has('s') {
		wg.notifyAll(label("ALL intensive server activity halted by " + displayName(a.sender) + "."))
	} else {
		a.sender.SendMessage(chat.Info("(Silent) ALL intensive server activity halted by " + displayName(a.sender) + "."))
	}
	for _, w := range wg.Server.Worlds() {
		if removed := removeIntensiveEntities(w); removed > 10 {
			sendRaw(a.sender, strconv.Itoa(removed)+" entities (>10) auto-removed from "+wg.WorldName(w))
		}
	}
	return nil
}
