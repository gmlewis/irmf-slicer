// Package voxels converts voxels to STL.
package voxels

import (
	"fmt"
	"image"
	"log"
	"strings"

	"github.com/gmlewis/irmf-slicer/irmf"
	"github.com/gmlewis/irmf-slicer/stl"
)

// Slicer represents a slicer that provides slices of voxels for multiple
// materials (from an IRMF model).
type Slicer interface {
	NumMaterials() int
	MaterialName(materialNum int) string // 1-based
	MBB() (min, max [3]float32)          // in millimeters

	// PrepareRenderX() error
	// RenderXSlices(materialNum int, sp irmf.XSliceProcessor, order irmf.Order) error
	// PrepareRenderY() error
	// RenderYSlices(materialNum int, sp irmf.YSliceProcessor, order irmf.Order) error
	PrepareRenderZ() error
	RenderZSlices(materialNum int, sp irmf.ZSliceProcessor, order irmf.Order) error
}

// Slice slices an IRMF model into one or more STL files (one per material).
func Slice(baseFilename string, slicer Slicer) error {
	for materialNum := 1; materialNum <= slicer.NumMaterials(); materialNum++ {
		materialName := strings.ReplaceAll(slicer.MaterialName(materialNum), " ", "-")

		filename := fmt.Sprintf("%v-mat%02d-%v.stl", baseFilename, materialNum, materialName)
		log.Printf("Writing: %v", filename)
		w, err := stl.New(filename)
		if err != nil {
			return fmt.Errorf("stl.New: %v", err)
		}

		c := new(w, slicer)

		if err := slicer.PrepareRenderZ(); err != nil {
			return fmt.Errorf("PrepareRenderZ: %v", err)
		}

		log.Printf("Processing +Z, +X, -X, +Y, -Y...")
		c.newNormal(0, 0, 1)
		fmt.Printf("newNormal(0,0,1)\n")
		if err := slicer.RenderZSlices(materialNum, c, irmf.MaxToMin); err != nil {
			return fmt.Errorf("RenderZSlices: %v", err)
		}

		log.Printf("Processing -Z...")
		c.newNormal(0, 0, -1)
		fmt.Printf("newNormal(0,0,-1)\n")
		if err := slicer.RenderZSlices(materialNum, c, irmf.MinToMax); err != nil {
			return fmt.Errorf("RenderZSlices: %v", err)
		}

		if err := w.Close(); err != nil {
			return fmt.Errorf("Close: %v", err)
		}
	}

	return nil
}

// TriWriter is a writer that accepts STL triangles.
type TriWriter interface {
	Write(t *stl.Tri) error
}

// client represents a voxels-to-STL converter.
// It implements the irmf.SliceProcessor interface.
type client struct {
	w      TriWriter
	slicer Slicer

	// Current normal vector
	n [3]float32

	// Last slice
	lastSlice *uvSlice

	// Current slice
	curSlice *uvSlice

	uSize int
	vSize int
	depth float32
}

// client implements the ZSliceProcessor interface.
var _ irmf.ZSliceProcessor = &client{}

// uvSlice represents a slice of voxels indexed by uv (integer) coordinates
// where the depth value represents the third dimension of the current face.
//
// For example, when processing the +X normal vector, u represents Y,
// v represents Z, and depth represents the current X value of the front
// face of the voxel.
type uvSlice struct {
	p map[int]struct{}
}

// new returns a new voxels to STL client.
func new(w TriWriter, slicer Slicer) *client {
	return &client{w: w, slicer: slicer}
}

// newNormal starts a new normal unit vector along a major axis (e.g. +X,+Y,+Z,-X,-Y,-Z).
func (c *client) newNormal(x, y, z float32) {
	c.n = [3]float32{x, y, z}
	c.lastSlice = nil
	c.curSlice = nil
}

func (c *client) ProcessZSlice(sliceNum int, z, voxelRadius float32, img image.Image) error {
	// labels := connectedComponentLabeling(img)
	// for _, label := range labels {
	// 	c.processLabel(label)
	// }

	min, _ := c.slicer.MBB()
	depth := float32(z) + c.n[2]*float32(voxelRadius)
	vr := float32(voxelRadius)
	vr2 := float32(2.0 * voxelRadius)

	// log.Printf("voxels.ProcessZSlice(sliceNum=%v, z=%v, voxelRadius=%v), depth=%v, vr2=%v", sliceNum, z, voxelRadius, depth, vr2)

	var xpwf, xmwf, ypwf, ymwf writeFunc
	if c.n[2] > 0 { // Also process +X, -X, +Y, and -Y.
		xpwf = func(u, v int) error {
			fmt.Printf("xp(%v,%v)\n", u, v)
			x := vr2*float32(u) + vr + float32(min[0])
			y := vr2*float32(v) + vr + float32(min[1])
			n := [3]float32{1, 0, 0}
			v1 := [3]float32{x + vr, y - vr, depth - vr2}
			v3 := [3]float32{x + vr, y + vr, depth}
			t := &stl.Tri{N: n, V1: v1, V2: [3]float32{x + vr, y + vr, depth - vr2}, V3: v3}
			if err := c.w.Write(t); err != nil {
				return err
			}
			t = &stl.Tri{N: n, V1: v1, V2: v3, V3: [3]float32{x + vr, y - vr, depth}}
			return c.w.Write(t)
		}

		xmwf = func(u, v int) error {
			fmt.Printf("xm(%v,%v)\n", u, v)
			x := vr2*float32(u) + vr + float32(min[0])
			y := vr2*float32(v) + vr + float32(min[1])
			n := [3]float32{-1, 0, 0}
			v1 := [3]float32{x - vr, y + vr, depth}
			v3 := [3]float32{x - vr, y - vr, depth - vr2}
			t := &stl.Tri{N: n, V1: v1, V2: [3]float32{x - vr, y + vr, depth - vr2}, V3: v3}
			if err := c.w.Write(t); err != nil {
				return err
			}
			t = &stl.Tri{N: n, V1: v1, V2: v3, V3: [3]float32{x - vr, y - vr, depth}}
			return c.w.Write(t)
		}

		ypwf = func(u, v int) error {
			fmt.Printf("yp(%v,%v)\n", u, v)
			x := vr2*float32(u) + vr + float32(min[0])
			y := vr2*float32(v) + vr + float32(min[1])
			n := [3]float32{0, 1, 0}
			v1 := [3]float32{x + vr, y + vr, depth - vr2}
			v3 := [3]float32{x - vr, y + vr, depth}
			t := &stl.Tri{N: n, V1: v1, V2: [3]float32{x - vr, y + vr, depth - vr2}, V3: v3}
			if err := c.w.Write(t); err != nil {
				return err
			}
			t = &stl.Tri{N: n, V1: v1, V2: v3, V3: [3]float32{x + vr, y + vr, depth}}
			return c.w.Write(t)
		}

		ymwf = func(u, v int) error {
			fmt.Printf("ym(%v,%v)\n", u, v)
			x := vr2*float32(u) + vr + float32(min[0])
			y := vr2*float32(v) + vr + float32(min[1])
			n := [3]float32{0, -1, 0}
			v1 := [3]float32{x - vr, y - vr, depth}
			v3 := [3]float32{x + vr, y - vr, depth - vr2}
			t := &stl.Tri{N: n, V1: v1, V2: [3]float32{x - vr, y - vr, depth - vr2}, V3: v3}
			if err := c.w.Write(t); err != nil {
				return err
			}
			t = &stl.Tri{N: n, V1: v1, V2: v3, V3: [3]float32{x + vr, y - vr, depth}}
			return c.w.Write(t)
		}
	}

	zwf := func(u, v int) error {
		fmt.Printf("z(%v,%v)\n", u, v)
		x := vr2*float32(u) + vr + float32(min[0])
		y := vr2*float32(v) + vr + float32(min[1])

		n := [3]float32{c.n[0], c.n[1], c.n[2]}
		v1 := [3]float32{x - vr, y - vr, depth}
		v3 := [3]float32{x + vr, y + vr, depth}
		if c.n[2] < 0 {
			v1, v3 = v3, v1
		}

		t := &stl.Tri{N: n, V1: v1, V2: [3]float32{x + vr, y - vr, depth}, V3: v3}
		if err := c.w.Write(t); err != nil {
			return err
		}

		t = &stl.Tri{N: n, V1: v1, V2: v3, V3: [3]float32{x - vr, y + vr, depth}}
		return c.w.Write(t)
	}

	return c.newSlice(img, xpwf, xmwf, ypwf, ymwf, zwf)
}

type writeFunc func(u, v int) error

// newSlice processes a new slice of voxels with the given writeFunc.
func (c *client) newSlice(img image.Image, xpwf, xmwf, ypwf, ymwf, zwf writeFunc) error {
	b := img.Bounds()

	// log.Printf("newSlice: img: %v", b)
	c.lastSlice = c.curSlice
	c.curSlice = &uvSlice{
		p: map[int]struct{}{},
	}
	// b.Min.X and b.Min.Y are always zero in this slicer.
	uSize := b.Max.X - b.Min.X
	genKey := func(u, v int) int {
		return v*uSize + u
	}

	for v := b.Min.Y; v < b.Max.Y; v++ {
		var xmInside bool
		for u := b.Min.X; u < b.Max.X; u++ {
			color := img.At(u, v)
			if r, _, _, _ := color.RGBA(); r == 0 {
				xmInside = false
				continue
			}

			if xmwf != nil && !xmInside {
				if err := xmwf(u, v); err != nil {
					return err
				}
				xmInside = true
			}

			key := genKey(u, v)
			c.curSlice.p[key] = struct{}{}
			if c.lastSlice != nil {
				if _, ok := c.lastSlice.p[key]; ok {
					continue // already covered
				}
			}

			if err := zwf(u, v); err != nil {
				return err
			}
		}

		if xpwf != nil {
			var xpInside bool
			for u := b.Max.X - 1; u >= b.Min.X; u-- {
				color := img.At(u, v)
				if r, _, _, _ := color.RGBA(); r == 0 {
					xpInside = false
					continue
				}

				if !xpInside {
					if err := xpwf(u, v); err != nil {
						return err
					}
					xpInside = true
				}
			}
		}
	}

	if ypwf != nil && ymwf != nil {
		for u := b.Min.X; u < b.Max.X; u++ {
			{
				var ymInside bool
				for v := b.Min.Y; v < b.Max.Y; v++ {
					color := img.At(u, v)
					if r, _, _, _ := color.RGBA(); r == 0 {
						ymInside = false
						continue
					}

					if !ymInside {
						if err := ymwf(u, v); err != nil {
							return err
						}
						ymInside = true
					}
				}
			}

			var ypInside bool
			for v := b.Max.Y - 1; v >= b.Min.Y; v-- {
				color := img.At(u, v)
				if r, _, _, _ := color.RGBA(); r == 0 {
					ypInside = false
					continue
				}

				if !ypInside {
					if err := ypwf(u, v); err != nil {
						return err
					}
					ypInside = true
				}
			}
		}
	}

	return nil
}
