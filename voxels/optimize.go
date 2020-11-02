package voxels

import (
	"image"
	"log"
	"math"
	"sort"

	"github.com/gmlewis/irmf-slicer/stl"
)

func (c *client) optimizeSTL(sliceNum int, z, voxelRadius float32, img image.Image) error {
	if c.n[2] < 0 { // temporarily for debugging
		return nil
	}

	labels := connectedComponentLabeling(img)
	log.Printf("voxels.optimizeSTL(sliceNum=%v, z=%v, voxelRadius=%v): generated %v connected-component labels", sliceNum, z, voxelRadius, len(labels))

	// Generate labels in consistent, repeatable order.
	keys := make([]int, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Ints(keys)

	min, _ := c.slicer.MBB()
	depth := float32(z) + c.n[2]*float32(voxelRadius)
	vr := float32(voxelRadius)
	vr2 := float32(2.0 * voxelRadius)

	for _, key := range keys {
		label := labels[key]
		log.Printf("Processing label #%v: %v pixels", key, len(label.pixels))
		edges := findEdges(label)
		log.Printf("found %v edges", len(edges))
		paths := edgesToPaths(edges)
		log.Printf("found %v paths", len(paths))
		for i, path := range paths {
			hullPath := convexHull(path, i > 0)
			log.Printf("convex hull #%v: %v points in path", i, len(hullPath))
			if err := c.writeConvexHull(hullPath, path, min, depth, vr, vr2); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *client) writeConvexHull(hullPath, path Path, min [3]float32, depth, vr, vr2 float32) error {
	if len(hullPath) < 2 {
		return nil
	}

	calcXY := func(u, v int) (float32, float32) {
		x := vr2*float32(u) + vr + float32(min[0])
		y := vr2*float32(v) + vr + float32(min[1])
		return x, y
	}

	genTris := func(x1, y1, x2, y2 float32) error {
		// length := math.Sqrt(float64((x2-x1)*(x2-x1) + (y2-y1)*(y2-y1)))
		angle := math.Atan2(float64(y2-y1), float64(x2-x1))
		n := [3]float32{float32(math.Sin(angle)), float32(math.Cos(angle)), 0}
		v1 := [3]float32{x1, y1, depth - vr2}
		v3 := [3]float32{x2, y2, depth}
		t := &stl.Tri{N: n, V1: v1, V2: [3]float32{x2, y2, depth - vr2}, V3: v3}
		if err := c.w.Write(t); err != nil {
			return err
		}
		t = &stl.Tri{N: n, V1: v1, V2: v3, V3: [3]float32{x1, y1, depth}}
		return c.w.Write(t)
	}

	if c.n[2] > 0 { // Also process +X, -X, +Y, and -Y.
		u, v := parseKey(hullPath[0])
		lastX, lastY := calcXY(u, v)
		for _, path := range hullPath[1:] {
			u, v = parseKey(path)
			x, y := calcXY(u, v)
			if err := genTris(lastX, lastY, x, y); err != nil {
				return err
			}
			lastX, lastY = x, y
		}
	}

	return nil
}
