package worldguard

import (
	"slices"
	"sort"
	"strings"
	"sync"
)

// Domain is a region's owners or members: players by UUID, legacy players
// by name (lower case) and permission groups (lower case). It is safe for
// concurrent use. WorldGuard: DefaultDomain, PlayerDomain, GroupDomain
type Domain struct {
	mu     sync.RWMutex
	uuids  []UUID
	names  []string
	groups []string
	dirty  bool
}

// NewDomain answers an empty domain.
func NewDomain() *Domain { return &Domain{dirty: true} }

// Copy answers a copy of d.
func (d *Domain) Copy() *Domain {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return &Domain{uuids: slices.Clone(d.uuids), names: slices.Clone(d.names), groups: slices.Clone(d.groups), dirty: true}
}

func (d *Domain) change(fn func()) {
	d.mu.Lock()
	fn()
	d.dirty = true
	d.mu.Unlock()
}

func addTo[T comparable](s []T, v T) []T {
	if slices.Contains(s, v) {
		return s
	}
	return append(s, v)
}

func removeFrom[T comparable](s []T, v T) []T {
	return slices.DeleteFunc(s, func(e T) bool { return e == v })
}

// AddPlayer adds a player by UUID.
func (d *Domain) AddPlayer(id UUID) { d.change(func() { d.uuids = addTo(d.uuids, id) }) }

// RemovePlayer removes a player by UUID.
func (d *Domain) RemovePlayer(id UUID) { d.change(func() { d.uuids = removeFrom(d.uuids, id) }) }

// AddPlayerName adds a player by name, as regions from before UUIDs have
// them.
func (d *Domain) AddPlayerName(name string) {
	if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
		d.change(func() { d.names = addTo(d.names, name) })
	}
}

// RemovePlayerName removes a player by name.
func (d *Domain) RemovePlayerName(name string) {
	name = strings.ToLower(strings.TrimSpace(name))
	d.change(func() { d.names = removeFrom(d.names, name) })
}

// RemoveLocalPlayer removes p by name and by UUID.
func (d *Domain) RemoveLocalPlayer(p LocalPlayer) {
	d.RemovePlayerName(p.Name())
	d.RemovePlayer(p.UUID())
}

// AddGroup adds a permission group.
func (d *Domain) AddGroup(name string) {
	if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
		d.change(func() { d.groups = addTo(d.groups, name) })
	}
}

// RemoveGroup removes a permission group.
func (d *Domain) RemoveGroup(name string) {
	name = strings.ToLower(strings.TrimSpace(name))
	d.change(func() { d.groups = removeFrom(d.groups, name) })
}

// AddAll adds everyone in other.
func (d *Domain) AddAll(other *Domain) {
	for _, n := range other.Names() {
		d.AddPlayerName(n)
	}
	for _, id := range other.UUIDs() {
		d.AddPlayer(id)
	}
	for _, g := range other.Groups() {
		d.AddGroup(g)
	}
}

// RemoveAll removes everyone in other.
func (d *Domain) RemoveAll(other *Domain) {
	for _, n := range other.Names() {
		d.RemovePlayerName(n)
	}
	for _, id := range other.UUIDs() {
		d.RemovePlayer(id)
	}
	for _, g := range other.Groups() {
		d.RemoveGroup(g)
	}
}

// Clear removes everyone.
func (d *Domain) Clear() { d.change(func() { d.uuids, d.names, d.groups = nil, nil, nil }) }

// UUIDs, Names and Groups answer copies of the domain's entries.
func (d *Domain) UUIDs() []UUID {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return slices.Clone(d.uuids)
}

func (d *Domain) Names() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return slices.Clone(d.names)
}

func (d *Domain) Groups() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return slices.Clone(d.groups)
}

// Contains reports whether p is in the domain, by UUID, name or group.
func (d *Domain) Contains(p LocalPlayer) bool {
	d.mu.RLock()
	uuid := slices.Contains(d.uuids, p.UUID())
	name := len(d.names) > 0 && slices.Contains(d.names, strings.ToLower(strings.TrimSpace(p.Name())))
	groups := slices.Clone(d.groups)
	d.mu.RUnlock()
	if uuid || name {
		return true
	}
	return slices.ContainsFunc(groups, p.HasGroup)
}

// ContainsUUID reports whether the player with id is in the domain by UUID.
func (d *Domain) ContainsUUID(id UUID) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return slices.Contains(d.uuids, id)
}

// ContainsName reports whether a player is in the domain by name.
func (d *Domain) ContainsName(name string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return slices.Contains(d.names, strings.ToLower(strings.TrimSpace(name)))
}

// Size answers the number of entries.
func (d *Domain) Size() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.uuids) + len(d.names) + len(d.groups)
}

// Dirty reports whether the domain changed since SetDirty(false).
func (d *Domain) Dirty() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.dirty
}

// SetDirty marks the domain changed or saved.
func (d *Domain) SetDirty(v bool) {
	d.mu.Lock()
	d.dirty = v
	d.mu.Unlock()
}

// PlayersString answers the players as /rg info shows them: "name:x" for a
// name, the cached name with a star or "uuid:x" for a UUID, sorted ignoring
// case. names resolves a UUID, nil for none. WorldGuard:
// DefaultDomain.toPlayersString
func (d *Domain) PlayersString(names func(UUID) (string, bool)) string {
	var out []string
	for _, n := range d.Names() {
		out = append(out, "name:"+n)
	}
	for _, id := range d.UUIDs() {
		if names != nil {
			if n, ok := names(id); ok {
				out = append(out, n+"*")
				continue
			}
		}
		out = append(out, "uuid:"+id.String())
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return strings.Join(out, ", ")
}

// GroupsString answers the groups as "g:name, g:other". WorldGuard:
// DefaultDomain.toGroupsString
func (d *Domain) GroupsString() string {
	gs := d.Groups()
	for i, g := range gs {
		gs[i] = "g:" + g
	}
	return strings.Join(gs, ", ")
}

// String answers the players then the groups, separated by "; ".
// WorldGuard: DefaultDomain.toUserFriendlyString
func (d *Domain) String(names func(UUID) (string, bool)) string {
	var parts []string
	if len(d.Names())+len(d.UUIDs()) > 0 {
		parts = append(parts, d.PlayersString(names))
	}
	if len(d.Groups()) > 0 {
		parts = append(parts, d.GroupsString())
	}
	return strings.Join(parts, "; ")
}
