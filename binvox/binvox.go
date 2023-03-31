// Package binvox slices the model and writes binvox files.
package binvox

import (
	"fmt"
	"image"
	"log"
	"strings"

	"github.com/gmlewis/irmf-slicer/v3/irmf"
	"github.com/gmlewis/stldice/v4/binvox"
)

// Slicer represents a slicer that writes binvox files for multiple
// materials (from an IRMF model).
type Slicer interface {
	NumMaterials() int
	MaterialName(materialNum int) string // 1-based
	MBB() (min, max [3]float32)          // in millimeters

	PrepareRenderZ() error
	RenderZSlices(materialNum int, sp irmf.ZSliceProcessor, order irmf.Order) error
	NumXSlices() int
	NumYSlices() int
	NumZSlices() int
}

// Slice slices an IRMF model into one or more binvox files (one per material).
func Slice(baseFilename string, slicer Slicer) error {
	for materialNum := 1; materialNum <= slicer.NumMaterials(); materialNum++ {
		materialName := strings.ReplaceAll(slicer.MaterialName(materialNum), " ", "-")

		filename := fmt.Sprintf("%v-mat%02d-%v.binvox", baseFilename, materialNum, materialName)

		min, max := slicer.MBB()
		scale := float64(max[2] - min[2])
		b := binvox.New(
			slicer.NumXSlices(),
			slicer.NumYSlices(),
			slicer.NumZSlices(),
			float64(min[0]),
			float64(min[1]),
			float64(min[2]),
			scale,
			false,
		)

		c := new(b, slicer)

		if err := slicer.PrepareRenderZ(); err != nil {
			return fmt.Errorf("PrepareRenderZ: %v", err)
		}

		log.Printf("Processing +Z, +X, -X, +Y, -Y...")
		c.newNormal(0, 0, 1)
		if err := slicer.RenderZSlices(materialNum, c, irmf.MaxToMin); err != nil {
			return fmt.Errorf("RenderZSlices: %v", err)
		}

		log.Printf("Processing -Z...")
		c.newNormal(0, 0, -1)
		if err := slicer.RenderZSlices(materialNum, c, irmf.MinToMax); err != nil {
			return fmt.Errorf("RenderZSlices: %v", err)
		}

		log.Printf("Writing: %v", filename)
		if err := b.Write(filename, 0, 0, 0, b.NX, b.NY, b.NZ); err != nil {
			return fmt.Errorf("Write: %v", err)
		}
	}

	return nil
}

// client represents an IRMF-to-binvox converter.
// It implements the irmf.SliceProcessor interface.
type client struct {
	b      *binvox.BinVOX
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

// new returns a new IRMF-to-binvox client.
func new(b *binvox.BinVOX, slicer Slicer) *client {
	return &client{b: b, slicer: slicer}
}

// newNormal starts a new normal unit vector along a major axis (e.g. +X,+Y,+Z,-X,-Y,-Z).
func (c *client) newNormal(x, y, z float32) {
	c.n = [3]float32{x, y, z}
	c.lastSlice = nil
	c.curSlice = nil
}

func (c *client) ProcessZSlice(sliceNum int, z, voxelRadius float32, img image.Image) error {
	// log.Printf("binvox.ProcessZSlice(sliceNum=%v, z=%v, voxelRadius=%v), depth=%v, vr2=%v", sliceNum, z, voxelRadius, depth, vr2)

	var xpwf, xmwf, ypwf, ymwf, zwf writeFunc
	if c.n[2] > 0 { // Also process +X, -X, +Y, and -Y.
		zval := c.slicer.NumZSlices() - sliceNum - 1

		xpwf = func(u, v int) error {
			c.b.Add(u, v, zval)
			return nil
		}

		xmwf = func(u, v int) error {
			c.b.Add(u, v, zval)
			return nil
		}

		ypwf = func(u, v int) error {
			c.b.Add(u, v, zval)
			return nil
		}

		ymwf = func(u, v int) error {
			c.b.Add(u, v, zval)
			return nil
		}

		zwf = func(u, v int) error {
			c.b.Add(u, v, zval)
			return nil
		}
	} else {
		zwf = func(u, v int) error {
			c.b.Add(u, v, sliceNum)
			return nil
		}
	}

	return c.newSlice(img, xpwf, xmwf, ypwf, ymwf, zwf)
}

type writeFunc func(u, v int) error

// newSlice processes a new slice with the given writeFunc.
func (c *client) newSlice(img image.Image, xpwf, xmwf, ypwf, ymwf, zwf writeFunc) error {
	b := img.Bounds()

	// log.Printf("newSlice: img: %v", b)
	c.lastSlice = c.curSlice
	c.curSlice = &uvSlice{
		p: map[int]struct{}{},
	}
	// b.Min.X and b.Min.Y are always zero in this slicer.
	uSize := b.Max.X - b.Min.X
	vSize := b.Max.Y - b.Min.Y
	c.b.NX = uSize
	c.b.NY = vSize
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
