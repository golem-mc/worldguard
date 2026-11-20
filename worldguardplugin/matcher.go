package worldguardplugin

import (
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// matchPlayerNames answers the online players a name filter picks:
// "@name" exactly, "*part" by part of the name, otherwise by the start.
// WorldGuard: BukkitStringMatcher.matchPlayerNames
func (wg *WorldGuard) matchPlayerNames(filter string) []plugin.Player {
	filter = strings.ToLower(filter)
	var out []plugin.Player
	for _, p := range wg.Server.Players() {
		name := strings.ToLower(p.Name())
		switch {
		case strings.HasPrefix(filter, "@") && len(filter) >= 2:
			if name == filter[1:] {
				return []plugin.Player{p}
			}
		case strings.HasPrefix(filter, "*") && len(filter) >= 2:
			if strings.Contains(name, filter[1:]) {
				out = append(out, p)
			}
		case strings.HasPrefix(name, filter):
			out = append(out, p)
		}
	}
	return out
}

// matchPlayers answers the players a filter picks: "*" for everyone,
// "#world" and "#near" around the sender, or a name filter. WorldGuard:
// BukkitStringMatcher.matchPlayers
func (wg *WorldGuard) matchPlayers(sender plugin.CommandSender, filter string) ([]plugin.Player, error) {
	online := wg.Server.Players()
	if len(online) == 0 {
		return nil, cmdError("No players matched query.")
	}
	var out []plugin.Player
	switch {
	case filter == "*":
		out = online
	case strings.HasPrefix(filter, "#"):
		src, err := checkPlayer(sender)
		if err != nil {
			return nil, err
		}
		switch strings.ToLower(filter) {
		case "#world":
			for _, p := range online {
				if p.World() == src.World() {
					out = append(out, p)
				}
			}
		case "#near":
			for _, p := range online {
				if p.World() == src.World() && p.Pos().DistanceSq(src.Pos()) < 900 {
					out = append(out, p)
				}
			}
		default:
			return nil, cmdError("Invalid group '" + filter + "'.")
		}
	default:
		out = wg.matchPlayerNames(filter)
	}
	if len(out) == 0 {
		return nil, cmdError("No players matched query.")
	}
	return out, nil
}

// matchSinglePlayer answers the one player a filter picks. WorldGuard:
// StringMatcher.matchSinglePlayer
func (wg *WorldGuard) matchSinglePlayer(sender plugin.CommandSender, filter string) (plugin.Player, error) {
	players, err := wg.matchPlayers(sender, filter)
	if err != nil {
		return nil, err
	}
	if len(players) > 1 {
		return nil, cmdError("More than one player found! Use @<name> for exact matching.")
	}
	return players[0], nil
}

// matchWorld answers the world a filter names: a world's name, #main,
// #normal, #nether, #end or #player:<name>. WorldGuard:
// BukkitStringMatcher.matchWorld
func (wg *WorldGuard) matchWorld(sender plugin.CommandSender, filter string) (plugin.World, error) {
	worlds := wg.Server.Worlds()
	if strings.HasPrefix(filter, "#") {
		byDim := func(dim, what string) (plugin.World, error) {
			for _, w := range worlds {
				if w.Dimension() == plugin.MinecraftKey(dim) {
					return w, nil
				}
			}
			return nil, cmdError("No " + what + " world found.")
		}
		lower := strings.ToLower(filter)
		switch {
		case lower == "#main":
			return wg.Server.Overworld(), nil
		case lower == "#normal":
			return byDim("overworld", "normal")
		case lower == "#nether":
			return byDim("the_nether", "nether")
		case lower == "#end":
			return byDim("the_end", "end")
		case strings.HasPrefix(lower, "#player"):
			_, name, ok := strings.Cut(filter, ":")
			if !ok {
				return nil, cmdError("Argument expected for #player.")
			}
			players, err := wg.matchPlayers(sender, name)
			if err != nil {
				return nil, err
			}
			return players[0].World(), nil
		}
		return nil, cmdError("Invalid identifier '" + filter + "'.")
	}
	for _, w := range worlds {
		if wg.WorldName(w) == filter {
			return w, nil
		}
	}
	return nil, cmdError("No world by that exact name found.")
}

// worldNamesFor completes a world argument.
func (wg *WorldGuard) worldNamesFor() []string {
	var out []string
	for _, w := range wg.Server.Worlds() {
		out = append(out, wg.WorldName(w))
	}
	return out
}
