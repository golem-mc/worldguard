package worldguard

import (
	"cmp"
	"log/slog"
	"maps"
	"slices"
	"sync"

	"github.com/golem-mc/golem/plugin"
)

// RegionStore loads and saves one world's regions. WorldGuard:
// RegionDatabase
type RegionStore interface {
	// Name is the world's name.
	Name() string
	// LoadAll reads every region, with flag values read through flags.
	LoadAll(flags *FlagRegistry) ([]*Region, error)
	// SaveAll replaces the stored regions with regions.
	SaveAll(regions []*Region) error
}

// RemovalStrategy is what removing a region does to its children.
// WorldGuard: RemovalStrategy
type RemovalStrategy uint8

const (
	// RemoveChildren removes the children too, and theirs.
	RemoveChildren RemovalStrategy = iota
	// UnsetParentInChildren leaves the children without a parent.
	UnsetParentInChildren
)

// RegionManager holds one world's regions. It is safe for concurrent use.
// WorldGuard: RegionManager with its index
type RegionManager struct {
	store RegionStore
	log   *slog.Logger

	mu      sync.RWMutex
	regions map[string]*Region
	removed map[*Region]bool
	// chunks caches, per chunk column, the regions whose box meets it;
	// cleared on every change.
	chunks map[plugin.ChunkPos][]*Region
}

// NewRegionManager answers an empty manager over store; log takes load
// warnings.
func NewRegionManager(store RegionStore, log *slog.Logger) *RegionManager {
	return &RegionManager{store: store, log: log, regions: map[string]*Region{}, removed: map[*Region]bool{},
		chunks: map[plugin.ChunkPos][]*Region{}}
}

// Name answers the world's name.
func (m *RegionManager) Name() string { return m.store.Name() }

// Store answers the manager's store.
func (m *RegionManager) Store() RegionStore { return m.store }

// Load replaces the regions with the stored ones.
func (m *RegionManager) Load() error {
	regions, err := m.store.LoadAll(Flags)
	if err != nil {
		return err
	}
	for _, r := range regions {
		r.SetDirty(false)
	}
	m.SetRegions(regions)
	return nil
}

// Save writes every region that is not transient.
func (m *RegionManager) Save() error {
	m.mu.Lock()
	clear(m.removed)
	for _, r := range m.regions {
		r.SetDirty(false)
	}
	regions := m.saved()
	m.mu.Unlock()
	return m.store.SaveAll(regions)
}

func (m *RegionManager) saved() []*Region {
	var out []*Region
	for _, r := range m.regions {
		if !r.IsTransient() {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b *Region) int { return cmp.Compare(a.ID(), b.ID()) })
	return out
}

// SaveChanges saves when a region changed or was removed since the last
// save, and reports whether it did. A failed save keeps the changes marked.
func (m *RegionManager) SaveChanges() (bool, error) {
	m.mu.Lock()
	var changed []*Region
	for _, r := range m.regions {
		if r.Dirty() {
			changed = append(changed, r)
			r.SetDirty(false)
		}
	}
	removed := maps.Clone(m.removed)
	clear(m.removed)
	regions := m.saved()
	m.mu.Unlock()
	if len(changed) == 0 && len(removed) == 0 {
		return false, nil
	}
	if err := m.store.SaveAll(regions); err != nil {
		for _, r := range changed {
			r.SetDirty(true)
		}
		m.mu.Lock()
		maps.Copy(m.removed, removed)
		m.mu.Unlock()
		return false, err
	}
	return true, nil
}

// Dirty reports whether there are changes not saved.
func (m *RegionManager) Dirty() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.removed) > 0 {
		return true
	}
	for _, r := range m.regions {
		if r.Dirty() {
			return true
		}
	}
	return false
}

// Regions answers every region by id.
func (m *RegionManager) Regions() map[string]*Region {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return maps.Clone(m.regions)
}

// SetRegions replaces every region, keeping no changes to save.
func (m *RegionManager) SetRegions(regions []*Region) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.regions = map[string]*Region{}
	for _, r := range regions {
		m.add(r)
	}
	clear(m.removed)
	clear(m.chunks)
}

// AddRegion adds r, and its parents, replacing a region of the same id; the
// replaced region's children take r as their parent.
func (m *RegionManager) AddRegion(r *Region) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.add(r)
	clear(m.chunks)
}

func (m *RegionManager) add(r *Region) {
	r.SetDirty(true)
	if old, ok := m.regions[r.ID()]; ok && old != r {
		m.removeLocked(old.ID(), UnsetParentInChildren, r)
		delete(m.removed, old)
	}
	m.regions[r.ID()] = r
	delete(m.removed, r)
	if p := r.Parent(); p != nil {
		m.add(p)
	}
}

// HasRegion reports whether a region has the id, ignoring case.
func (m *RegionManager) HasRegion(id string) bool {
	_, ok := m.Region(id)
	return ok
}

// Region answers the region with the id, ignoring case.
func (m *RegionManager) Region(id string) (*Region, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.regions[NormalizeID(id)]
	return r, ok
}

// RemoveRegion removes the region with the id and answers what was removed:
// with RemoveChildren its children too.
func (m *RegionManager) RemoveRegion(id string, strategy RemovalStrategy) []*Region {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.removeLocked(id, strategy, nil)
	clear(m.chunks)
	return out
}

func (m *RegionManager) removeLocked(id string, strategy RemovalStrategy, replacement *Region) []*Region {
	r, ok := m.regions[NormalizeID(id)]
	if !ok {
		return nil
	}
	delete(m.regions, r.ID())
	removed := map[*Region]bool{r: true}
	out := []*Region{r}
	for {
		before := len(out)
		for _, cur := range slices.Sorted(maps.Keys(m.regions)) {
			c := m.regions[cur]
			if p := c.Parent(); p != nil && removed[p] {
				if strategy == RemoveChildren {
					removed[c] = true
					out = append(out, c)
					delete(m.regions, cur)
				} else if err := c.SetParent(replacement); err != nil {
					_ = c.SetParent(nil)
				}
			}
		}
		if strategy == UnsetParentInChildren || len(out) == before {
			break
		}
	}
	for _, x := range out {
		m.removed[x] = true
	}
	return out
}

// Len answers the number of regions, the global region included.
func (m *RegionManager) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.regions)
}

// candidates answers the regions whose box meets the chunk column of pos.
func (m *RegionManager) candidates(pos plugin.BlockPos) []*Region {
	c := pos.Chunk()
	m.mu.RLock()
	list, ok := m.chunks[c]
	m.mu.RUnlock()
	if ok {
		return list
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if list, ok := m.chunks[c]; ok {
		return list
	}
	x0, z0 := c.X<<4, c.Z<<4
	list = []*Region{}
	for _, r := range m.regions {
		if r.IsPhysical() && r.max.X >= x0 && r.min.X <= x0+15 && r.max.Z >= z0 && r.min.Z <= z0+15 {
			list = append(list, r)
		}
	}
	m.chunks[c] = list
	return list
}

// QueryOption says whether a query adds the parents of the regions found.
type QueryOption uint8

const (
	// ComputeParents adds each region's parents, as flag queries need.
	ComputeParents QueryOption = iota
	// NoParents answers only the regions found.
	NoParents
)

// ApplicableRegions answers the regions containing pos.
func (m *RegionManager) ApplicableRegions(pos plugin.BlockPos, opt QueryOption) *RegionResultSet {
	var found []*Region
	for _, r := range m.candidates(pos) {
		if r.Contains(pos) {
			found = append(found, r)
		}
	}
	return m.resultSet(found, opt)
}

// ApplicableRegionsFor answers the regions intersecting region.
func (m *RegionManager) ApplicableRegionsFor(region *Region, opt QueryOption) *RegionResultSet {
	return m.resultSet(region.IntersectingRegions(m.physical()), opt)
}

func (m *RegionManager) physical() []*Region {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Region, 0, len(m.regions))
	for _, r := range m.regions {
		if r.IsPhysical() {
			out = append(out, r)
		}
	}
	return out
}

func (m *RegionManager) resultSet(found []*Region, opt QueryOption) *RegionResultSet {
	if opt == ComputeParents {
		seen := map[*Region]bool{}
		var all []*Region
		for _, r := range found {
			for q := r; q != nil && !seen[q]; q = q.Parent() {
				seen[q] = true
				all = append(all, q)
			}
		}
		found = all
	}
	global, _ := m.Region(GlobalRegionID)
	return NewRegionResultSet(found, global)
}

// ApplicableRegionIDs answers the ids of the regions containing pos.
func (m *RegionManager) ApplicableRegionIDs(pos plugin.BlockPos) []string {
	var out []string
	for _, r := range m.candidates(pos) {
		if r.Contains(pos) {
			out = append(out, r.ID())
		}
	}
	return out
}

// OverlapsUnownedRegion reports whether region intersects a region p does
// not own.
func (m *RegionManager) OverlapsUnownedRegion(region *Region, p LocalPlayer) bool {
	for _, r := range region.IntersectingRegions(m.physical()) {
		if !r.Owners().Contains(p) {
			return true
		}
	}
	return false
}

// RegionCountOfPlayer answers how many regions p owns directly.
func (m *RegionManager) RegionCountOfPlayer(p LocalPlayer) int {
	n := 0
	for _, r := range m.Regions() {
		if r.Owners().Contains(p) {
			n++
		}
	}
	return n
}
