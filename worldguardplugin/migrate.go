package worldguardplugin

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/internal/storage"
)

// migration rewrites every stored world's regions. WorldGuard: Migration
type migration interface {
	migrate(store *storage.File) error
	post()
}

// migrate saves and unloads the loaded regions, runs m over every world
// with a regions.yml, and loads the regions again. WorldGuard:
// RegionContainer.migrate, AbstractMigration.migrate
func (c *regionContainer) migrate(m migration, log *slog.Logger) {
	log.Info("Unloading and saving region data that is currently loaded...")
	c.unloadAll()
	defer func() {
		log.Info("Loading region data for loaded worlds...")
		c.loadWorlds()
	}()
	for _, store := range c.driver.All() {
		if err := m.migrate(store); err != nil {
			log.Warn(fmt.Sprintf("Migration of one world (%s) failed with an error: %v", store.Name(), err))
		}
	}
	m.post()
}

// autoMigrate converts region owners and members from names to UUIDs once,
// on the first start. WorldGuard: RegionContainer.autoMigrate
func (c *regionContainer) autoMigrate() {
	cfg := c.wg.Config
	if !cfg.MigrateRegionsToUUID {
		return
	}
	c.migrate(&uuidMigration{wg: c.wg, log: c.wg.Log, keepUnresolved: cfg.KeepUnresolvedNames}, c.wg.Log)
	c.wg.Log.Info("Regions saved after UUID migration! This won't happen again unless " +
		"you change the relevant configuration option in WorldGuard's config.")
	cfg.DisableUUIDMigration()
}

// uuidMigration turns player names into UUIDs, looked up among the players
// the server knows. WorldGuard: UUIDMigration
type uuidMigration struct {
	wg             *WorldGuard
	log            *slog.Logger
	keepUnresolved bool
	unresolved     bool
}

func (u *uuidMigration) migrate(store *storage.File) error {
	regions, err := store.LoadAll(worldguard.Flags)
	if err != nil {
		return fmt.Errorf("failed to load region data for the world '%s': %w", store.Name(), err)
	}
	var names []string
	for _, r := range regions {
		names = append(names, r.Owners().Names()...)
		names = append(names, r.Members().Names()...)
	}
	if len(names) == 0 {
		return nil
	}
	u.log.Info(fmt.Sprintf("Migrating regions in '%s' to convert names -> UUIDs...", store.Name()))
	slices.Sort(names)
	u.log.Info(fmt.Sprintf("Resolving %d name(s) into UUIDs... this may take a while.", len(slices.Compact(names))))
	for _, r := range regions {
		u.convert(r.Owners())
		u.convert(r.Members())
	}
	u.log.Info("UUIDs resolved... now migrating all regions to UUIDs where possible...")
	if err := store.SaveAll(regions); err != nil {
		return fmt.Errorf("failed to save region data after migration of the world '%s': %w", store.Name(), err)
	}
	return nil
}

func (u *uuidMigration) convert(d *worldguard.Domain) {
	for _, name := range d.Names() {
		if p, ok := u.wg.Server.OfflinePlayerNamed(name); ok {
			d.AddPlayer(p.UUID())
			d.RemovePlayerName(name)
			continue
		}
		u.unresolved = true
		if !u.keepUnresolved {
			d.RemovePlayerName(name)
		}
	}
}

func (u *uuidMigration) post() {
	if !u.unresolved {
		return
	}
	if u.keepUnresolved {
		u.log.Warn("Some member and owner names do not seem to exist or own Minecraft so they " +
			"could not be converted into UUIDs. They have been left as names, but the conversion can " +
			"be re-run with 'keep-names-that-lack-uuids' set to false in the configuration in " +
			"order to remove these names. Leaving the names means that someone can register with one of " +
			"these names in the future and become that player.")
		return
	}
	u.log.Warn("Some member and owner names do not seem to exist or own Minecraft so they " +
		"could not be converted into UUIDs. These names have been removed.")
}

// heightMigration stretches regions that span the old 0 to 255 world
// height to the world's height. WorldGuard: WorldHeightMigration
type heightMigration struct {
	wg  *WorldGuard
	log *slog.Logger
	// world limits the migration to one world; empty for all.
	world   string
	changed int
}

func (h *heightMigration) migrate(store *storage.File) error {
	if h.world != "" && !strings.EqualFold(store.Name(), h.world) {
		return nil
	}
	h.log.Info(fmt.Sprintf("Migrating regions in '%s' to new height limits...", store.Name()))
	regions, err := store.LoadAll(worldguard.Flags)
	if err != nil {
		return fmt.Errorf("failed to load region data for the world '%s': %w", store.Name(), err)
	}
	lo, hi := -64, 319
	if w, ok := h.wg.WorldByName(store.Name()); ok {
		lo, hi = w.MinY(), w.MinY()+w.Height()-1
		if lo == 0 && hi == 255 {
			return nil
		}
	}
	for _, r := range regions {
		if r.IsPhysical() && r.Min().Y <= 0 && r.Max().Y >= 255 {
			r.SetHeight(lo, hi)
			h.changed++
		}
	}
	if err := store.SaveAll(regions); err != nil {
		return fmt.Errorf("failed to save region data after migration of the world '%s': %w", store.Name(), err)
	}
	return nil
}

func (h *heightMigration) post() {
	h.log.Info(fmt.Sprintf("A total of %d top-to-bottom regions were vertically expanded.", h.changed))
}
