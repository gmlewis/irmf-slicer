// Package voxels converts voxels to STL.
package voxels

import (
	"fmt"
	"image"
	"strings"

	"github.com/gmlewis/irmf-slicer/irmf"
	"github.com/gmlewis/irmf-slicer/stl"
)

// Slicer represents a slicer that provides slices of voxels for multiple
// materials (from an IRMF model).
type Slicer interface {
	NumMaterials() int
	MaterialName(materialNum int) string // 1-based
	MBB() (min, max [3]float64)          // in millimeters

	PrepareRenderX() error
	RenderXSlices(materialNum int, sp irmf.SliceProcessor, order irmf.Order) error
	PrepareRenderY() error
	RenderYSlices(materialNum int, sp irmf.SliceProcessor, order irmf.Order) error
	PrepareRenderZ() error
	RenderZSlices(materialNum int, sp irmf.SliceProcessor, order irmf.Order) error
}

// Slice slices an IRMF model into one or more STL files (one per material).
func Slice(baseFilename string, slicer Slicer) error {
	for materialNum := 1; materialNum <= slicer.NumMaterials(); materialNum++ {
		materialName := strings.ReplaceAll(slicer.MaterialName(materialNum), " ", "-")

		filename := fmt.Sprintf("%v-mat%02d-%v.stl", baseFilename, materialNum, materialName)
		w, err := stl.New(filename)
		if err != nil {
			return fmt.Errorf("stl.New: %v", err)
		}

		c := new(w, slicer)

		if err := slicer.PrepareRenderZ(); err != nil {
			return fmt.Errorf("PrepareRenderZ: %v", err)
		}

		// Process +Z
		c.newNormal(0, 0, 1)
		if err := slicer.RenderZSlices(materialNum, c, irmf.MaxToMin); err != nil {
			return fmt.Errorf("RenderZSlices: %v", err)
		}

		// Process -Z
		c.newNormal(0, 0, -1)
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

// client implements the SliceProcessor interface.
var _ irmf.SliceProcessor = &client{}

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

func (c *client) ProcessSlice(sliceNum int, z, voxelRadius float64, img image.Image) error {
	// log.Printf("voxels.ProcessSlice(sliceNum=%v, z=%v, voxelRadius=%v)", sliceNum, z, voxelRadius)

	min, _ := c.slicer.MBB()
	depth := float32(z) + c.n[2]*float32(voxelRadius)
	vr := float32(voxelRadius)

	wf := func(u, v int) error {
		x := 2.0*vr*float32(u) + vr + float32(min[0])
		y := 2.0*vr*float32(v) + vr + float32(min[1])

		// log.Printf("writeFunc(%v,%v): (%v,%v,%v)-(%v,%v,%v)", u, v, x-vr, y-vr, depth, x+vr, y+vr, depth)

		t := &stl.Tri{
			N:  c.n,
			V1: [3]float32{x - vr, y - vr, depth},
			V2: [3]float32{x + vr, y - vr, depth},
			V3: [3]float32{x + vr, y + vr, depth},
		}
		if err := c.w.Write(t); err != nil {
			return err
		}

		t = &stl.Tri{
			N:  c.n,
			V1: [3]float32{x - vr, y - vr, depth},
			V2: [3]float32{x + vr, y + vr, depth},
			V3: [3]float32{x - vr, y + vr, depth},
		}
		return c.w.Write(t)
	}

	return c.newSlice(img, wf)
}

type writeFunc func(u, v int) error

// newSlice processes a new slice of voxels with the given writeFunc.
func (c *client) newSlice(img image.Image, wf writeFunc) error {
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
		for u := b.Min.X; u < b.Max.X; u++ {
			color := img.At(u, v)
			if r, _, _, _ := color.RGBA(); r == 0 {
				continue
			}

			key := genKey(u, v)
			c.curSlice.p[key] = struct{}{}
			if c.lastSlice != nil {
				if _, ok := c.lastSlice.p[key]; ok {
					continue // already covered
				}
			}

			if err := wf(u, v); err != nil {
				return err
			}
		}
	}

	return nil
}
