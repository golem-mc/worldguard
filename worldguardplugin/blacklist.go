package worldguardplugin

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/worldguard/event"
	"github.com/golem-mc/worldguard/internal/blacklist"
	"github.com/golem-mc/worldguard/internal/config"
)

// openBlacklists holds each world folder's loaded blacklist, so that a
// reload or Disable closes its log files.
var openBlacklists sync.Map // world folder -> *blacklist.Blacklist

// registerBlacklist loads every world's blacklist.txt and checks what
// players do against it. WorldGuard: BlacklistListener
func (wg *WorldGuard) registerBlacklist() {
	wg.Config.OnWorldLoad = wg.loadBlacklist
	for _, w := range wg.Config.Worlds() {
		wg.loadBlacklist(w)
	}
	s := wg.Server
	opts := plugin.ListenOptions{IgnoreCancelled: true}
	plugin.Listen(s, wg.blacklistBreak, opts)
	plugin.Listen(s, func(e *event.PlaceBlockEvent) {
		wg.blacklistBlocks(&e.Blocks, blacklist.Place)
	}, opts)
	plugin.Listen(s, func(e *event.UseBlockEvent) {
		wg.blacklistBlocks(&e.Blocks, blacklist.Interact)
	}, opts)
	plugin.Listen(s, wg.blacklistSpawn, opts)
	plugin.Listen(s, wg.blacklistDestroy, opts)
	plugin.Listen(s, wg.blacklistUseItem, opts)
	plugin.Listen(s, func(e *plugin.PlayerDropItemEvent) {
		b, pl := wg.blacklistOf(e.Player.World()), wg.blacklistPlayer(e.Player)
		if b != nil && !b.Check(blacklist.Event{Type: blacklist.Drop, Player: pl, Pos: e.Item.Location().Pos.Block(),
			Target: blacklist.ItemTarget(e.Item.Stack().Item)}, false, false) {
			e.SetCancelled(true)
		}
	}, opts)
	plugin.Listen(s, func(e *plugin.BlockDispenseEvent) {
		b := wg.blacklistOf(e.Block.World)
		if b != nil && !b.Check(blacklist.Event{Type: blacklist.Dispense, Pos: e.Block.Pos,
			Target: blacklist.ItemTarget(e.Item.Item)}, false, false) {
			e.SetCancelled(true)
			return
		}
		// A dispenser dressing a player. WorldGuard: BlacklistListener.onBlockDispenseArmor
		if pl, ok := e.Target.(plugin.Player); ok && b != nil && isArmor(e.Item.Item) &&
			!b.Check(wg.itemEvent(blacklist.Equip, pl, e.Item), false, false) {
			e.SetCancelled(true)
		}
	}, opts)
	plugin.Listen(s, wg.blacklistClick, opts)
	plugin.Listen(s, wg.blacklistDrag, opts)
	plugin.Listen(s, func(e *plugin.PlayerItemHeldEvent) {
		b := wg.blacklistOf(e.Player.World())
		item := e.Player.Inventory().Item(e.New)
		if b != nil && !item.Empty() && !b.Check(wg.itemEvent(blacklist.Acquire, e.Player, item), false, false) {
			e.Player.Inventory().SetItem(e.New, plugin.ItemStack{})
		}
	}, opts)
}

// loadBlacklist reads a world's blacklist.txt and sets up its loggers; an
// empty blacklist is none. WorldGuard: BukkitWorldConfiguration.loadConfiguration
func (wg *WorldGuard) loadBlacklist(w *config.World) {
	if old, ok := openBlacklists.LoadAndDelete(w.Dir); ok {
		old.(*blacklist.Blacklist).Close()
	}
	w.Blacklist = nil
	f, err := os.Open(w.BlacklistFile)
	if errors.Is(err, fs.ErrNotExist) {
		wg.Log.Warn("WorldGuard blacklist does not exist.")
		return
	}
	if err != nil {
		wg.Log.Warn("Could not load WorldGuard blacklist: " + err.Error())
		return
	}
	defer f.Close()
	b, err := blacklist.Load(f, filepath.Base(w.BlacklistFile), w.UseBlacklistAsWhitelist, func(msg string) { wg.Log.Warn(msg) })
	if err != nil {
		wg.Log.Warn("Could not load WorldGuard blacklist: " + err.Error())
		return
	}
	if b.Len() == 0 {
		return
	}
	if w.SummaryOnStart {
		wg.Log.Info(fmt.Sprintf("(%s) Blacklist loaded with %d entries.", w.Name, b.Len()))
	}
	if w.LogDatabase {
		wg.Log.Warn(fmt.Sprintf("(%s) blacklist.logging.database is not supported; blacklist events are not logged to a database.", w.Name))
	}
	if w.LogConsole {
		b.Loggers = append(b.Loggers, blacklist.ConsoleLogger{World: w.Name, Logger: wg.Log})
	}
	if w.LogFile {
		b.Loggers = append(b.Loggers, &blacklist.FileLogger{Pattern: w.LogFilePattern, CacheSize: w.LogFileCacheSize, World: w.Name, Logger: wg.Log})
	}
	b.Notify = wg.notifyAll
	w.Blacklist = b
	openBlacklists.Store(w.Dir, b)
}

// closeBlacklists closes every loaded blacklist's log files.
func closeBlacklists() {
	openBlacklists.Range(func(k, v any) bool {
		v.(*blacklist.Blacklist).Close()
		openBlacklists.Delete(k)
		return true
	})
}

func (wg *WorldGuard) blacklistOf(w plugin.World) *blacklist.Blacklist {
	if w == nil {
		return nil
	}
	b, _ := wg.WorldConfig(w).Blacklist.(*blacklist.Blacklist)
	return b
}

// bannablePlayer is a player as the blacklist sees them.
type bannablePlayer struct {
	plugin.Player
	wg *WorldGuard
}

func (wg *WorldGuard) blacklistPlayer(p plugin.Player) blacklist.Player {
	if lp, ok := p.(*localPlayer); ok {
		p = lp.Player
	}
	return bannablePlayer{Player: p, wg: wg}
}

func (p bannablePlayer) Groups() []string { return p.wg.Groups(p.Player) }

func (p bannablePlayer) HasPermission(node string) bool { return p.wg.HasPermission(p.Player, node) }

// Ban bans the player with vanilla's default reason and kicks them with
// reason. WorldGuard: BukkitPlayer.ban
func (p bannablePlayer) Ban(reason plugin.Text) {
	p.wg.Server.Bans().Ban(p.UUID().String(), "Banned by an operator.", time.Time{}, "(Unknown)")
	p.Kick(reason)
}

// itemEvent is an item event at the player's feet.
func (wg *WorldGuard) itemEvent(t blacklist.EventType, p plugin.Player, item plugin.ItemStack) blacklist.Event {
	return blacklist.Event{Type: t, Player: wg.blacklistPlayer(p), Pos: p.Location().Pos.Block(), Target: blacklist.ItemTarget(item.Item)}
}

func (wg *WorldGuard) blacklistBreak(e *event.BreakBlockEvent) {
	b, pl := wg.blacklistOf(e.World), e.Cause.FirstPlayer()
	if b == nil || pl == nil {
		return
	}
	bp := wg.blacklistPlayer(pl)
	held := pl.Inventory().Item(pl.Inventory().HeldSlot())
	e.Filter(func(loc plugin.Location) bool {
		pos := loc.Pos.Block()
		return b.Check(blacklist.Event{Type: blacklist.Break, Player: bp, Pos: pos, Target: blacklist.BlockTarget(blockAtOr(e.World, pos, e.Type))}, false, false) &&
			b.Check(blacklist.Event{Type: blacklist.DestroyWith, Player: bp, Pos: pos, Target: blacklist.ItemTarget(held.Item)}, false, false)
	}, false)
}

// blacklistBlocks checks placing or using each of an event's blocks.
func (wg *WorldGuard) blacklistBlocks(e *event.Blocks, t blacklist.EventType) {
	b, pl := wg.blacklistOf(e.World), e.Cause.FirstPlayer()
	if b == nil || pl == nil {
		return
	}
	bp := wg.blacklistPlayer(pl)
	e.Filter(func(loc plugin.Location) bool {
		pos := loc.Pos.Block()
		return b.Check(blacklist.Event{Type: t, Player: bp, Pos: pos, Target: blacklist.BlockTarget(blockAtOr(e.World, pos, e.Type))}, false, false)
	}, false)
}

// blockAtOr answers the block at pos, or t where the world still holds air
// (a block about to be placed).
func blockAtOr(w plugin.World, pos plugin.BlockPos, t registry.Block) registry.Block {
	if b := w.BlockState(pos).Block(); !isAir(b) || t == 0 {
		return b
	}
	return t
}

// blacklistSpawn checks using the item that places an entity.
func (wg *WorldGuard) blacklistSpawn(e *event.SpawnEntityEvent) {
	b, pl := wg.blacklistOf(e.World), e.Cause.FirstPlayer()
	if b == nil || pl == nil {
		return
	}
	if item, ok := itemOfEntity(e.Type); ok && !b.Check(blacklist.Event{Type: blacklist.Use, Player: wg.blacklistPlayer(pl),
		Pos: e.Target.Pos.Block(), Target: blacklist.ItemTarget(item)}, false, false) {
		e.SetCancelled(true)
	}
}

// blacklistDestroy checks picking up an item and breaking an entity
// placed by an item.
func (wg *WorldGuard) blacklistDestroy(e *event.DestroyEntityEvent) {
	b, pl := wg.blacklistOf(e.World), e.Cause.FirstPlayer()
	if b == nil || pl == nil || e.Entity.Entity == nil {
		return
	}
	bp := wg.blacklistPlayer(pl)
	at := e.Entity.Entity.Location().Pos.Block()
	if item, ok := e.Entity.Entity.(plugin.ItemEntity); ok {
		if !b.Check(blacklist.Event{Type: blacklist.Acquire, Player: bp, Pos: at, Target: blacklist.ItemTarget(item.Stack().Item)}, false, true) {
			e.SetCancelled(true)
			return
		}
	}
	if item, ok := itemOfEntity(e.Entity.Entity.Type()); ok && !b.Check(blacklist.Event{Type: blacklist.Break, Player: bp,
		Pos: e.Target.Pos.Block(), Target: blacklist.ItemTarget(item)}, false, false) {
		e.SetCancelled(true)
	}
}

func (wg *WorldGuard) blacklistUseItem(e *event.UseItemEvent) {
	b, pl := wg.blacklistOf(e.World), e.Cause.FirstPlayer()
	if b == nil || pl == nil {
		return
	}
	if !b.Check(wg.itemEvent(blacklist.Use, pl, e.Item), false, false) {
		e.SetCancelled(true)
		return
	}
	if isArmor(e.Item.Item) && !b.Check(wg.itemEvent(blacklist.Equip, pl, e.Item), false, false) {
		e.SetCancelled(true)
	}
}

// isArmor reports whether an item is worn on the head, body, legs or feet:
// armour, skulls, carved pumpkins, elytra. WorldGuard: Materials.isArmor
func isArmor(i registry.Item) bool {
	eq, ok := i.Equippable()
	if !ok {
		return false
	}
	switch eq.Slot {
	case "head", "chest", "legs", "feet":
		return true
	}
	return false
}

// ownInventory reports whether a view is the player's own inventory.
func ownInventory(v plugin.InventoryView) bool {
	return v == nil || v.Top() == nil || v.Top().Type() == plugin.InvPlayer
}

// blacklistClick checks taking an item out of a container, creative
// picks, and putting armour on. WorldGuard: BlacklistListener.onInventoryClick,
// onInventoryCreative
func (wg *WorldGuard) blacklistClick(e *plugin.InventoryClickEvent) {
	b := wg.blacklistOf(e.Player.World())
	if b == nil {
		return
	}
	if e.Click == plugin.ClickCreative {
		if !e.Cursor.Empty() && !b.Check(wg.itemEvent(blacklist.Acquire, e.Player, e.Cursor), false, false) {
			e.SetCancelled(true)
		}
		return
	}
	var top plugin.Inventory
	if e.View != nil {
		top = e.View.Top()
	}
	if e.Current.Empty() || top == nil || top.Holder() == nil {
		return
	}
	if !b.Check(wg.itemEvent(blacklist.Acquire, e.Player, e.Current), false, false) {
		e.SetCancelled(true)
		if h, ok := top.Holder().(plugin.Entity); ok && h.UUID() == e.Player.UUID() {
			e.Current = plugin.ItemStack{}
		}
	}
	if item, ok := equipped(e); ok && !b.Check(wg.itemEvent(blacklist.Equip, e.Player, item), false, false) {
		e.SetCancelled(true)
	}
}

// equipped answers the item a click puts on an armour slot.
// WorldGuard: BlacklistListener.checkEquipped
func equipped(e *plugin.InventoryClickEvent) (plugin.ItemStack, bool) {
	own := e.Inventory != nil && e.Inventory.Type() == plugin.InvPlayer
	switch {
	case own && e.Slot >= 36 && e.Slot <= 39:
		switch e.Click {
		case plugin.ClickLeft, plugin.ClickRight:
			return e.Cursor, !e.Cursor.Empty()
		case plugin.ClickNumberKey:
			return e.Player.Inventory().Item(e.Button), true
		case plugin.ClickSwapOffhand:
			return e.Player.Inventory().Get(plugin.SlotOffHand), true
		}
	case own && ownInventory(e.View) && (e.Click == plugin.ClickShiftLeft || e.Click == plugin.ClickShiftRight):
		return e.Current, true
	}
	return plugin.ItemStack{}, false
}

// blacklistDrag checks dragging an item over the armour slots of the
// player's own inventory. WorldGuard: BlacklistListener.onInventoryDrag
func (wg *WorldGuard) blacklistDrag(e *plugin.InventoryDragEvent) {
	b := wg.blacklistOf(e.Player.World())
	if b == nil || !ownInventory(e.View) {
		return
	}
	for raw, item := range e.Slots {
		if raw >= 5 && raw <= 8 {
			if !b.Check(wg.itemEvent(blacklist.Equip, e.Player, item), false, false) {
				e.SetCancelled(true)
			}
			return
		}
	}
}
