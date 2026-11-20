package worldguard

import "github.com/golem-mc/golem/plugin"

// API is what the WorldGuard plugin offers other plugins, found with
// Lookup. Importing this package registers no plugin, so any plugin can
// depend on it; the service is there once WorldGuard is installed.
// WorldGuard: WorldGuard.getInstance() and its platform
type API interface {
	// RegionManager answers the regions of w; false when regions are off in
	// w or its region data failed to load.
	RegionManager(w plugin.World) (*RegionManager, bool)
	// Query answers flag queries at locations, honouring each world's
	// regions.enable setting and failed loads.
	Query() RegionQuery
	// WrapPlayer answers p as region membership sees it, with its
	// permission groups.
	WrapPlayer(p plugin.Player) LocalPlayer
	// WorldName answers the name WorldGuard knows w by, as in its
	// worlds/<name> folder.
	WorldName(w plugin.World) string
	// HasBypass reports whether p ignores region protection in w.
	HasBypass(p plugin.Player, w plugin.World) bool
}

// Lookup answers the WorldGuard service, false when WorldGuard is not
// installed or not enabled.
func Lookup(s plugin.Server) (API, bool) { return plugin.Service[API](s) }
