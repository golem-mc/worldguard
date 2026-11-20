package worldguardplugin

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard/internal/chat"
)

// targets answers the players a command acts on: the sender, needing
// perm, or the players its argument picks, needing perm + ".other".
func (wg *WorldGuard) targets(a *cmdArgs, perm string) ([]plugin.Player, error) {
	if a.len() == 0 {
		p, err := checkPlayer(a.sender)
		if err != nil {
			return nil, err
		}
		if !wg.HasPermission(a.sender, perm) {
			return nil, errPermission
		}
		return []plugin.Player{p}, nil
	}
	players, err := wg.matchPlayers(a.sender, a.args[0])
	if err != nil {
		return nil, err
	}
	if !wg.HasPermission(a.sender, perm+".other") {
		return nil, errPermission
	}
	return players, nil
}

// isSender reports whether p ran the command.
func isSender(p plugin.Player, s plugin.CommandSender) bool {
	q, ok := s.(plugin.Player)
	return ok && q.UUID() == p.UUID()
}

// forEachTarget runs act on each target and tells them, unless -s; the
// sender hears others unless they were a target. WorldGuard: the shape of
// every GeneralCommands command
func (wg *WorldGuard) forEachTarget(a *cmdArgs, perm, self, other, summary string, act func(p plugin.Player) bool) error {
	players, err := wg.targets(a, perm)
	if err != nil {
		return err
	}
	included := false
	for _, p := range players {
		if !act(p) {
			continue
		}
		if isSender(p, a.sender) {
			p.SendMessage(chat.Info(self))
			included = true
		} else if !a.has('s') {
			p.SendMessage(chat.Info(other + " " + displayName(a.sender) + "."))
		}
	}
	if !included {
		a.sender.SendMessage(chat.Info(summary))
	}
	return nil
}

// generalCommands are /god, /ungod, /heal, /slay, /locate and /stack.
// WorldGuard: GeneralCommands
func generalCommands() []*wgCommand {
	return []*wgCommand{
		{aliases: []string{"god"}, usage: "[player]", desc: "Enable godmode on a player", flags: "s", max: 1,
			run: func(wg *WorldGuard, a *cmdArgs) error {
				return wg.forEachTarget(a, "worldguard.god", "God mode enabled! Use /ungod to disable.",
					"God enabled by", "Players now have god mode.", func(p plugin.Player) bool {
						wg.sessions.get(p).godMode.on = true
						p.SetFireTicks(0)
						return true
					})
			}},
		{aliases: []string{"ungod"}, usage: "[player]", desc: "Disable godmode on a player", flags: "s", max: 1,
			run: func(wg *WorldGuard, a *cmdArgs) error {
				return wg.forEachTarget(a, "worldguard.god", "God mode disabled!",
					"God disabled by", "Players no longer have god mode.", func(p plugin.Player) bool {
						wg.sessions.get(p).godMode.on = false
						return true
					})
			}},
		{aliases: []string{"heal"}, usage: "[player]", desc: "Heal a player", flags: "s", max: 1,
			run: func(wg *WorldGuard, a *cmdArgs) error {
				return wg.forEachTarget(a, "worldguard.heal", "Healed!", "Healed by", "Players healed.",
					func(p plugin.Player) bool {
						p.SetHealth(p.MaxHealth())
						p.SetFood(20)
						p.SetSaturation(20)
						p.SetExhaustion(0)
						return true
					})
			}},
		{aliases: []string{"slay"}, usage: "[player]", desc: "Slay a player", flags: "s", max: 1,
			run: func(wg *WorldGuard, a *cmdArgs) error {
				return wg.forEachTarget(a, "worldguard.slay", "Slain!", "Slain by", "Players slain.",
					func(p plugin.Player) bool {
						p.SetHealth(0)
						return true
					})
			}},
		{aliases: []string{"locate"}, usage: "[player]", desc: "Locate a player", max: 1, perm: "worldguard.locate",
			run: func(wg *WorldGuard, a *cmdArgs) error {
				p, err := checkPlayer(a.sender)
				if err != nil {
					return err
				}
				if a.len() == 0 {
					p.SetCompassTarget(nil)
					a.sender.SendMessage(chat.Info("Compass reset to spawn."))
					return nil
				}
				target, err := wg.matchSinglePlayer(a.sender, a.args[0])
				if err != nil {
					return err
				}
				loc := target.Location()
				p.SetCompassTarget(&loc)
				a.sender.SendMessage(chat.Info("Compass repointed."))
				return nil
			}},
		{aliases: []string{"stack", ";"}, desc: "Stack items", max: 0, perm: "worldguard.stack",
			run: func(wg *WorldGuard, a *cmdArgs) error {
				p, err := checkPlayer(a.sender)
				if err != nil {
					return err
				}
				wg.stackInventory(p)
				p.SendMessage(chat.Info("Items compacted into stacks!"))
				return nil
			}},
	}
}

// stackInventory merges partial stacks of the same item, leaving
// unstackable items alone. WorldGuard:
// BukkitWorldGuardPlatform.stackPlayerInventory
func (wg *WorldGuard) stackInventory(p plugin.Player) {
	inv := p.Inventory()
	items := inv.Contents()
	f := wg.Server.Items()
	affected := 0
	for i := range items {
		item := &items[i]
		if item.Empty() || f.MaxStackSize(*item) == 1 {
			continue
		}
		maxSize := f.MaxStackSize(*item)
		if item.Count >= maxSize {
			continue
		}
		needed := maxSize - item.Count
		for j := i + 1; j < len(items); j++ {
			other := &items[j]
			if other.Empty() || !f.Similar(*other, *item) {
				continue
			}
			if other.Count > needed {
				item.Count = maxSize
				other.Count -= needed
				affected++
				break
			}
			item.Count += other.Count
			*other = plugin.ItemStack{}
			needed = maxSize - item.Count
			affected++
		}
	}
	if affected > 0 {
		inv.SetContents(items)
	}
}
