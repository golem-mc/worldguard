package worldguardplugin

import (
	"path/filepath"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/vault"
	"github.com/golem-mc/worldguard"
)

// WorldName answers the name WorldGuard knows w by: the level name for
// the overworld, <level>_nether and <level>_the_end for the vanilla
// nether and end (Bukkit's names), the key's path for another minecraft:
// world, namespace_path for any other.
func (wg *WorldGuard) WorldName(w plugin.World) string {
	if w == nil {
		return ""
	}
	key := w.Key()
	if key.Namespace != "minecraft" {
		return key.Namespace + "_" + key.Path
	}
	switch key.Path {
	case "overworld":
		return wg.LevelName()
	case "the_nether":
		return wg.LevelName() + "_nether"
	case "the_end":
		return wg.LevelName() + "_the_end"
	}
	return key.Path
}

// LevelName answers the overworld's folder name, server.properties'
// level-name.
func (wg *WorldGuard) LevelName() string {
	wg.levelOnce.Do(func() {
		wg.levelName = filepath.Base(wg.Server.WorldFolder(plugin.MinecraftKey("overworld")))
		if wg.levelName == "" || wg.levelName == "." || wg.levelName == string(filepath.Separator) {
			wg.levelName = "world"
		}
	})
	return wg.levelName
}

// WorldByName answers the loaded world WorldGuard names name, ignoring
// case.
func (wg *WorldGuard) WorldByName(name string) (plugin.World, bool) {
	for _, w := range wg.Server.Worlds() {
		if strings.EqualFold(wg.WorldName(w), name) {
			return w, true
		}
	}
	return nil, false
}

// localPlayer is a player as regions see one. WorldGuard: BukkitPlayer
type localPlayer struct {
	plugin.Player
	wg *WorldGuard
}

// WrapPlayer answers p as a worldguard.LocalPlayer.
func (wg *WorldGuard) WrapPlayer(p plugin.Player) worldguard.LocalPlayer {
	if lp, ok := p.(*localPlayer); ok {
		return lp
	}
	return &localPlayer{Player: p, wg: wg}
}

func (p *localPlayer) Association(regions []*worldguard.Region) worldguard.Association {
	return worldguard.PlayerAssociation(p, regions)
}

// HasGroup asks the permission plugin through Vault; without one a
// player is in no group. WorldGuard: WorldGuardPlugin.inGroup
func (p *localPlayer) HasGroup(group string) bool {
	perms, ok := vault.LookupPermission(p.wg.Server)
	if !ok || !perms.GroupSupport() {
		return false
	}
	for _, g := range perms.PlayerGroups(p.World(), p.UUID()) {
		if strings.EqualFold(g, group) {
			return true
		}
	}
	return false
}

// Groups answers the player's permission groups, lower case.
func (wg *WorldGuard) Groups(p plugin.Player) []string {
	perms, ok := vault.LookupPermission(wg.Server)
	if !ok || !perms.GroupSupport() {
		return nil
	}
	var out []string
	for _, g := range perms.PlayerGroups(p.World(), p.UUID()) {
		out = append(out, strings.ToLower(g))
	}
	return out
}

// HasPermission decides a WorldGuard node: an operator has every node in a
// world with op-permissions on, the console every node. WorldGuard:
// WorldGuardPlugin.hasPermission
func (wg *WorldGuard) HasPermission(s plugin.CommandSender, node string) bool {
	if p, ok := s.(plugin.Player); ok {
		if p.OpLevel() > 0 && wg.WorldConfig(p.World()).OpPermissions {
			return true
		}
		return p.HasPermission(node)
	}
	return s.OpLevel() > 0 || s.HasPermission(node)
}

// HasBypass reports whether p ignores region protection in w: they hold
// worldguard.region.bypass.<world> and have not turned bypass off.
// WorldGuard: AbstractSessionManager.hasBypass
func (wg *WorldGuard) HasBypass(p plugin.Player, w plugin.World) bool {
	if lp, ok := p.(*localPlayer); ok {
		p = lp.Player
	}
	if !p.IsValid() {
		return false
	}
	s := wg.sessions.getIfPresent(p)
	if s == nil || s.bypassDisabled {
		return false
	}
	return wg.HasPermission(p, "worldguard.region.bypass."+wg.WorldName(w))
}
