package worldguardplugin

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard"
)

// opCommand matches /op and /deop, with or without a namespace.
// WorldGuard: WorldGuardPlayerListener.opPattern
var opCommand = regexp.MustCompile(`(?i)^/?(?:minecraft:)?(?:bukkit:)?(?:de)?op(?:\s.*)?$`)

// registerPlayerRules applies what WorldGuard does to players: chat and
// command flags, de-op on join, the region wand, trampling.
// WorldGuard: WorldGuardPlayerListener
func (p *protection) registerPlayerRules() {
	s := p.wg.Server
	plugin.Listen(s, p.onChat, plugin.ListenOptions{Priority: plugin.Normal, IgnoreCancelled: true})
	plugin.Listen(s, p.onLogin)
	plugin.Listen(s, p.onInteractRules, plugin.ListenOptions{Priority: plugin.High})
	plugin.Listen(s, func(e *plugin.PlayerItemHeldEvent) { p.removeInfiniteStack(e.Player, e.New) }, plugin.ListenOptions{Priority: plugin.High})
	plugin.Listen(s, p.onCommandRules, plugin.ListenOptions{Priority: plugin.Lowest, IgnoreCancelled: true})
}

// onChat applies send-chat and receive-chat. WorldGuard: onPlayerChat
func (p *protection) onChat(e *plugin.PlayerChatEvent) {
	pl := e.Player
	if !p.cfg(pl.World()).UseRegions {
		return
	}
	lp := p.wg.WrapPlayer(pl)
	from := p.query().ApplicableRegions(pl.Location())
	if !from.TestState(lp, worldguard.FlagSendChat) {
		msg, _ := from.QueryValue(lp, worldguard.FlagDenyMessage).(string)
		p.sendDeny(pl, "chat", msg)
		e.SetCancelled(true)
		return
	}
	n := len(e.Recipients)
	e.Recipients = slices.DeleteFunc(slices.Clone(e.Recipients), func(r plugin.Player) bool {
		return !p.query().TestState(r.Location(), p.wg.WrapPlayer(r), worldguard.FlagReceiveChat)
	})
	if len(e.Recipients) != n && len(e.Recipients) == 0 && p.cfg(pl.World()).RegionCancelEmptyChatEvents {
		e.SetCancelled(true)
	}
}

// onLogin refuses a player who did not connect with their host key, and
// deops joining players when deop-on-join is set.
// WorldGuard: onPlayerLogin
func (p *protection) onLogin(e *plugin.PlayerLoginEvent) {
	if e.Result != plugin.LoginAllowed {
		return
	}
	if p.wg.Config.DeopOnJoin {
		e.Player.SetOpLevel(0)
	}
	cfg := p.wg.Config
	if len(cfg.HostKeys) == 0 {
		return
	}
	key, ok := cfg.HostKeys[strings.ToLower(e.Player.UUID().String())]
	if !ok {
		key, ok = cfg.HostKeys[strings.ToLower(e.Player.Name())]
	}
	if !ok {
		return
	}
	host := e.Hostname
	if i := strings.IndexByte(host, ':'); i != -1 {
		host = host[:i]
	}
	// Forge clients append "\0FML\0" to the host name.
	if cfg.HostKeysAllowFMLClients {
		host = strings.TrimSuffix(host, "\x00FML\x00")
	}
	if !strings.EqualFold(host, key) {
		e.Result = plugin.LoginOther
		e.Reason = plugin.Plain("You did not join with the valid host key!")
		p.wg.Log.Warn(fmt.Sprintf("WorldGuard host key check: %s joined with '%s' but '%s' was expected.", e.Player.Name(), host, key))
	}
}

// onInteractRules shows the regions at a block for the region wand, and
// stops players trampling crops and eggs when set. WorldGuard: onPlayerInteract
func (p *protection) onInteractRules(e *plugin.PlayerInteractEvent) {
	pl := e.Player
	switch e.Action {
	case plugin.RightClickBlock:
		p.wand(e)
	case plugin.Physical:
		if !e.DenyBlock && e.Block != nil {
			cfg := p.cfg(pl.World())
			switch e.Block.State().Block() {
			case registry.BlockFarmland:
				if cfg.DisablePlayerCropTrampling {
					e.SetCancelled(true)
				}
			case registry.BlockTurtleEgg:
				if cfg.DisablePlayerTurtleEggTrampling {
					e.SetCancelled(true)
				}
			case registry.BlockSnifferEgg:
				if cfg.DisablePlayerSnifferEggTrample {
					e.SetCancelled(true)
				}
			}
		}
	}
	p.removeInfiniteStack(pl, pl.Inventory().HeldSlot())
}

// wand tells a player holding the region wand which regions hold the block
// they clicked. WorldGuard: handleBlockRightClick
func (p *protection) wand(e *plugin.PlayerInteractEvent) {
	if e.DenyItem || e.Block == nil {
		return
	}
	pl := e.Player
	cfg := p.cfg(e.Block.World)
	if cfg.RemoveInfiniteStacks && inventoryBlock(e.Block.State().Block()) && !p.wg.HasPermission(pl, "worldguard.override.infinite-stack") {
		for slot := range 40 {
			if s := pl.Inventory().Item(slot); s.Count < 0 {
				pl.Inventory().SetItem(slot, plugin.ItemStack{})
				tell(pl, fmt.Sprintf("§cInfinite stack in slot #%d removed.", slot))
			}
		}
	}
	if !cfg.UseRegions || e.Item.Empty() || e.Item.Item != cfg.RegionWand || !p.wg.HasPermission(pl, "worldguard.region.wand") {
		return
	}
	lp := p.wg.WrapPlayer(pl)
	set := p.query().ApplicableRegions(blockLoc(*e.Block))
	if set.Len() > 0 {
		answer := "No"
		if set.TestState(lp, worldguard.FlagBuild) {
			answer = "Yes"
		}
		tell(pl, "§eCan you build? "+answer)
		var ids []string
		for _, r := range set.Regions() {
			ids = append(ids, r.ID())
		}
		tell(pl, "§dApplicable regions: "+strings.Join(ids, ", "))
	} else {
		tell(pl, "§dWorldGuard: No defined regions here!")
	}
	e.DenyItem = true
}

// removeInfiniteStack clears a held stack of negative size, when set.
func (p *protection) removeInfiniteStack(pl plugin.Player, slot int) {
	if !p.cfg(pl.World()).RemoveInfiniteStacks || p.wg.HasPermission(pl, "worldguard.override.infinite-stack") {
		return
	}
	if s := pl.Inventory().Item(slot); s.Count < 0 {
		pl.Inventory().SetItem(slot, plugin.ItemStack{})
		tell(pl, "§cInfinite stack removed.")
	}
}

// onCommandRules applies blocked-cmds and allowed-cmds, and keeps /op and
// /deop to the console when set. WorldGuard: onPlayerCommandPreprocess
func (p *protection) onCommandRules(e *plugin.PlayerCommandEvent) {
	pl := e.Player
	line := "/" + strings.TrimPrefix(e.Command, "/")
	if p.cfg(pl.World()).UseRegions && !p.wg.HasBypass(pl, pl.World()) {
		lp := p.wg.WrapPlayer(pl)
		set := p.query().ApplicableRegions(pl.Location())
		f := commandFilter{permitted: stringList(set.QueryValue(lp, worldguard.FlagAllowedCmds)),
			denied: stringList(set.QueryValue(lp, worldguard.FlagBlockedCmds))}
		if !f.allows(line) {
			msg, _ := set.QueryValue(lp, worldguard.FlagDenyMessage).(string)
			p.sendDeny(pl, "use "+line, msg)
			e.SetCancelled(true)
			return
		}
	}
	if p.wg.Config.BlockInGameOp && opCommand.MatchString(line) {
		tell(pl, "§c/op and /deop can only be used in console (as set by a WG setting).")
		e.SetCancelled(true)
	}
}
