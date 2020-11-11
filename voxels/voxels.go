// Package voxels converts voxels to STL.
package voxels

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"strings"

	"github.com/gmlewis/irmf-slicer/irmf"
	"github.com/gmlewis/stldice/v4/binvox"
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
	NumXSlices() int
	NumYSlices() int
	NumZSlices() int
}

// Slice slices an IRMF model into one or more STL files (one per material).
func Slice(baseFilename string, slicer Slicer) error {
	for materialNum := 1; materialNum <= slicer.NumMaterials(); materialNum++ {
		materialName := strings.ReplaceAll(slicer.MaterialName(materialNum), " ", "-")

		stlFile := fmt.Sprintf("%v-mat%02d-%v.stl", baseFilename, materialNum, materialName)

		min, max := slicer.MBB()
		scale := float64(max[2] - min[2])
		model := binvox.New(
			slicer.NumXSlices(),
			slicer.NumYSlices(),
			slicer.NumZSlices(),
			float64(min[0]),
			float64(min[1]),
			float64(min[2]),
			scale,
			false,
		)

		c := &client{model: model, slicer: slicer}

		log.Printf("Rendering...")
		if err := slicer.PrepareRenderZ(); err != nil {
			return fmt.Errorf("PrepareRenderZ: %v", err)
		}

		if err := slicer.RenderZSlices(materialNum, c, irmf.MaxToMin); err != nil {
			return fmt.Errorf("RenderZSlices: %v", err)
		}

		log.Printf("Converting to STL...")
		mesh := model.MarchingCubes()
		log.Printf("Writing: %v", stlFile)
		if err := mesh.SaveSTL(stlFile); err != nil {
			log.Fatalf("SaveSTL: %v", err)
		}
	}

	return nil
}

// client represents a voxels-to-STL converter.
// It implements the irmf.SliceProcessor interface.
type client struct {
	model  *binvox.BinVOX
	slicer Slicer
}

// client implements the ZSliceProcessor interface.
var _ irmf.ZSliceProcessor = &client{}

func (c *client) ProcessZSlice(sliceNum int, z, voxelRadius float32, img image.Image) error {
	scanImage(img, c.model, sliceNum)
	return nil
}

func scanImage(img image.Image, model *binvox.BinVOX, z int) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			c := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			if c.Y >= 128 {
				model.Add(x, y, z)
			}
		}
	}
}
