// Package binvox slices the model and writes binvox files.
package binvox

import (
	"fmt"
	"image"
	"log"

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
		materialName := irmf.SafeMaterialName(slicer.MaterialName(materialNum))

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

		log.Printf("Slicing material %v...", materialName)
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
}

// client implements the ZSliceProcessor interface.
var _ irmf.ZSliceProcessor = &client{}

// new returns a new IRMF-to-binvox client.
func new(b *binvox.BinVOX, slicer Slicer) *client {
	return &client{b: b, slicer: slicer}
}

func (c *client) ProcessZSlice(sliceNum int, z, voxelRadius float32, img image.Image) error {
	b := img.Bounds()
	uSize := b.Max.X - b.Min.X
	vSize := b.Max.Y - b.Min.Y
	c.b.NX = uSize
	c.b.NY = vSize

	for v := b.Min.Y; v < b.Max.Y; v++ {
		for u := b.Min.X; u < b.Max.X; u++ {
			color := img.At(u, v)
			if r, _, _, _ := color.RGBA(); r > 0 {
				c.b.Add(u, v, sliceNum)
			}
		}
	}

	return nil
}
