package worldguardplugin

import (
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/internal/storage"
)

// saveInterval and loadRetryInterval are WorldGuard's 30 second periods.
// WorldGuard: RegionContainerImpl.SAVE_INTERVAL, LOAD_ATTEMPT_INTERVAL
const (
	saveInterval      = 30 * time.Second
	loadRetryInterval = 30 * time.Second
)

// regionContainer holds the region managers of the loaded worlds and saves
// them in the background. WorldGuard: RegionContainer,
// RegionContainerImpl, BukkitRegionContainer
type regionContainer struct {
	wg     *WorldGuard
	driver storage.Driver

	mu       sync.Mutex
	managers map[string]*worldguard.RegionManager
	// failing maps the key of each world whose data failed to load to its name.
	failing map[string]string
	// saveFailures are the worlds whose last background save failed.
	saveFailures map[string]bool
	lastLoadErr  string
	tasks        []plugin.Task
}

func newRegionContainer(wg *WorldGuard, driver storage.Driver) *regionContainer {
	return &regionContainer{wg: wg, driver: driver, managers: map[string]*worldguard.RegionManager{},
		failing: map[string]string{}, saveFailures: map[string]bool{}}
}

func (c *regionContainer) initialize() {
	c.loadWorlds()
	c.autoMigrate()
	plugin.Listen(c.wg.Server, func(e *plugin.WorldLoadEvent) { c.load(c.wg.WorldName(e.World)) })
	plugin.Listen(c.wg.Server, func(e *plugin.WorldUnloadEvent) { c.unload(c.wg.WorldName(e.World)) },
		plugin.ListenOptions{Priority: plugin.Monitor, IgnoreCancelled: true})
	sched := c.wg.Server.Scheduler()
	c.tasks = append(c.tasks,
		sched.RunAsyncTimer(saveInterval, saveInterval, c.backgroundSave),
		sched.RunAsyncTimer(loadRetryInterval, loadRetryInterval, c.retryLoads))
}

func (c *regionContainer) loadWorlds() {
	c.wg.Log.Info("Loading region data...")
	for _, w := range c.wg.Server.Worlds() {
		c.load(c.wg.WorldName(w))
	}
}

// load reads a world's regions unless regions are off there; a failed load
// is retried in the background.
func (c *regionContainer) load(name string) *worldguard.RegionManager {
	if !c.wg.Config.Get(name).UseRegions {
		return nil
	}
	key := worldguard.NormalizeID(name)
	c.mu.Lock()
	defer c.mu.Unlock()
	if m, ok := c.managers[key]; ok {
		return m
	}
	m, err := c.createAndLoad(name)
	if err != nil {
		c.wg.Log.Warn(fmt.Sprintf("Failed to load the region data for '%s' (periodic attempts will be made to load the data until success): %v", name, err))
		c.failing[key] = name
		return nil
	}
	c.managers[key] = m
	delete(c.failing, key)
	return m
}

func (c *regionContainer) createAndLoad(name string) (*worldguard.RegionManager, error) {
	m := worldguard.NewRegionManager(c.driver.Get(name), c.wg.Log)
	return m, m.Load()
}

func (c *regionContainer) unload(name string) {
	key := worldguard.NormalizeID(name)
	c.mu.Lock()
	defer c.mu.Unlock()
	if m, ok := c.managers[key]; ok {
		if err := m.Save(); err != nil {
			c.wg.Log.Warn(fmt.Sprintf("Failed to save the region data for '%s': %v", name, err))
		}
		delete(c.managers, key)
		delete(c.saveFailures, key)
	}
	delete(c.failing, key)
}

// unloadAll saves the changes of every world and forgets them.
func (c *regionContainer) unloadAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, key := range slices.Sorted(maps.Keys(c.managers)) {
		m := c.managers[key]
		if _, err := m.SaveChanges(); err != nil {
			c.wg.Log.Warn(fmt.Sprintf("Failed to save the region data for '%s' while unloading the data for all worlds: %v", m.Name(), err))
		}
	}
	c.managers = map[string]*worldguard.RegionManager{}
	c.failing = map[string]string{}
	c.saveFailures = map[string]bool{}
}

// reload saves and forgets every world's regions, then loads them again.
func (c *regionContainer) reload() {
	c.unloadAll()
	c.loadWorlds()
}

func (c *regionContainer) shutdown() {
	for _, t := range c.tasks {
		t.Cancel()
	}
	c.unloadAll()
}

func (c *regionContainer) get(name string) (*worldguard.RegionManager, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.managers[worldguard.NormalizeID(name)]
	return m, ok
}

// loaded answers the managers of the loaded worlds, by name order.
func (c *regionContainer) loaded() []*worldguard.RegionManager {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*worldguard.RegionManager, 0, len(c.managers))
	for _, key := range slices.Sorted(maps.Keys(c.managers)) {
		out = append(out, c.managers[key])
	}
	return out
}

// saveFailed reports whether the world's last background save failed.
func (c *regionContainer) saveFailed(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveFailures[worldguard.NormalizeID(name)]
}

// backgroundSave saves the worlds whose regions changed. WorldGuard:
// RegionContainerImpl.BackgroundSaver
func (c *regionContainer) backgroundSave() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, key := range slices.Sorted(maps.Keys(c.managers)) {
		m := c.managers[key]
		saved, err := m.SaveChanges()
		if err != nil {
			c.saveFailures[key] = true
			c.wg.Log.Warn(fmt.Sprintf("Failed to save the region data for '%s' during a periodical save: %v", m.Name(), err))
			continue
		}
		delete(c.saveFailures, key)
		if saved {
			c.wg.Log.Info(fmt.Sprintf("Region data changes made in '%s' have been background saved", m.Name()))
		}
	}
}

// retryLoads tries the worlds whose region data failed to load again.
// WorldGuard: RegionContainerImpl.BackgroundLoader
func (c *regionContainer) retryLoads() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.failing) == 0 {
		return
	}
	c.wg.Log.Info("Attempting to load region data that has previously failed to load...")
	for _, key := range slices.Sorted(maps.Keys(c.failing)) {
		m, err := c.createAndLoad(c.failing[key])
		if err != nil {
			if err.Error() == c.lastLoadErr {
				c.wg.Log.Warn(fmt.Sprintf("Region data is still failing to load, at least for the world named '%s'", key))
				return
			}
			c.wg.Log.Warn(fmt.Sprintf("Region data is still failing to load, at least for the world named '%s': %v", c.failing[key], err))
			c.lastLoadErr = err.Error()
			return
		}
		c.managers[key] = m
		c.wg.Log.Info(fmt.Sprintf("Successfully loaded region data for '%s'", c.failing[key]))
		delete(c.failing, key)
	}
}
