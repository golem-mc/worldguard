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
otherwise grant the nodes with a permission plugin. Besides the command nodes above:

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
| `regions.max-region-count-per-player.default` | `7` | how many regions a player may claim; add a line per group |
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

## Moving from Bukkit or Paper

Copy `plugins/WorldGuard/config.yml` and the `plugins/WorldGuard/worlds/` folder to
`plugins/worldguard/`. The regions load as they are, flags of other plugins included;
owners and members still stored by name are turned into UUIDs on the first start, from
the players the server knows.

## Differences from WorldGuard

- Region data is kept in YAML files only; the MySQL storage is not available.
- `/wg profile` writes a Go CPU profile (`profile.pprof`, read with `go tool pprof`) of the
  whole server instead of a text report of Java threads; `-i` and `-t` change nothing.
- `/wg report -p` and `/wg profile -p` do not paste to EngineHub's paste service.
- Greeting and farewell titles always use the default fade times.
- When another plugin, such as Essentials, already has `/god`, `/heal` or another general
  command, WorldGuard's is not registered.
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
