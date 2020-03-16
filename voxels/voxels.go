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

	PrepareRenderX() error
	RenderXSlices(materialNum int, sp irmf.XSliceProcessor, order irmf.Order) error
	PrepareRenderY() error
	RenderYSlices(materialNum int, sp irmf.YSliceProcessor, order irmf.Order) error
	PrepareRenderZ() error
	RenderZSlices(materialNum int, sp irmf.ZSliceProcessor, order irmf.Order) error
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

		if err := slicer.PrepareRenderX(); err != nil {
			return fmt.Errorf("PrepareRenderX: %v", err)
		}

		log.Printf("Processing +X...")
		c.newNormal(1, 0, 0)
		if err := slicer.RenderXSlices(materialNum, c, irmf.MaxToMin); err != nil {
			return fmt.Errorf("RenderXSlices: %v", err)
		}

		log.Printf("Processing -X...")
		c.newNormal(-1, 0, 0)
		if err := slicer.RenderXSlices(materialNum, c, irmf.MinToMax); err != nil {
			return fmt.Errorf("RenderXSlices: %v", err)
		}

		if err := slicer.PrepareRenderY(); err != nil {
			return fmt.Errorf("PrepareRenderY: %v", err)
		}

		log.Printf("Processing +Y...")
		c.newNormal(0, 1, 0)
		if err := slicer.RenderYSlices(materialNum, c, irmf.MaxToMin); err != nil {
			return fmt.Errorf("RenderYSlices: %v", err)
		}

		log.Printf("Processing -Y...")
		c.newNormal(0, -1, 0)
		if err := slicer.RenderYSlices(materialNum, c, irmf.MinToMax); err != nil {
			return fmt.Errorf("RenderYSlices: %v", err)
		}

		if err := slicer.PrepareRenderZ(); err != nil {
			return fmt.Errorf("PrepareRenderZ: %v", err)
		}

		log.Printf("Processing +Z...")
		c.newNormal(0, 0, 1)
		if err := slicer.RenderZSlices(materialNum, c, irmf.MaxToMin); err != nil {
			return fmt.Errorf("RenderZSlices: %v", err)
		}

		log.Printf("Processing -Z...")
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

// client implements the *SliceProcessor interfaces.
var _ irmf.XSliceProcessor = &client{}
var _ irmf.YSliceProcessor = &client{}
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

func (c *client) ProcessXSlice(sliceNum int, x, voxelRadius float32, img image.Image) error {
	debug := false // x <= 2.0*voxelRadius
	if debug {
		log.Printf("voxels.ProcessXSlice(sliceNum=%v, x=%v, voxelRadius=%v)", sliceNum, x, voxelRadius)
	}

	min, _ := c.slicer.MBB()
	depth := float32(x) + c.n[0]*float32(voxelRadius)
	vr := float32(voxelRadius)
	vr2 := float32(2.0 * voxelRadius)

	wf := func(u, v int) error {
		y := vr2*float32(u) + vr + float32(min[1])
		z := vr2*float32(v) + vr + float32(min[2])

		if u == 0 && v == 0 && debug {
			log.Printf("x writeFunc(%v,%v): (%v,%v,%v)-(%v,%v,%v)", u, v, depth, y-vr, z-vr, depth, y+vr, z+vr)
		}

		t := &stl.Tri{
			N:  c.n,
			V1: [3]float32{depth, y - vr, z - vr},
			V2: [3]float32{depth, y + vr, z - vr},
			V3: [3]float32{depth, y + vr, z + vr},
		}
		if err := c.w.Write(t); err != nil {
			return err
		}

		t = &stl.Tri{
			N:  c.n,
			V1: [3]float32{depth, y - vr, z - vr},
			V2: [3]float32{depth, y + vr, z + vr},
			V3: [3]float32{depth, y - vr, z + vr},
		}
		return c.w.Write(t)
	}

	return c.newSlice(img, wf)
}

func (c *client) ProcessYSlice(sliceNum int, y, voxelRadius float32, img image.Image) error {
	debug := false // y <= 2.0*voxelRadius
	if debug {
		log.Printf("voxels.ProcessYSlice(sliceNum=%v, y=%v, voxelRadius=%v)", sliceNum, y, voxelRadius)
	}

	min, _ := c.slicer.MBB()
	depth := float32(y) + c.n[1]*float32(voxelRadius)
	vr := float32(voxelRadius)
	vr2 := float32(2.0 * voxelRadius)

	wf := func(u, v int) error {
		x := vr2*float32(u) + vr + float32(min[0])
		z := vr2*float32(v) + vr + float32(min[2])

		if u == 0 && v == 0 && debug {
			log.Printf("y writeFunc(%v,%v): (%v,%v,%v)-(%v,%v,%v)", u, v, x-vr, depth, z-vr, x+vr, depth, z+vr)
		}

		t := &stl.Tri{
			N:  c.n,
			V1: [3]float32{x - vr, depth, z - vr},
			V2: [3]float32{x + vr, depth, z - vr},
			V3: [3]float32{x + vr, depth, z + vr},
		}
		if err := c.w.Write(t); err != nil {
			return err
		}

		t = &stl.Tri{
			N:  c.n,
			V1: [3]float32{x - vr, depth, z - vr},
			V2: [3]float32{x + vr, depth, z + vr},
			V3: [3]float32{x - vr, depth, z + vr},
		}
		return c.w.Write(t)
	}

	return c.newSlice(img, wf)
}

func (c *client) ProcessZSlice(sliceNum int, z, voxelRadius float32, img image.Image) error {
	debug := false // z <= 2.0*voxelRadius
	if debug {
		log.Printf("voxels.ProcessZSlice(sliceNum=%v, z=%v, voxelRadius=%v)", sliceNum, z, voxelRadius)
	}

	min, _ := c.slicer.MBB()
	depth := float32(z) + c.n[2]*float32(voxelRadius)
	vr := float32(voxelRadius)
	vr2 := float32(2.0 * voxelRadius)

	wf := func(u, v int) error {
		x := vr2*float32(u) + vr + float32(min[0])
		y := vr2*float32(v) + vr + float32(min[1])

		if u == 0 && v == 0 && debug {
			log.Printf("z writeFunc(%v,%v): (%v,%v,%v)-(%v,%v,%v)", u, v, x-vr, y-vr, depth, x+vr, y+vr, depth)
		}

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
