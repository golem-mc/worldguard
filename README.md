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

Flags work as in WorldGuard: see WorldGuard's
[flag list](https://worldguard.enginehub.org/en/latest/regions/flags/) for every flag,
its values and its default.

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

Every option is described in WorldGuard's
[configuration guide](https://worldguard.enginehub.org/en/latest/config/).

## Moving from Bukkit or Paper

Copy `plugins/WorldGuard/config.yml` and the `plugins/WorldGuard/worlds/` folder to
`plugins/worldguard/`. The regions load as they are, flags of other plugins included;
owners and members still stored by name are turned into UUIDs on the first start, from
the players the server knows.

## Differences from WorldGuard

- Region data is kept in YAML files only; the MySQL storage is not available.

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
