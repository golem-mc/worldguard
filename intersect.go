package worldguard

import (
	"slices"
)

// areasOverlap reports whether two polygons (even-odd filled, as AWT's
// Polygon is) share an area larger than zero; touching edges do not count.
// WorldGuard compares java.awt.geom.Area intersections.
//
// The x axis is cut at every vertex and at every crossing of two edges, so
// that within each slab no edge ends or crosses another and each polygon's
// cross-section is a fixed set of intervals whose ends move linearly. Two
// such sets overlap somewhere inside the slab exactly when they overlap at
// its middle.
func areasOverlap(a, b [][2]float64) bool {
	xs := make([]float64, 0, len(a)+len(b))
	for _, p := range a {
		xs = append(xs, p[0])
	}
	for _, p := range b {
		xs = append(xs, p[0])
	}
	edges := append(polygonEdges(a), polygonEdges(b)...)
	for i := range edges {
		for j := i + 1; j < len(edges); j++ {
			if x, ok := crossingX(edges[i], edges[j]); ok {
				xs = append(xs, x)
			}
		}
	}
	slices.Sort(xs)
	xs = slices.Compact(xs)
	for i := 0; i+1 < len(xs); i++ {
		mid := (xs[i] + xs[i+1]) / 2
		if intervalsOverlap(crossSection(a, mid), crossSection(b, mid)) {
			return true
		}
	}
	return false
}

type edge struct{ x0, z0, x1, z1 float64 }

func polygonEdges(p [][2]float64) []edge {
	out := make([]edge, 0, len(p))
	for i := range p {
		q := p[(i+1)%len(p)]
		out = append(out, edge{p[i][0], p[i][1], q[0], q[1]})
	}
	return out
}

// crossingX answers the x where two non-vertical edges cross strictly inside
// both their x spans.
func crossingX(e, f edge) (float64, bool) {
	if e.x0 == e.x1 || f.x0 == f.x1 {
		return 0, false
	}
	// z = z0 + (x - x0) * slope for each; solve for equal z.
	se := (e.z1 - e.z0) / (e.x1 - e.x0)
	sf := (f.z1 - f.z0) / (f.x1 - f.x0)
	if se == sf {
		return 0, false
	}
	x := (f.z0 - e.z0 + se*e.x0 - sf*f.x0) / (se - sf)
	inside := func(g edge) bool { return x > min(g.x0, g.x1) && x < max(g.x0, g.x1) }
	return x, inside(e) && inside(f)
}

// crossSection answers the polygon's z intervals at x, which is no vertex's
// x, as sorted pairs.
func crossSection(p [][2]float64, x float64) []float64 {
	var zs []float64
	for _, e := range polygonEdges(p) {
		if (e.x0 < x) != (e.x1 < x) {
			zs = append(zs, e.z0+(x-e.x0)*(e.z1-e.z0)/(e.x1-e.x0))
		}
	}
	slices.Sort(zs)
	return zs
}

// intervalsOverlap reports whether two sorted interval lists (pairs of ends)
// overlap by more than a point.
func intervalsOverlap(a, b []float64) bool {
	for i := 0; i+1 < len(a); i += 2 {
		for j := 0; j+1 < len(b); j += 2 {
			if min(a[i+1], b[j+1])-max(a[i], b[j]) > 1e-9 {
				return true
			}
		}
	}
	return false
}
