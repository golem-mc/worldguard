# Porting WorldGuard to golem

This module is WorldGuard 7 (Java: `worldguard-core` and `worldguard-bukkit`) ported to
a golem plugin. The reference is upstream's source on its master branch; clone it into
`WorldGuard/` (gitignored) to compare.

## Layout

| Path | What | Java counterpart |
|---|---|---|
| `/` (package `worldguard`) | The API: flags and the flag registry, regions, domains, the flag value calculator, region managers, queries and the `API` service | `protection/`, `domains/`, `protection/flags/`, `RegionQuery` |
| `worldguardplugin/` | The `worldguard` plugin: enabling, the region container, sessions, players, migrations | `WorldGuardPlugin`, `BukkitWorldGuardPlatform`, `RegionContainer*`, `session/` |
| `worldguardplugin/commands_*.go`, `cmdbase.go` | `/region` and its sub-commands | `RegionCommands`, `MemberCommands`, `RegionCommandsBase`, `commands/task/` |
| `worldguardplugin/printout.go`, `lister.go`, `flaghelper.go` | `/rg info`, `/rg list`, `/rg flags` output | `RegionPrintoutBuilder`, `RegionLister`, `FlagHelperBox`, WorldEdit's `MessageBox`, `PaginationBox` |
| `worldguardplugin/permmodel.go` | Region command permissions | `RegionPermissionModel` |
| `worldguardplugin/profiles.go` | Player names and UUIDs typed in commands | `DomainInputResolver`, squirrelid's profile services |
| `internal/config/` | `config.yml` and `worlds/<world>/config.yml`, and the default files | `ConfigurationManager`, `YamlWorldConfiguration`, `BukkitWorldConfiguration`, `resources/defaults` |
| `internal/storage/` | `worlds/<world>/regions.yml` | `DirectoryYamlDriver`, `YamlRegionFile` |
| `internal/yamlfile/` | YAML files with dotted paths and defaults written back | WorldEdit's `YAMLProcessor` |
| `worldguardplugin/sessions.go`, `handlers.go` | Sessions and their flag handlers | `session/Session`, `session/handler/*` |
| `worldguardplugin/movement.go`, `invincibility.go` | Movement, teleport, respawn and join listeners; invincibility and player modes | `PlayerMoveListener`, `WorldGuardPlayerListener` (teleport, respawn, join), `InvincibilityListener`, `PlayerModesListener` |
| `worldguardplugin/cmdline.go`, `matcher.go` | The sk89q command style (flags, quoted words, usage errors, completion) and player and world matching | `CommandsManager`, `BukkitStringMatcher` |
| `worldguardplugin/general.go`, `toggles.go`, `wgcommands.go` | `/god` and the other general commands, the toggles, `/wg` | `GeneralCommands`, `ToggleCommands`, `WorldGuardCommands` |
| `internal/chat/` | Legacy `§` text and WorldEdit's message colours | `CommandUtils`, `Actor.print*` |
| `event/` | WorldGuard's own events (place, break and use blocks; spawn, destroy, use and damage entities; use items; disallowed PvP) and their `Cause` | `bukkit/event/`, `bukkit/cause/Cause` |
| `worldguardplugin/abstraction.go` | Turns golem's events into WorldGuard's | `EventAbstractionListener` |
| `worldguardplugin/regionprotection.go`, `buildperms.go` | Judges WorldGuard's events against regions and build permission nodes; the flags blocks and mobs obey | `RegionProtectionListener`, `BuildPermissionListener`, `RegionFlagsListener` |
| `worldguardplugin/blockrules.go`, `entityrules.go`, `playerrules.go`, `worldrules.go`, `commandfilter.go` | The world settings and the flags read on golem's own events | `WorldGuardBlockListener`, `WorldGuardEntityListener`, `WorldGuardHangingListener`, `WorldGuardPlayerListener` (chat, commands, wand, de-op), `CommandFilter`, `WorldGuardWeatherListener`, `WorldGuardWorldListener`, `WorldRulesListener`, `BlockedPotionsListener` |
| `worldguardplugin/materials.go`, `materials_table.go`, `entities.go` | Which blocks, items and entities count as what | `bukkit/util/Materials`, `Entities`, `Blocks` |
| `internal/blacklist/` | `blacklist.txt`: parsing, entries, actions, console and file loggers | `blacklist/` |
| `worldguardplugin/blacklist.go` | Loads each world's blacklist and checks WorldGuard's events and golem's inventory events against it | `BlacklistListener`, `BukkitWorldConfiguration` (blacklist loading) |
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
- **Sessions** keep upstream's handler order and its `FlagValueChangeHandler` semantics
  (a handler sees a value change only when the regions or the world change). Movement
  types that upstream only distinguishes for custom handlers (glide, swim, ride) are all
  `move`. `GameModeFlag`'s game mode change block is not ported: upstream never sets the
  value it compares against, so it never fires.
- **Titles** always use the default fade times (golem-mc/golem#159).
- **Commands** are registered one per name; when another plugin holds a name,
  `worldguard:<name>` is still registered and an info line is logged; a name is skipped
  with a warning only when registering fails completely (golem-mc/golem#160). They do not set golem's command permission, so a player
  without it gets WorldGuard's own "You don't have permission." instead of an unknown
  command, and `op-permissions` applies.
- **`/wg profile`** uses Go's CPU profiler (`runtime/pprof`) for the whole process; Java's
  per-thread sampling and its `-t` filter have no Go counterpart. `-i` is checked but the
  profiler's rate is Go's.
- **`/wg running`** always answers that nothing runs: the port has no task supervisor,
  loading and saving regions happen synchronously or on golem's scheduler.
- **Saving**: changes are saved every 30 seconds in the background and on shutdown, as
  upstream does.

- **Protection events** are defined on golem's event bus (`plugin.DefineEvent`), so other
  plugins can listen to them as to upstream's. Upstream's bulk events share the block list
  with the server event; here the positions left after listeners filter are copied back
  into the server event (explosions, tree growth). A block event whose positions all go
  is denied, as upstream's.
- **Materials** come from upstream's `Materials` table, converted to `materials_table.go`
  by name; tags golem does not know (Bukkit's own, or a newer version's) fall back to
  their members' name patterns. Bukkit's entity interfaces (`Enemy`, `Creature`,
  `Ambient`) are read from the vanilla mob category.
- **Causes golem does not give** are stood in for: a fire or lava block near an ignited
  block for fire spread and lava fire, the hopper side of an item move for its initiator,
  the mob's own block for the lightning that converts it. Primed TNT and falling blocks
  have no known origin, so they are an unknown cause. (golem-mc/golem#164)
- **Damage causes** are read from the vanilla damage type: `lava`, `cactus`,
  `out_of_world`, `in_wall` and so on; an explosion with no entity is a block explosion.
- **Host keys** are not checked, since golem does not give plugins the handshake's host
  name; a warning is logged when they are set.
- **Not ported:** sign chest protection (`ChestProtectionListener`, deprecated upstream),
  CommandBook god mode interop (`WorldGuardCommandBookListener`, `WorldGuardServerListener`),
  the cancel debugging (`DebuggingListener`), and listeners for events golem does not
  fire: potion splashes and lingering clouds, leashing, lectern books, farmland moisture,
  knockback (golem-mc/golem#166); redstone sponges and tipped arrows need golem-mc/golem#167.
- **Debouncing** keeps upstream's windows (10 seconds for mobs on blocks and pickups, 30
  for hopper moves), with a plain map instead of Guava's cache.
- **Commands** keep sk89q's rules: flags anywhere among the arguments, `-w world` value
  flags, quoted words joined, "Too few arguments." with the usage line, `/rg` alone
  answering `/rg <sub|commands>` (in declaration order; a hash set's upstream). Owners given
  to `/rg define` are the arguments after the id; upstream slices the raw line, so
  `-w world` in front of the id shifts its owners.
- **Async tasks** (`AsyncCommandBuilder`) run on a goroutine and report on the tick
  goroutine, because golem's handles may only be used there; the "(Please wait...)"
  message comes after one second. Printouts (`/rg info`, `/rg flags`) are built on the
  tick goroutine.
- **Names** typed in commands resolve among the online and cached players first
  (`OfflinePlayerNamed`), then with Mojang's bulk lookup, as upstream's
  `CombinedProfileService(BukkitPlayerService, HttpRepositoryService)`. Upstream's
  `profiles.sqlite` cache is not ported: last known names come from the server's user
  cache.
- **Permissions** resolve `a.b.*` and `*` set by the permission plugin when the node
  itself is not set, as WorldEdit's `DinnerPermsResolver` (and LuckPerms through Vault) do.
- **Tab completion** of `/rg` (sub-commands, region ids, flag names and values, `-w`
  worlds) is the port's; upstream completes only player names.
- **`/rg migratedb`** knows the `yaml` and `mysql` drivers and reports MySQL as
  unsupported, which is what upstream answers for a driver it lacks.

- **Blacklist targets** are vanilla ids; an item with a block of the same name is that
  block, as Bukkit's `Material` is. Names are translatable, and where upstream prints
  them as plain text (logs, kicks, `message`'s `%s`) the translation key shows, as on
  Paper. Headings also take old numeric block ids, through WorldEdit's legacy block
  table (WorldEdit does not export its legacy item table, so numeric ids of items that
  are not blocks are refused). The database logger (`DatabaseHandler`) is not ported; a
  warning says so when it is on. The file logger keeps upstream's month from `00`.
  Dispensers equipping armour are not checked against `on-equip` (golem-mc/golem#170).
- **Armour** for the blacklist is any item equippable on the head, chest, legs or feet,
  which covers upstream's armour tags, skulls, carved pumpkins and elytra.
- **`/wg debug`** (`debug.go`, upstream's `BukkitDebugHandler` and `CancelReport`) fires
  golem's own events through `Events.Fire` with an observer registered last at each
  priority, so it reports at which priority the cancelled state changed; golem cannot say
  which plugin changed it (golem-mc/golem#172). `-s` logs the report instead of pasting it.
  `wgCommand.group` nests a command group under a sub-command for it.

## Tests

- `calculator_test.go` is upstream's `FlagValueCalculatorTest`, translated line by line.
- `region_test.go` ports `RegionOverlapTest`, `RegionPriorityTest`,
  `HashMapIndexRemovalTest` and `RegionIntersectTest`, and checks flag parsing and
  `regions.yml` marshalling.
- `internal/storage` loads a `regions.yml` in upstream's format and saves it back.
- `worldguardplugin` runs the plugin on a golem test server (`plugintest`): breaking,
  placing, PvP, chat and command flags, the wand, build permission nodes, TNT and the
  `/rg` commands, whose tests answer Mojang lookups with a local HTTP server that knows
  no one.
- `commandfilter_test.go` is upstream's `CommandFilterTest`, translated.
