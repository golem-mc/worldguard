package worldguardplugin

import (
	"strconv"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/worldguard"
)

// registerMovement listens to the moves that cross region boundaries.
// WorldGuard: PlayerMoveListener, WorldGuardPlayerListener
func (m *sessionManager) registerMovement() {
	wg := m.wg
	s := wg.Server
	plugin.Listen(s, wg.onTeleport, plugin.ListenOptions{Priority: plugin.Low, IgnoreCancelled: true})
	plugin.Listen(s, wg.onRespawnSpawnFlag, plugin.ListenOptions{Priority: plugin.Highest})
	if !wg.Config.UsePlayerMove {
		return
	}
	plugin.Listen(s, wg.onMove, plugin.ListenOptions{Priority: plugin.High})
	plugin.Listen(s, func(e *plugin.PlayerRespawnEvent) {
		m.get(e.Player).testMoveTo(wg.local(e.Player), e.Location, moveRespawn, true)
	}, plugin.ListenOptions{Priority: plugin.Monitor})
	plugin.Listen(s, func(e *plugin.EntityMountEvent) {
		p, ok := e.Entity.(plugin.Player)
		if !ok {
			return
		}
		if m.get(p).testMoveTo(wg.local(p), e.Vehicle.Location(), moveEmbark, true) != nil {
			e.SetCancelled(true)
		}
	})
}

// onMove sends a player back to the last place they were allowed to be
// when the session refuses the block they move into. WorldGuard:
// PlayerMoveListener.onPlayerMove
func (wg *WorldGuard) onMove(e *plugin.PlayerMoveEvent) {
	if e.From.Pos.Block() == e.To.Pos.Block() && e.From.World == e.To.World {
		return
	}
	p := e.Player
	back := wg.sessions.get(p).testMoveTo(wg.local(p), e.To, moveMove, false)
	if back == nil {
		return
	}
	b := back.Pos.Block()
	override := plugin.Location{World: back.World, Yaw: e.To.Yaw, Pitch: e.To.Pitch,
		Pos: plugin.Vec3{X: float64(b.X) + 0.5, Y: float64(b.Y), Z: float64(b.Z) + 0.5}}
	e.To = override
	vehicle, ok := p.Vehicle()
	if !ok {
		return
	}
	// Riding out of a region the player may not leave: unseat everyone and
	// put the vehicles and the player back.
	above := override
	above.Pos.Y++
	p.Dismount()
	for cur, ok := vehicle, true; ok; cur, ok = cur.Vehicle() {
		for _, rider := range cur.Passengers() {
			cur.RemovePassenger(rider)
		}
		vehicle.SetVelocity(plugin.Vec3{})
		if _, living := vehicle.(plugin.LivingEntity); living {
			vehicle.Teleport(override, plugin.TeleportPlugin)
		} else {
			vehicle.Teleport(above, plugin.TeleportPlugin)
		}
	}
	p.Teleport(above, plugin.TeleportPlugin)
	wg.Server.Scheduler().RunLater(1, func() { p.Teleport(above, plugin.TeleportPlugin) })
}

// onTeleport applies the enderpearl and chorus-fruit-teleport flags and
// the entry and exit flags to teleports. WorldGuard:
// WorldGuardPlayerListener.onPlayerTeleport
func (wg *WorldGuard) onTeleport(e *plugin.EntityTeleportEvent) {
	p, ok := e.Entity.(plugin.Player)
	if !ok || !wg.WorldConfig(p.World()).UseRegions || !wg.Config.UsePlayerTeleports {
		return
	}
	lp := wg.local(p)
	q := wg.Query()
	var flag *worldguard.StateFlag
	switch e.Cause {
	case plugin.TeleportEnderPearl:
		flag = worldguard.FlagEnderpearl
	case plugin.TeleportChorusFruit:
		flag = worldguard.FlagChorusFruitTeleport
	}
	if flag != nil && !wg.HasBypass(p, p.World()) {
		var msg string
		var cancel bool
		if from := q.ApplicableRegions(e.From); !from.TestState(lp, flag) {
			cancel = true
			msg, _ = worldguard.Value[string](from, lp, worldguard.FlagExitDenyMessage)
		} else if to := q.ApplicableRegions(e.To); !to.TestState(lp, flag) {
			cancel = true
			msg, _ = worldguard.Value[string](to, lp, worldguard.FlagEntryDenyMessage)
		}
		if cancel {
			if msg != "" {
				sendRaw(p, msg)
			}
			e.SetCancelled(true)
			return
		}
	}
	if wg.sessions.get(p).testMoveTo(lp, e.To, moveTeleport, false) != nil {
		e.SetCancelled(true)
	}
}

// onRespawnSpawnFlag respawns players at the spawn flag of where they
// died. WorldGuard: WorldGuardPlayerListener.onPlayerRespawn
func (wg *WorldGuard) onRespawnSpawnFlag(e *plugin.PlayerRespawnEvent) {
	p := e.Player
	if !wg.WorldConfig(p.World()).UseRegions {
		return
	}
	lp := wg.local(p)
	spawn, ok := worldguard.Value[worldguard.Location](wg.Query().ApplicableRegions(p.Location()), lp, worldguard.FlagSpawn)
	if !ok {
		return
	}
	if loc, ok := wg.location(spawn); ok {
		e.Location = loc
	}
}

// location answers a flag's location in its world, false when the world
// is not loaded.
func (wg *WorldGuard) location(l worldguard.Location) (plugin.Location, bool) {
	w, ok := wg.WorldByName(l.World)
	if !ok {
		return plugin.Location{}, false
	}
	return plugin.Location{World: w, Pos: l.Pos, Yaw: l.Yaw, Pitch: l.Pitch}, true
}

// onJoin tells the player about halted activity and stopped fire, and
// starts their session. WorldGuard: WorldGuardPlayerListener.onPlayerJoin
func (wg *WorldGuard) onJoin(p plugin.Player) {
	w := p.World()
	if wg.Config.ActivityHaltToggle {
		p.SendMessage(plugin.Plain("Intensive server activity has been HALTED.").Color("yellow"))
		if removed := removeIntensiveEntities(w); removed > 10 {
			wg.Log.Info("Halt-Act: " + strconv.Itoa(removed) + " entities (>10) auto-removed from " + wg.WorldName(w))
		}
	}
	if wg.WorldConfig(w).FireSpreadDisableToggle {
		p.SendMessage(plugin.Plain("Fire spread is currently globally disabled for this world.").Color("yellow"))
	}
	wg.sessions.process(p)
}
