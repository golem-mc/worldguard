// Package config reads WorldGuard's config.yml and the per-world
// worlds/<world>/config.yml, writing the defaults back as WorldGuard does.
package config

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldedit/block"
	"github.com/golem-mc/worldguard"
	"github.com/golem-mc/worldguard/internal/yamlfile"
)

//go:embed defaults
var defaults embed.FS

// Default answers one of WorldGuard's default files: config.yml,
// config_world.yml or blacklist.txt.
func Default(name string) []byte {
	data, _ := defaults.ReadFile("defaults/" + name)
	return data
}

// header is config.yml's header. WorldGuard: ConfigurationManager.CONFIG_HEADER
const header = `#
# WorldGuard's main configuration file
#
# This is the global configuration file. Anything placed into here will
# be applied to all worlds. However, each world has its own configuration
# file to allow you to replace most settings in here for that world only.
#
# About editing this file:
# - DO NOT USE TABS. You MUST use spaces or Bukkit will complain. If
#   you use an editor like Notepad++ (recommended for Windows users), you
#   must configure it to "replace tabs with spaces." In Notepad++, this can
#   be changed in Settings > Preferences > Language Menu.
# - Don't get rid of the indents. They are indented so some entries are
#   in categories (like "enforce-single-session" is in the "protection"
#   category.
# - If you want to check the format of this file before putting it
#   into WorldGuard, paste it into http://yaml-online-parser.appspot.com/
#   and see if it gives "ERROR:".
# - Lines starting with # are comments and so they are ignored.
#
`

// worldHeader is a world config's header. WorldGuard:
// WorldConfiguration.CONFIG_HEADER
const worldHeader = `#
# WorldGuard's world configuration file
#
# This is a world configuration file. Anything placed into here will only
# affect this world. If you don't put anything in this file, then the
# settings will be inherited from the main configuration file.
#
# If you see {} below, that means that there are NO entries in this file.
# Remove the {} and add your own entries.
#
`

// Manager is the global configuration and the per-world ones.
// WorldGuard: ConfigurationManager, YamlConfigurationManager,
// BukkitConfigurationManager
type Manager struct {
	DataDir string
	Log     *slog.Logger
	// OnWorldLoad runs after a world's configuration is read, to load what
	// depends on it (the blacklist).
	OnWorldLoad func(w *World)

	file *yamlfile.File

	mu     sync.Mutex
	worlds map[string]*World

	MigrateRegionsToUUID       bool
	KeepUnresolvedNames        bool
	UseRegionsCreatureSpawn    bool
	UseRegionsPreCreatureSpawn bool
	DisableDefaultBypass       bool
	AnnounceBypassStatus       bool
	UseGodPermission           bool
	UseGodGroup                bool
	UseAmphibiousGroup         bool
	UsePlayerMove              bool
	UsePlayerTeleports         bool
	ParticleEffects            bool
	DisablePermissionCache     bool
	DeopOnJoin                 bool
	BlockInGameOp              bool
	HostKeys                   map[string]string
	HostKeysAllowFMLClients    bool
	UseSQL                     bool
	// ActivityHaltToggle is /halt-activity's switch, not saved.
	ActivityHaltToggle bool
}

// WorldsDir answers the folder of the per-world files.
func (m *Manager) WorldsDir() string { return filepath.Join(m.DataDir, "worlds") }

// copyDefault writes a default file to path when it does not exist.
func copyDefault(path, name string) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, Default(name), 0o644)
	}
}

// Load reads config.yml and the configs of worlds, then saves config.yml
// with every default filled in.
func (m *Manager) Load(worlds []string) error {
	path := filepath.Join(m.DataDir, "config.yml")
	copyDefault(path, "config.yml")
	f, err := yamlfile.Load(path, true)
	if err != nil {
		m.Log.Error("Error reading configuration for global config: " + err.Error())
	}
	m.file = f
	f.Header = header
	f.Remove("suppress-tick-sync-warnings")
	m.MigrateRegionsToUUID = f.Bool("regions.uuid-migration.perform-on-next-start", true)
	m.KeepUnresolvedNames = f.Bool("regions.uuid-migration.keep-names-that-lack-uuids", true)
	m.UseRegionsCreatureSpawn = f.Bool("regions.use-creature-spawn-event", true)
	m.UseRegionsPreCreatureSpawn = f.Bool("regions.use-pre-creature-spawn-event", false)
	m.DisableDefaultBypass = f.Bool("regions.disable-bypass-by-default", false)
	m.AnnounceBypassStatus = f.Bool("regions.announce-bypass-status", false)
	m.UseGodPermission = f.Bool("auto-invincible", f.Bool("auto-invincible-permission", false))
	m.UseGodGroup = f.Bool("auto-invincible-group", false)
	m.UseAmphibiousGroup = f.Bool("auto-no-drowning-group", false)
	f.Remove("auto-invincible-permission")
	m.UsePlayerMove = f.Bool("use-player-move-event", true)
	m.UsePlayerTeleports = f.Bool("use-player-teleports", true)
	m.ParticleEffects = f.Bool("use-particle-effects", true)
	m.DisablePermissionCache = f.Bool("disable-permission-cache", false)
	m.DeopOnJoin = f.Bool("security.deop-everyone-on-join", false)
	m.BlockInGameOp = f.Bool("security.block-in-game-op-command", false)
	m.HostKeys = map[string]string{}
	if raw, ok := f.Get("host-keys"); ok {
		if keys, isMap := raw.(map[string]any); isMap {
			for k, v := range keys {
				m.HostKeys[strings.ToLower(k)] = yamlfile.Stringify(v)
			}
		} else {
			f.Set("host-keys", map[string]any{})
		}
	} else {
		f.Set("host-keys", map[string]any{})
	}
	m.HostKeysAllowFMLClients = f.Bool("security.host-keys-allow-forge-clients", false)
	m.UseSQL = f.Bool("regions.sql.use", false)
	f.String("regions.sql.dsn", "jdbc:mysql://localhost/worldguard")
	f.String("regions.sql.username", "worldguard")
	f.String("regions.sql.password", "worldguard")
	f.String("regions.sql.table-prefix", "")
	if !m.UseSQL {
		f.Remove("regions.sql")
	} else {
		m.Log.Warn("SQL region storage is not available in this port; regions are kept in YAML files.")
	}
	m.mu.Lock()
	m.worlds = map[string]*World{}
	m.mu.Unlock()
	for _, w := range worlds {
		m.Get(w)
	}
	return f.Save()
}

// DisableUUIDMigration turns the region UUID migration off for later
// starts.
func (m *Manager) DisableUUIDMigration() {
	m.file.Set("regions.uuid-migration.perform-on-next-start", false)
	if err := m.file.Save(); err != nil {
		m.Log.Error("Error saving configuration!")
	}
}

// Unload forgets the per-world configurations.
func (m *Manager) Unload() {
	m.mu.Lock()
	m.worlds = map[string]*World{}
	m.mu.Unlock()
}

// Get answers the configuration of the world named name, reading it on
// first use.
func (m *Manager) Get(name string) *World {
	m.mu.Lock()
	w, ok := m.worlds[name]
	m.mu.Unlock()
	if ok {
		return w
	}
	w = m.loadWorld(name)
	m.mu.Lock()
	if existing, ok := m.worlds[name]; ok {
		w = existing
	} else {
		m.worlds[name] = w
	}
	m.mu.Unlock()
	return w
}

// Worlds answers the configurations read so far.
func (m *Manager) Worlds() []*World {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*World, 0, len(m.worlds))
	for _, w := range m.worlds {
		out = append(out, w)
	}
	return out
}

// MaxRegionCount answers how many regions a player in groups may own:
// the largest group limit, else the default.
func (w *World) MaxRegionCount(groups []string) int {
	limit := -1
	for _, g := range groups {
		if v, ok := w.maxRegionCounts[g]; ok && v > limit {
			limit = v
		}
	}
	if limit <= -1 {
		return w.MaxRegionCountPerPlayer
	}
	return limit
}

// parseBlocks reads block names and legacy "id:data" numbers, warning of
// the rest. WorldGuard: WorldConfiguration.convertLegacyBlocks
func parseBlocks(log *slog.Logger, names []string) map[registry.Block]bool {
	out := map[registry.Block]bool{}
	for _, n := range names {
		if b, ok := ParseBlock(n); ok {
			out[b] = true
		} else if log != nil {
			log.Warn(fmt.Sprintf("Unknown block type '%s'", n))
		}
	}
	return out
}

// ParseBlock reads a block name ("stone", "minecraft:stone") or a legacy
// "id[:data]" number.
func ParseBlock(s string) (registry.Block, bool) {
	s = strings.TrimSpace(s)
	if st, ok := block.LegacyBlockState(s); ok {
		return st.Block(), true
	}
	if !strings.Contains(s, ":") {
		s = "minecraft:" + s
	}
	return registry.BlockByName(strings.ToLower(s))
}

// ParseItem reads an item name or a legacy "id[:data]" number.
// WorldGuard: WorldConfiguration.convertLegacyItem
func ParseItem(s string) (registry.Item, bool) {
	s = strings.TrimSpace(s)
	if st, ok := block.LegacyBlockState(s); ok {
		if it, ok := st.Block().Item(); ok {
			return it, true
		}
	}
	if !strings.Contains(s, ":") {
		s = "minecraft:" + s
	}
	return registry.ItemByName(strings.ToLower(s))
}

// EntityType reads an entity type name.
func EntityType(s string) (registry.EntityType, bool) {
	v, ok := worldguard.EntityTypes.Lookup(strings.ToLower(s))
	if !ok {
		return 0, false
	}
	return v.(registry.EntityType), true
}

// Effect is a potion effect type named in gameplay.block-potions, resolved
// by the plugin.
type Effect = plugin.Effect
