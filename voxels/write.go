package voxels

import (
	"math"

	"github.com/gmlewis/irmf-slicer/stl"
)

func (c *client) writePath(path Path, min [3]float32, depth, vr, vr2 float32) error {
	if len(path) < 2 {
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
		u, v := parseKey(path[0])
		lastX, lastY := calcXY(u, v)
		for _, label := range path[1:] {
			u, v = parseKey(label)
			x, y := calcXY(u, v)
			if err := genTris(lastX, lastY, x, y); err != nil {
				return err
			}
			lastX, lastY = x, y
		}
	}

	return nil
}
