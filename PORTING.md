# Porting WorldGuard to golem

This module is WorldGuard 7 (Java: `worldguard-core` and `worldguard-bukkit`) ported to
a golem plugin. The reference is upstream's source on its master branch; clone it into
`WorldGuard/` (gitignored) to compare.

## Layout

| Path | What | Java counterpart |
|---|---|---|
| `/` (package `worldguard`) | The API: flags and the flag registry, regions, domains, the flag value calculator, region managers, queries and the `API` service | `protection/`, `domains/`, `protection/flags/`, `RegionQuery` |
| `worldguardplugin/` | The `worldguard` plugin: enabling, the region container, sessions, players, migrations | `WorldGuardPlugin`, `BukkitWorldGuardPlatform`, `RegionContainer*`, `session/` |
| `internal/config/` | `config.yml` and `worlds/<world>/config.yml`, and the default files | `ConfigurationManager`, `YamlWorldConfiguration`, `BukkitWorldConfiguration`, `resources/defaults` |
| `internal/storage/` | `worlds/<world>/regions.yml` | `DirectoryYamlDriver`, `YamlRegionFile` |
| `internal/yamlfile/` | YAML files with dotted paths and defaults written back | WorldEdit's `YAMLProcessor` |
| `cmd/server/` | A server with WorldGuard and WorldEdit built in | — |

## Decisions

- **Flag values** are Go values held as `any`: `State` for state flags (`StateNone`
  stands for Java's `null`), `bool`, `string`, `int`, `float64`, `Location` (a world by
  name), `plugin.Vec3`, `[]any` for a set (no repeats), `map[any]any` for a map flag,
  `plugin.GameMode`, `plugin.Weather` and `registry.EntityType` for the registry flags.
  `worldguard.Value[T]` reads one typed.
- **The build flag** is a `StateFlag` with upstream's `BuildFlag` overrides switched on;
  a query of it without a subject answers as for a non-member instead of throwing.
- **Region shapes are immutable**, as upstream asks; `Region.SetHeight` exists only for
  `/rg migrateheights`, before a manager holds the region.
- **Polygon intersection** uses AWT's semantics (a cuboid covers its blocks' full
  extent, a polygon its points' hull, touching edges do not overlap) with an exact
  slab test instead of `java.awt.geom.Area`.
- **The region index** is one map plus a per-chunk cache of candidate regions, cleared
  on every change; upstream's R-tree and per-block query cache are not needed at this
  size. Regions of equal priority are ordered by id where upstream's order is a hash
  set's.
- **Region and world ids** are compared lower case; upstream also NFC-normalises, which
  changes nothing for the characters region ids allow.
- **World names** are Bukkit's: the overworld is the level name, the vanilla nether and
  end `<level>_nether` and `<level>_the_end`, another `minecraft:` world its key's path
  and any other key `namespace_path`, as golem-mc/multiverse names them.
- **SQL region storage** (`regions.sql`) is not ported: it needs a MySQL driver and
  upstream deprecates it for removal. `regions.sql.use: true` logs a warning and the YAML
  files are used.
- **The UUID migration** resolves names from the players the server knows (its user
  cache) instead of Mojang's profile service.
- **Groups** come from the permission plugin through golem-mc/vault's `vault.Permission`,
  as upstream asks Vault through WorldEdit's permissions resolver.
- **Permissions** are decided as upstream's `WorldGuardPlugin.hasPermission`: operators
  have every node when `op-permissions` is on; otherwise golem's permission check, where an
  unregistered node is an operator's, as on Bukkit.
- **Files**: `config.yml`'s header is written with `\n` line ends; `custom-metrics-charts`
  is not read (there are no metrics).
- **Saving**: changes are saved every 30 seconds in the background and on shutdown, as
  upstream does.

## Tests

- `calculator_test.go` is upstream's `FlagValueCalculatorTest`, translated line by line.
- `region_test.go` ports `RegionOverlapTest`, `RegionPriorityTest`,
  `HashMapIndexRemovalTest` and `RegionIntersectTest`, and checks flag parsing and
  `regions.yml` marshalling.
- `internal/storage` loads a `regions.yml` in upstream's format and saves it back.
- `worldguardplugin` runs the plugin on a golem test server (`plugintest`).
