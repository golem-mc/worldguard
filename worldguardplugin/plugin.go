// Package worldguardplugin is the WorldGuard plugin for golem, registered
// as "worldguard": regions and their flags, protection, the blacklist and
// WorldGuard's commands. Other plugins use package worldguard, its API.
package worldguardplugin

import (
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/internal/config"
	"github.com/golem-mc/worldguard/internal/storage"

	// WorldGuard needs WorldEdit, for selections and as upstream does.
	_ "github.com/golem-mc/worldedit"
)

func init() {
	plugin.Register("worldguard", &WorldGuard{},
		plugin.Depends("worldedit"),
		plugin.SoftDepends("luckperms", "vault"),
	)
}

// WorldGuard is the plugin. WorldGuard: WorldGuardPlugin, WorldGuard
type WorldGuard struct {
	Server plugin.Server
	Log    *slog.Logger
	Config *config.Manager

	regions  *regionContainer
	sessions *sessionManager

	levelOnce sync.Once
	levelName string
}

var current struct {
	sync.Mutex
	wg *WorldGuard
}

// Get answers the enabled plugin, for tests.
func Get() (*WorldGuard, bool) {
	current.Lock()
	defer current.Unlock()
	return current.wg, current.wg != nil
}

// Enable reads the configuration, loads every world's regions and starts
// the listeners. WorldGuard: WorldGuardPlugin.onEnable
func (wg *WorldGuard) Enable(s plugin.Server) error {
	wg.Server = s
	wg.Log = s.Logger()
	wg.Config = &config.Manager{DataDir: s.DataDir(), Log: wg.Log}
	if err := wg.Config.Load(wg.worldNames()); err != nil {
		wg.Log.Error("Error saving configuration: " + err.Error())
	}
	wg.sessions = newSessionManager(wg)
	wg.regions = newRegionContainer(wg, storage.Driver{Root: filepath.Join(s.DataDir(), "worlds"), Log: wg.Log})
	wg.regions.initialize()
	wg.sessions.register()
	worldguard.Flags.SetInitialized(true)
	plugin.Provide[worldguard.API](s, &api{wg}, 0)
	current.Lock()
	current.wg = wg
	current.Unlock()
	return nil
}

// Disable saves the regions. WorldGuard: WorldGuard.disable
func (wg *WorldGuard) Disable() {
	if wg.regions != nil {
		wg.regions.shutdown()
	}
	worldguard.Flags.SetInitialized(false)
	current.Lock()
	if current.wg == wg {
		current.wg = nil
	}
	current.Unlock()
}

// worldNames answers the names of the loaded worlds.
func (wg *WorldGuard) worldNames() []string {
	var out []string
	for _, w := range wg.Server.Worlds() {
		out = append(out, wg.WorldName(w))
	}
	return out
}

// WorldConfig answers the configuration of w.
func (wg *WorldGuard) WorldConfig(w plugin.World) *config.World {
	return wg.Config.Get(wg.WorldName(w))
}

// Regions answers the regions of w, false when regions are off there or
// failed to load.
func (wg *WorldGuard) Regions(w plugin.World) (*worldguard.RegionManager, bool) {
	return wg.regions.get(wg.WorldName(w))
}

// Query answers flag queries at locations.
func (wg *WorldGuard) Query() worldguard.RegionQuery {
	return worldguard.RegionQuery{Applicable: wg.applicable}
}

// applicable answers the regions at loc: a permissive set where regions
// are off, a protective one where they failed to load. WorldGuard:
// RegionQuery.getApplicableRegions
func (wg *WorldGuard) applicable(loc plugin.Location) worldguard.ApplicableRegionSet {
	if loc.World == nil {
		return worldguard.PermissiveRegionSet
	}
	name := wg.WorldName(loc.World)
	if !wg.Config.Get(name).UseRegions {
		return worldguard.PermissiveRegionSet
	}
	m, ok := wg.regions.get(name)
	if !ok {
		return worldguard.FailedLoadRegionSet
	}
	return m.ApplicableRegions(loc.Pos.Block(), worldguard.ComputeParents)
}

// api is the worldguard.API service.
type api struct{ wg *WorldGuard }

func (a *api) RegionManager(w plugin.World) (*worldguard.RegionManager, bool) { return a.wg.Regions(w) }
func (a *api) Query() worldguard.RegionQuery                                  { return a.wg.Query() }
func (a *api) WrapPlayer(p plugin.Player) worldguard.LocalPlayer              { return a.wg.WrapPlayer(p) }
func (a *api) WorldName(w plugin.World) string                                { return a.wg.WorldName(w) }
func (a *api) HasBypass(p plugin.Player, w plugin.World) bool                 { return a.wg.HasBypass(p, w) }
