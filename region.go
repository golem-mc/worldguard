package worldguard

import (
	"errors"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/golem-mc/golem/plugin"
)

// GlobalRegionID is the id of a world's global region, which covers the
// whole world. WorldGuard: ProtectedRegion.GLOBAL_REGION
const GlobalRegionID = "__global__"

// RegionType is a region's shape. WorldGuard: RegionType
type RegionType uint8

const (
	RegionCuboid RegionType = iota
	RegionPolygon
	RegionGlobal
)

// String answers the type's name as regions.yml stores it.
func (t RegionType) String() string {
	return [...]string{"cuboid", "poly2d", "global"}[t]
}

// Point2D is a polygon's corner.
type Point2D struct{ X, Z int }

var validRegionID = regexp.MustCompile(`^[A-Za-z0-9_,'\-+/]+$`)

// ValidRegionID reports whether id may name a region. WorldGuard:
// ProtectedRegion.isValidId
func ValidRegionID(id string) bool { return validRegionID.MatchString(id) }

// NormalizeID answers the form region and world names are compared in.
// WorldGuard: Normal.normalize
func NormalizeID(id string) string { return strings.ToLower(id) }

// ErrCircularInheritance is answered by SetParent for a parent that is the
// region or one of its children. WorldGuard: CircularInheritanceException
var ErrCircularInheritance = errors.New("circular inheritance")

// Region is a protected region: a cuboid, a polygon extruded between two
// heights, or a world's global region; with a priority, a parent, owners,
// members and flags. Its shape never changes once made; /rg redefine
// replaces the region. It is safe for concurrent use. WorldGuard:
// ProtectedRegion and its kinds
type Region struct {
	id        string
	kind      RegionType
	transient bool
	min, max  plugin.BlockPos
	points    []Point2D

	mu       sync.RWMutex
	priority int
	parent   *Region
	owners   *Domain
	members  *Domain
	flags    map[Flag]any
	dirty    bool
}

func newRegion(id string, kind RegionType, transient bool) (*Region, error) {
	if !ValidRegionID(id) {
		return nil, errors.New("invalid region ID: " + id)
	}
	return &Region{id: NormalizeID(id), kind: kind, transient: transient, owners: NewDomain(), members: NewDomain(),
		flags: map[Flag]any{}, dirty: true}, nil
}

// NewCuboidRegion answers a cuboid between two corners, inclusive.
func NewCuboidRegion(id string, a, b plugin.BlockPos) (*Region, error) {
	r, err := newRegion(id, RegionCuboid, false)
	if err != nil {
		return nil, err
	}
	r.min = plugin.BlockPos{X: min(a.X, b.X), Y: min(a.Y, b.Y), Z: min(a.Z, b.Z)}
	r.max = plugin.BlockPos{X: max(a.X, b.X), Y: max(a.Y, b.Y), Z: max(a.Z, b.Z)}
	return r, nil
}

// NewPolygonRegion answers a polygon of points between minY and maxY.
func NewPolygonRegion(id string, points []Point2D, minY, maxY int) (*Region, error) {
	if len(points) == 0 {
		return nil, errors.New("a polygon needs points")
	}
	r, err := newRegion(id, RegionPolygon, false)
	if err != nil {
		return nil, err
	}
	r.points = slices.Clone(points)
	// The first point carries minY and every other one maxY, so a minY above
	// maxY swaps them. WorldGuard: ProtectedPolygonalRegion.setMinMaxPoints
	lo, hi := minY, maxY
	if len(points) == 1 {
		hi = minY
	}
	r.min = plugin.BlockPos{X: points[0].X, Y: min(lo, hi), Z: points[0].Z}
	r.max = plugin.BlockPos{X: points[0].X, Y: max(lo, hi), Z: points[0].Z}
	for _, p := range points {
		r.min.X, r.min.Z = min(r.min.X, p.X), min(r.min.Z, p.Z)
		r.max.X, r.max.Z = max(r.max.X, p.X), max(r.max.Z, p.Z)
	}
	return r, nil
}

// NewGlobalRegion answers a global region.
func NewGlobalRegion(id string) (*Region, error) { return newRegion(id, RegionGlobal, false) }

// NewTransientRegion answers a copy of r's shape under id that is never
// saved.
func NewTransientRegion(r *Region, id string) (*Region, error) {
	n, err := newRegion(id, r.kind, true)
	if err != nil {
		return nil, err
	}
	n.min, n.max, n.points = r.min, r.max, slices.Clone(r.points)
	return n, nil
}

// ID answers the region's id, lower case.
func (r *Region) ID() string { return r.id }

// Type answers the region's shape.
func (r *Region) Type() RegionType { return r.kind }

// IsPhysical reports whether the region has an area (it is not global).
func (r *Region) IsPhysical() bool { return r.kind != RegionGlobal }

// IsTransient reports whether the region is never saved.
func (r *Region) IsTransient() bool { return r.transient }

// Min and Max answer the region's bounding box corners, inclusive.
func (r *Region) Min() plugin.BlockPos { return r.min }
func (r *Region) Max() plugin.BlockPos { return r.max }

// Points answers the region's outline: a polygon's points, a cuboid's four
// corners, a global region's origin. WorldGuard: getPoints
func (r *Region) Points() []Point2D {
	switch r.kind {
	case RegionPolygon:
		return slices.Clone(r.points)
	case RegionCuboid:
		return []Point2D{{r.min.X, r.min.Z}, {r.max.X, r.min.Z}, {r.max.X, r.max.Z}, {r.min.X, r.max.Z}}
	}
	return []Point2D{{r.min.X, r.min.Z}}
}

// Volume answers the number of blocks in a cuboid, capped at the largest
// int32; polygons and global regions answer 0, as WorldGuard's do.
func (r *Region) Volume() int {
	if r.kind != RegionCuboid {
		return 0
	}
	v := int64(r.max.X-r.min.X+1) * int64(r.max.Y-r.min.Y+1)
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	v *= int64(r.max.Z - r.min.Z + 1)
	return int(min(v, math.MaxInt32))
}

// Contains reports whether pos is inside the region; a global region
// contains nothing.
func (r *Region) Contains(pos plugin.BlockPos) bool {
	switch r.kind {
	case RegionCuboid:
		return pos.X >= r.min.X && pos.X <= r.max.X && pos.Y >= r.min.Y && pos.Y <= r.max.Y &&
			pos.Z >= r.min.Z && pos.Z <= r.max.Z
	case RegionPolygon:
		return r.polygonContains(pos)
	}
	return false
}

// polygonContains is WorldGuard's crossing test, with points on an edge
// inside. WorldGuard: ProtectedPolygonalRegion.contains
func (r *Region) polygonContains(pos plugin.BlockPos) bool {
	tx, ty, tz := pos.X, pos.Y, pos.Z
	if ty < r.min.Y || ty > r.max.Y || tx < r.min.X || tx > r.max.X || tz < r.min.Z || tz > r.max.Z {
		return false
	}
	inside := false
	n := len(r.points)
	xOld, zOld := r.points[n-1].X, r.points[n-1].Z
	for _, p := range r.points {
		xNew, zNew := p.X, p.Z
		if xNew == tx && zNew == tz {
			return true
		}
		x1, z1, x2, z2 := xNew, zNew, xOld, zOld
		if xNew > xOld {
			x1, z1, x2, z2 = xOld, zOld, xNew, zNew
		}
		if x1 <= tx && tx <= x2 {
			cross := (int64(tz)-int64(z1))*int64(x2-x1) - (int64(z2)-int64(z1))*int64(tx-x1)
			if cross == 0 {
				if (z1 <= tz) == (tz <= z2) {
					return true
				}
			} else if cross < 0 && x1 != tx {
				inside = !inside
			}
		}
		xOld, zOld = xNew, zNew
	}
	return inside
}

// ContainsColumn reports whether the column at x, z is inside the region,
// tested at the region's lowest Y.
func (r *Region) ContainsColumn(x, z int) bool {
	return r.Contains(plugin.BlockPos{X: x, Y: r.min.Y, Z: z})
}

// Priority answers the region's priority.
func (r *Region) Priority() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.priority
}

// SetPriority sets the region's priority.
func (r *Region) SetPriority(p int) {
	r.mu.Lock()
	r.priority, r.dirty = p, true
	r.mu.Unlock()
}

// Parent answers the region's parent, nil for none.
func (r *Region) Parent() *Region {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.parent
}

// SetParent sets the parent; nil clears it. It fails with
// ErrCircularInheritance for r itself or one of its children.
func (r *Region) SetParent(p *Region) error {
	for q := p; q != nil; q = q.Parent() {
		if q == r {
			r.mu.Lock()
			r.dirty = true
			r.mu.Unlock()
			return ErrCircularInheritance
		}
	}
	r.mu.Lock()
	r.parent, r.dirty = p, true
	r.mu.Unlock()
	return nil
}

// Owners and Members answer the region's domains, live.
func (r *Region) Owners() *Domain {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.owners
}

func (r *Region) Members() *Domain {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.members
}

// SetOwners and SetMembers replace the domains with copies of d.
func (r *Region) SetOwners(d *Domain) {
	r.mu.Lock()
	r.owners, r.dirty = d.Copy(), true
	r.mu.Unlock()
}

func (r *Region) SetMembers(d *Domain) {
	r.mu.Lock()
	r.members, r.dirty = d.Copy(), true
	r.mu.Unlock()
}

// HasMembersOrOwners reports whether anyone owns or is a member of the
// region.
func (r *Region) HasMembersOrOwners() bool { return r.Owners().Size() > 0 || r.Members().Size() > 0 }

// IsOwner reports whether p owns the region or one of its parents.
func (r *Region) IsOwner(p LocalPlayer) bool {
	for q := r; q != nil; q = q.Parent() {
		if q.Owners().Contains(p) {
			return true
		}
	}
	return false
}

// IsMember reports whether p owns or is a member of the region or one of
// its parents.
func (r *Region) IsMember(p LocalPlayer) bool { return r.IsOwner(p) || r.IsMemberOnly(p) }

// IsMemberOnly reports whether p is a member, not counting ownership, of
// the region or one of its parents.
func (r *Region) IsMemberOnly(p LocalPlayer) bool {
	for q := r; q != nil; q = q.Parent() {
		if q.Members().Contains(p) {
			return true
		}
	}
	return false
}

// Flag answers the region's own value of f, nil when unset.
func (r *Region) Flag(f Flag) any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.flags[f]
}

// SetFlag sets f; a nil v unsets it.
func (r *Region) SetFlag(f Flag, v any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dirty = true
	if v == nil {
		delete(r.flags, f)
		return
	}
	r.flags[f] = v
}

// Flags answers a copy of the region's flags.
func (r *Region) Flags() map[Flag]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[Flag]any, len(r.flags))
	maps.Copy(out, r.flags)
	return out
}

// SetFlags replaces the region's flags.
func (r *Region) SetFlags(flags map[Flag]any) {
	cp := make(map[Flag]any, len(flags))
	for f, v := range flags {
		if v != nil {
			cp[f] = v
		}
	}
	r.mu.Lock()
	r.flags, r.dirty = cp, true
	r.mu.Unlock()
}

// CopyFrom takes other's owners, members, flags, priority and parent.
func (r *Region) CopyFrom(other *Region) {
	r.SetMembers(other.Members())
	r.SetOwners(other.Owners())
	r.SetFlags(other.Flags())
	r.SetPriority(other.Priority())
	_ = r.SetParent(other.Parent())
}

// Dirty reports whether the region changed since it was last saved; a
// transient region never is.
func (r *Region) Dirty() bool {
	if r.transient {
		return false
	}
	r.mu.RLock()
	d := r.dirty
	r.mu.RUnlock()
	return d || r.Owners().Dirty() || r.Members().Dirty()
}

// SetDirty marks the region changed or saved.
func (r *Region) SetDirty(v bool) {
	r.mu.Lock()
	r.dirty = v
	r.mu.Unlock()
	r.Owners().SetDirty(v)
	r.Members().SetDirty(v)
}

// Compare orders regions by priority, highest first, then by id.
func (r *Region) Compare(o *Region) int {
	if a, b := r.Priority(), o.Priority(); a != b {
		if a > b {
			return -1
		}
		return 1
	}
	return strings.Compare(r.id, o.id)
}

// intersectsBox reports whether the bounding boxes meet.
func (r *Region) intersectsBox(o *Region) bool {
	return o.max.X >= r.min.X && o.max.Y >= r.min.Y && o.max.Z >= r.min.Z &&
		o.min.X <= r.max.X && o.min.Y <= r.max.Y && o.min.Z <= r.max.Z
}

// Intersects reports whether the two physical regions share some area.
// WorldGuard: ProtectedRegion.intersects
func (r *Region) Intersects(o *Region) bool {
	if !r.IsPhysical() || !o.IsPhysical() || !r.intersectsBox(o) {
		return false
	}
	if r.kind == RegionCuboid && o.kind == RegionCuboid {
		return true
	}
	return areasOverlap(r.area(), o.area())
}

// IntersectingRegions answers the physical regions of regions that r
// shares some area with. WorldGuard: getIntersectingRegions
func (r *Region) IntersectingRegions(regions []*Region) []*Region {
	var out []*Region
	for _, o := range regions {
		if r.Intersects(o) {
			out = append(out, o)
		}
	}
	return out
}

// area answers the outline in Java's AWT terms: a cuboid covers its blocks'
// full extent, a polygon only its points' hull. WorldGuard: toArea
func (r *Region) area() [][2]float64 {
	if r.kind == RegionCuboid {
		x0, z0 := float64(r.min.X), float64(r.min.Z)
		x1, z1 := float64(r.max.X+1), float64(r.max.Z+1)
		return [][2]float64{{x0, z0}, {x1, z0}, {x1, z1}, {x0, z1}}
	}
	out := make([][2]float64, len(r.points))
	for i, p := range r.points {
		out[i] = [2]float64{float64(p.X), float64(p.Z)}
	}
	return out
}

// SetHeight moves a cuboid's or polygon's floor and ceiling. Only a
// migration does it, on regions no manager holds yet: a manager indexes a
// region by its shape. WorldGuard: WorldHeightMigration.expand
func (r *Region) SetHeight(minY, maxY int) {
	r.min.Y, r.max.Y = minY, maxY
	r.SetDirty(true)
}
