# WorldGuard for golem

An unofficial port of [WorldGuard](https://github.com/EngineHub/WorldGuard) to
[golem](https://github.com/golem-mc/golem), the Minecraft: Java Edition server written in Go.
It protects areas of your worlds with regions and flags, and reads the same `regions.yml`
and `config.yml` as WorldGuard on Bukkit or Paper. It is not made or endorsed by EngineHub.

## Installing

```sh
golem-launcher plugin add github.com/golem-mc/worldguard/worldguardplugin
```

WorldGuard needs [WorldEdit](https://github.com/golem-mc/worldedit) for selecting areas;
the line above installs it too. For permission groups in region membership (`g:admins`),
also install a permission plugin such as [LuckPerms](https://github.com/golem-mc/perms)
with [Vault](https://github.com/golem-mc/vault).

## Getting started

1. Select an area with WorldEdit: `//wand` gives you the wand (a wooden axe); left-click
   one corner and right-click the opposite one. For a polygon, `//sel poly` first, then
   right-click each further corner.
2. Make the selection a region: `/rg define shop`. A player without
   `worldguard.region.define` can claim one instead with `/rg claim shop`, which makes
   them its owner, within the configured size and count limits.
3. Add people: `/rg addmember shop Alex`, or a permission group with
   `/rg addowner shop g:admins`.
4. Set flags: `/rg flag shop greeting Welcome to the shop!`, `/rg flag shop pvp deny`,
   or for non-members only `/rg flag shop pvp -g nonmembers deny`. `/rg flags shop`
   shows every flag with clickable values.
5. Check the result with `/rg info shop`, or `/rg info` where you stand.

## Regions

A region is an area of a world with an id, a priority, owners, members and flags:

- **cuboid** and **polygonal** regions cover the blocks you select; polygonal regions
  extend between two heights;
- each world has a **global region**, `__global__`, that covers the whole world and is
  overridden by every other region;
- where regions overlap, the one with the higher **priority** decides; a region may
  have a **parent** whose owners, members and flags it inherits;
- **owners** and **members** are players or permission groups; by default only they
  may build in the region.

### Flags players notice

As players walk, ride or teleport across region boundaries, these flags act on them:

| Flag | What the player sees |
|---|---|
| `greeting`, `farewell` | a chat message on entering or leaving the region (`&` colour codes, `\n` for a new line, `%name%`, `%world%`, `%online%`, `%health%`, `%id%`) |
| `greeting-title`, `farewell-title` | the same as a title; text after `\n` is the subtitle |
| `entry`, `exit` | `deny` stops players from walking or teleporting in or out, with `entry-deny-message` or `exit-deny-message`; `exit-via-teleport` and `exit-override` loosen `exit` |
| `enderpearl`, `chorus-fruit-teleport` | `deny` stops these teleports into or out of the region |
| `notify-enter`, `notify-leave` | players with `worldguard.notify` are told who entered or left |
| `heal-amount`, `heal-delay`, `heal-min-health`, `heal-max-health` | heal (or, with a negative amount, hurt) players every `heal-delay` seconds |
| `feed-amount`, `feed-delay`, `feed-min-hunger`, `feed-max-hunger` | the same for hunger |
| `game-mode` | players switch to this game mode inside, and back on leaving |
| `time-lock`, `weather-lock` | players see this time (`6000`, or `+1000` relative to the world) or weather (`clear`, `rain`, `thunder_storm`) inside |
| `invincible` | `allow` makes players take no damage; `deny` overrides `/god` |
| `spawn` | players who die inside respawn here |

Players with `worldguard.region.bypass.<world>` (operators, by default) are not stopped by
`entry` and `exit` and keep their game mode.

### What regions protect

Inside a region, a player who is neither owner nor member cannot break or place blocks,
open doors and containers, use buttons, levers, beds and anvils, place or break vehicles,
paintings, item frames and armour stands, hurt animals, ride mounts, pick up or drop items
in the region, or fight other players. Fire spread, TNT, creepers, endermen and pistons
from outside the region cannot change it either. Monsters are not protected, nor is
anything outside regions. A denied player is told `Hey! Sorry, but you can't
break that block here.` (the `deny-message` flag).

Setting a flag on a region changes this for everyone in it: `block-break allow` lets
anyone break blocks, `pvp deny` stops fights even between members, `mob-spawning deny`
keeps mobs from spawning, `send-chat deny` mutes the region, `blocked-cmds /home` blocks
a command there.

Flags work as in WorldGuard: see WorldGuard's
[flag list](https://worldguard.enginehub.org/en/latest/regions/flags/) for every flag,
its values and its default.

## Commands

### /region

`/region` has the aliases `/regions` and `/rg`. Most commands take `-w <world>` to
work on another world than yours (needed from the console); `<id>` is a region id.

| Usage | What it does | Permission | Aliases |
|---|---|---|---|
| `/rg define [-n] [-g] <id> [owners...]` | Makes a region of your selection, owned by the players and `g:groups` listed; `-g` makes a global region with no area, `-n` stores owners by name | `worldguard.region.define` | `def`, `d`, `create` |
| `/rg redefine [-g] <id>` | Gives a region the area of your selection, keeping everything else | `worldguard.region.redefine.*` | `update`, `move` |
| `/rg claim <id>` | Makes a region of your selection owned by you, if it is small enough and overlaps only regions you own | `worldguard.region.claim` | |
| `/rg select [id]` | Makes the region (or the one you stand in) your WorldEdit selection | `worldguard.region.select.*` | `sel`, `s` |
| `/rg info [-u] [-s] [id]` | Shows a region's type, priority, flags, parents, owners, members and bounds; `-u` shows UUIDs instead of names, `-s` also selects it | `worldguard.region.info.*` | `i` |
| `/rg list [-p owner [-n]] [-s] [-i filter] [page]` | Lists the regions, or those `owner` owns or is a member of, those overlapping your selection (`-s`), those whose id contains `filter` | `worldguard.region.list`, `worldguard.region.list.own` | |
| `/rg flag [-g group] [-e] [-h page] <id> <flag> [value]` | Sets a flag, or removes it without a value; `-g` sets who it applies to (`members`, `owners`, `nonmembers`, `nonowners`, `everyone`), `-e` sets an empty value | `worldguard.region.flag.regions.*`, `worldguard.region.flag.flags.<flag>.*` | `f` |
| `/rg flags [-p page] [id]` | Shows every flag of a region with clickable values | `worldguard.region.info.*` | |
| `/rg setpriority <id> <priority>` | Sets the priority; the higher one wins where regions overlap | `worldguard.region.setpriority.*` | `priority`, `pri` |
| `/rg setparent <id> [parent]` | Sets or, without `parent`, clears the parent | `worldguard.region.setparent.*` | `parent`, `par` |
| `/rg remove [-f] [-u] <id>` | Removes a region; one with children needs `-f` (remove them too) or `-u` (keep them without a parent) | `worldguard.region.remove.*` | `delete`, `del`, `rem` |
| `/rg addmember [-n] <id> <players...>` | Adds members: player names, UUIDs or `g:group` | `worldguard.region.addmember.*` | `addmem`, `am` |
| `/rg addowner [-n] <id> <players...>` | Adds owners | `worldguard.region.addowner.*` | `ao` |
| `/rg removemember [-a] <id> [players...]` | Removes members, or all with `-a` | `worldguard.region.removemember.*` | `remmember`, `removemem`, `remmem`, `rm` |
| `/rg removeowner [-a] <id> [players...]` | Removes owners, or all with `-a` | `worldguard.region.removeowner.*` | `remowner`, `ro` |
| `/rg teleport [-s] [-c] <id>` | Teleports you to the region's `teleport` flag, its `spawn` flag with `-s`, or its center with `-c` (spectator mode only) | `worldguard.region.teleport.*`, `worldguard.region.teleportcenter.*` | `tp` |
| `/rg toggle-bypass [on\|off]` | Turns your region bypass off or on again | `worldguard.region.toggle-bypass` | `bypass` |
| `/rg load` | Reloads the regions of your world (or `-w`), or of every world from the console | `worldguard.region.load` | `reload` |
| `/rg save` | Saves the regions now | `worldguard.region.save` | `write` |
| `/rg migrateuuid` | Turns owners and members stored by name into UUIDs | `worldguard.region.migrateuuid` | |
| `/rg migrateheights -y [-w world]` | Stretches regions spanning the old 0 to 255 height to the world's full height | `worldguard.region.migrateheights` | |

Region ids may contain letters, digits and `_ , ' - + /`.

### Player commands

| Command | What it does | Permission |
|---|---|---|
| `/god [-s] [player]` | makes you, or the players named, take no damage | `worldguard.god`, `worldguard.god.other` |
| `/ungod [-s] [player]` | ends god mode | `worldguard.god`, `worldguard.god.other` |
| `/heal [-s] [player]` | restores health and hunger | `worldguard.heal`, `worldguard.heal.other` |
| `/slay [-s] [player]` | kills you, or the players named | `worldguard.slay`, `worldguard.slay.other` |
| `/locate [player]` | points your compass at a player, or back at spawn | `worldguard.locate` |
| `/stack` (alias `/;`) | merges the partial stacks in your inventory | `worldguard.stack` |

`player` is a name or its start, `@name` for an exact name, `*part` for names containing
`part`, `*` for everyone, `#world` for your world or `#near` for players within 30 blocks.
`-s` keeps the targets from being told.

### Toggles

| Command | What it does | Permission |
|---|---|---|
| `/stopfire [world]` | stops all fire spread in your world or the one named, until a restart or reload | `worldguard.fire-toggle.stop` |
| `/allowfire [world]` | lets fire spread again | `worldguard.fire-toggle.stop` |
| `/halt-activity [-c] [-i] [-s] [confirm]` (aliases `/stoplag`, `/haltactivity`) | removes items, TNT, falling blocks and untamed mobs from every loaded world and keeps them out; `-c` ends it, `-i` tells whether it is on, `-s` stays quiet | `worldguard.halt-activity` |

### /wg

| Command | What it does | Permission |
|---|---|---|
| `/wg version` | shows the version | none |
| `/wg reload` | reads the configuration and every world's regions again | `worldguard.reload` |
| `/wg report` | writes `plugins/worldguard/report.txt` about the server, the configuration and your regions | `worldguard.report` |
| `/wg profile [minutes]` | records the server's CPU use for 1 to 10 minutes (default 5) into `plugins/worldguard/profile.pprof` | `worldguard.profile` |
| `/wg stopprofile` | cancels a running profile | `worldguard.profile` |
| `/wg flushstates [player]` (alias `clearstates`) | applies the region flags afresh to everyone, or one player | `worldguard.flushstates` |
| `/wg running` (alias `queue`) | lists running tasks | `worldguard.running` |

`/worldguard` is the long form of `/wg`.

## Permissions

Operators have every WorldGuard permission while `op-permissions` is `true` (the default);
otherwise grant the nodes with a permission plugin.

Region permissions ending in `.*` above are per region, with three forms that a
permission plugin's wildcards make useful:

- `worldguard.region.<command>.own.<id>` for a region's owners,
- `worldguard.region.<command>.member.<id>` for its members (and owners),
- `worldguard.region.<command>.<id>` for anyone.

For example `worldguard.region.flag.regions.own.*` lets owners set flags on their own
regions, and `worldguard.region.flag.flags.pvp.*` lets them set the `pvp` flag;
`worldguard.region.flag.flags.<flag>.<value>.*` limits a flag to one value (`unset` for
removing it). Besides the command nodes above:

| Node | What it gives |
|---|---|
| `worldguard.region.bypass.<world>` | build and use everything in every region of the world, and ignore the entry and exit flags there |
| `worldguard.region.wand` | right-clicking a block with the region wand (leather) lists the regions there and whether you may build |
| `worldguard.notify` | hear `notify-enter` and `notify-leave`, `/stopfire` and `/halt-activity` notices |
| `worldguard.god.override-regions` | keep god mode in regions with `invincible` set to `deny` |
| `worldguard.auto-invincible` | god mode on joining, when `auto-invincible` is `true` |
| `worldguard.build.block.<block>.place`, `.remove`, `.interact` | with `build-permission-nodes.enable`, placing, breaking and using that block (`worldguard.build.block.place.<block>` works too) |
| `worldguard.build.entity.<entity>.place`, `.remove`, `.interact`, `.damage` | the same for entities, with `build-permission-nodes.enable` |
| `worldguard.build.item.<item>.use` | using that item, with `build-permission-nodes.enable` |
| `worldguard.override.lighter` | lighting fires where `ignition.block-lighter` is on |
| `worldguard.override.potions` | drinking and throwing the potions `gameplay.block-potions` names |
| `worldguard.override.infinite-stack` | keeping stacks of negative size where `protection.remove-infinite-stacks` is on |
| `worldguard.region.unlimited` | claim regions of any size and number |
| `worldguard.region.addowner.unclaimed.<id>` | add the first owner to a region nobody owns |
| `worldguard.region.locationoverride.*` | set `teleport` and `spawn` outside the region when `regions.location-flags-only-inside-regions` is on |

Block, entity and item names in `worldguard.build.*` are vanilla's, without
`minecraft:`: `worldguard.build.block.oak_door.interact`.

## Configuration

WorldGuard keeps its files in `plugins/worldguard/`:

| File | What it holds |
|---|---|
| `config.yml` | settings for every world; written with every option on the first start |
| `worlds/<world>/config.yml` | the same settings for one world, overriding `config.yml` |
| `worlds/<world>/regions.yml` | the world's regions |
| `worlds/<world>/blacklist.txt` | the world's blacklist |

Worlds are named as on Bukkit: the overworld by `level-name` (`world`), then
`world_nether` and `world_the_end`. The main options of `config.yml`:

| Option | Default | What it does |
|---|---|---|
| `regions.enable` | `true` | turns regions on in a world |
| `regions.max-claim-volume` | `30000` | the largest region `/rg claim` makes, in blocks |
| `regions.max-region-count-per-player.default` | `7` | how many regions a player may own for `/rg claim`; add a line per group |
| `regions.claim-only-inside-existing-regions` | `false` | `/rg claim` works only inside regions the player owns |
| `regions.set-parent-on-claim` | `''` | the parent a claimed region gets |
| `regions.location-flags-only-inside-regions` | `false` | `teleport` and `spawn` must lie in the region |
| `regions.disable-bypass-by-default` | `false` | operators must turn region bypass on |
| `op-permissions` | `true` | operators have every WorldGuard permission |
| `use-player-move-event` | `true` | apply the region flags as players walk and ride; `false` leaves only teleports |
| `use-player-teleports` | `true` | apply the region flags to teleports |
| `auto-invincible` | `false` | god mode on joining for players with `worldguard.auto-invincible` |
| `auto-invincible-group` | `false` | god mode on joining for players in the `wg-invincible` group |
| `auto-no-drowning-group` | `false` | no drowning for players in the `wg-amphibious` group |
| `regions.invincibility-removes-mobs` | `false` | a mob that hits an invincible player is removed |
| `regions.titles-always-use-default-times` | `true` | greeting and farewell titles fade over the default times |
| `build-permission-nodes.enable` | `false` | players need `worldguard.build.*` for what they build, break and use |
| `regions.wand` | `minecraft:leather` | the item that shows the regions at a block |
| `regions.high-frequency-flags` | `false` | checks the flags that need many queries: `fire-spread`, `lava-fire`, `water-flow`, `lava-flow` |
| `regions.protect-against-liquid-flow` | `false` | water and lava from outside a region cannot flow into it |
| `regions.explosion-flags-block-entity-damage` | `true` | where a blast may not break blocks, it does not hurt entities either |
| `ignition.block-tnt`, `ignition.block-tnt-block-damage` | `false` | TNT does not explode, or explodes without breaking blocks |
| `ignition.block-lighter` | `false` | flint and steel and fire charges do nothing |
| `fire.disable-all-fire-spread`, `fire.disable-lava-fire-spread` | `false` | fire does not spread, or lava does not set fires |
| `mobs.block-creeper-explosions`, `mobs.block-creeper-block-damage` | `false` | creepers do not explode, or do not break blocks; `mobs.*` has the same for withers, fireballs, the dragon and endermen |
| `mobs.block-creature-spawn` | `[]` | mobs that never spawn: `[minecraft:zombie, minecraft:phantom]` |
| `player-damage.disable-fall-damage` | `false` | players take no fall damage; `player-damage.*` covers lava, fire, drowning, void, mobs and more |
| `weather.disable-weather`, `weather.always-raining` | `false` | the world never rains, or always does |
| `dynamics.disable-leaf-decay` | `false` | leaves never decay; `dynamics.*` covers ice, snow, grass, vines, crops and copper |
| `protection.item-durability` | `true` | tools wear out; `false` keeps them whole when breaking blocks |
| `gameplay.block-potions` | `[]` | potion effects that are refused: `[speed, invisibility]` |
| `use-particle-effects` | `true` | a puff of smoke where something was denied |

Every option is described in WorldGuard's
[configuration guide](https://worldguard.enginehub.org/en/latest/config/).

## Blacklist

Each world's `worlds/<world>/blacklist.txt` lists blocks and items, and what happens when
a player does something with them. A heading in brackets names one or more blocks or
items (vanilla names, with or without `minecraft:`; old numeric block ids such as `35:14`
work too), and the lines under it say what to do:

```
# Players may not mine diamond ore; tell them, log it and tell the staff
[diamond_ore, deepslate_diamond_ore]
on-break=deny,tell,log,notify
comment=Diamond mining

# No lava buckets, except for admins and moderators
[lava_bucket]
on-use=deny,tell
ignore-groups=admins,mods
message=&cLava buckets are not allowed here.
```

| Event | When |
|---|---|
| `on-break` | the block is broken |
| `on-destroy-with` | a block is broken while holding the item |
| `on-place` | the block is placed |
| `on-use` | the item is used (buckets, flint and steel, spawn eggs, …) |
| `on-interact` | the block is used (doors, chests, buttons, …) |
| `on-drop` | the item is dropped |
| `on-acquire` | the item is picked up or taken out of a container |
| `on-equip` | the item is put on as armour |
| `on-dispense` | a dispenser shoots the item |

| Action | What it does |
|---|---|
| `deny` | stops it |
| `allow` | lets it happen (for `blacklist.use-as-whitelist`) |
| `tell` | tells the player it is not allowed |
| `notify` | tells players with `worldguard.notify` |
| `log` | logs it to the console or a file |
| `kick`, `ban` | kicks or bans the player |

| Option | What it does |
|---|---|
| `ignore-groups` | groups the entry does not apply to, comma-separated |
| `ignore-perms` | permissions whose holders the entry does not apply to |
| `message` | what `tell`, `kick` and `ban` say instead; `%s` is the block or item, `&` colour codes work |
| `comment` | a note `log` and `notify` add |

`tell`, `notify` and `log` stay quiet when the same player does the same thing again
within three seconds. The blacklist's settings in `config.yml`:

| Option | Default | What it does |
|---|---|---|
| `blacklist.use-as-whitelist` | `false` | everything listed is denied unless an `allow` action lets it |
| `blacklist.logging.console.enable` | `true` | `log` writes to the server log |
| `blacklist.logging.file.enable` | `false` | `log` writes to files |
| `blacklist.logging.file.path` | `worldguard/logs/%Y-%m-%d.log` | the files, relative to the server folder: `%Y` year, `%m` month (from `00`), `%d` day, `%W` week, `%H` hour, `%i` minute, `%s` second, `%u` player, `%w` world |
| `blacklist.logging.file.open-files` | `10` | how many log files stay open at once |

`/wg reload` reads the blacklists again.

## Moving from Bukkit or Paper

Copy `plugins/WorldGuard/config.yml` and the `plugins/WorldGuard/worlds/` folder to
`plugins/worldguard/`. The regions load as they are, flags of other plugins included;
owners and members still stored by name are turned into UUIDs on the first start, from
the players the server knows. Commands that take player names look them up among the
players the server knows, then with Mojang, as WorldGuard does.

## Differences from WorldGuard

- Region data is kept in YAML files only; the MySQL storage is not available, and
  `/rg migratedb` says so.
- `/rg` completes sub-commands, region ids, flag names and flag values with Tab.
- `/wg profile` writes a Go CPU profile (`profile.pprof`, read with `go tool pprof`) of the
  whole server instead of a text report of Java threads; `-i` and `-t` change nothing.
- `/wg report -p` and `/wg profile -p` do not paste to EngineHub's paste service.
- Greeting and farewell titles always use the default fade times.
- When another plugin, such as Essentials, already has `/god`, `/heal` or another general
  command, WorldGuard's is not registered.
- The blacklist cannot log to a database: `blacklist.logging.database` is ignored, with a
  warning when it is on.
- `chest-protection.enable` (protecting chests with `[Lock]` signs) is not available;
  WorldGuard itself deprecated it in favour of regions.

## For plugin developers

```sh
go get github.com/golem-mc/worldguard
```

Package `github.com/golem-mc/worldguard` is WorldGuard's API: regions, flags and the
queries over them. Importing it registers no plugin, so your plugin works with or
without WorldGuard installed. Look the service up when you need it:

```go
// canFight reports whether attacker may hurt someone at loc.
func canFight(s plugin.Server, attacker plugin.Player, loc plugin.Location) bool {
	wg, ok := worldguard.Lookup(s)
	if !ok {
		return true // WorldGuard is not installed
	}
	who := wg.WrapPlayer(attacker)
	if wg.HasBypass(attacker, loc.World) {
		return true
	}
	return wg.Query().QueryState(loc, who, worldguard.FlagPVP) != worldguard.StateDeny
}
```

The regions at a place, and a flag's value there:

```go
// regionsAt lists the ids of the regions at loc, highest priority first.
func regionsAt(s plugin.Server, loc plugin.Location) []string {
	wg, ok := worldguard.Lookup(s)
	if !ok {
		return nil
	}
	var ids []string
	for _, r := range wg.Query().ApplicableRegions(loc).Regions() {
		ids = append(ids, r.ID())
	}
	return ids
}
```

```go
// greetingAt answers the greeting flag's value at loc for p.
func greetingAt(s plugin.Server, p plugin.Player, loc plugin.Location) string {
	wg, ok := worldguard.Lookup(s)
	if !ok {
		return ""
	}
	set := wg.Query().ApplicableRegions(loc)
	greeting, _ := worldguard.Value[string](set, wg.WrapPlayer(p), worldguard.FlagGreeting)
	return greeting
}
```

A custom flag is registered before WorldGuard loads the regions, from an `init`
function or your plugin's `Load`; it can then be set with `/rg flag` and is saved in
`regions.yml`:

```go
// FlagAllowFlight is a custom flag: "/rg flag <region> allow-flight deny".
var FlagAllowFlight = worldguard.NewStateFlag("allow-flight", true)

func init() {
	// Register flags before WorldGuard loads the regions: in init or Load.
	if err := worldguard.Flags.Register(FlagAllowFlight); err != nil {
		panic(err)
	}
}

func mayFly(s plugin.Server, p plugin.Player) bool {
	wg, ok := worldguard.Lookup(s)
	if !ok {
		return true
	}
	return wg.Query().TestState(p.Location(), wg.WrapPlayer(p), FlagAllowFlight)
}
```

Regions can be made and changed through a world's region manager:

```go
// claimPlot makes a region owned by p.
func claimPlot(s plugin.Server, p plugin.Player, id string, a, b plugin.BlockPos) error {
	wg, ok := worldguard.Lookup(s)
	if !ok {
		return nil
	}
	regions, ok := wg.RegionManager(p.World())
	if !ok {
		return nil // regions are off in this world
	}
	plot, err := worldguard.NewCuboidRegion(id, a, b)
	if err != nil {
		return err
	}
	plot.Owners().AddPlayer(p.UUID())
	plot.SetFlag(worldguard.FlagGreeting, "Welcome to "+p.Name()+"'s plot")
	regions.AddRegion(plot) // saved in the background
	return nil
}
```

Flag types (`NewStateFlag`, `NewStringFlag`, `NewIntegerFlag`, `NewSetFlag`,
`NewLocationFlag`, ...) and the query methods follow WorldGuard's Java API; see
`go doc github.com/golem-mc/worldguard`.

## License

Like WorldGuard, this port is licensed under the GNU Lesser General Public License v3
([LICENSE](LICENSE), which builds on the GNU General Public License v3 in [COPYING](COPYING)).
