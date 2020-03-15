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

		c := new(w)

		if err := slicer.PrepareRenderZ(); err != nil {
			return fmt.Errorf("PrepareRenderZ: %v", err)
		}

		// Process +Z
		c.newNormal(0, 0, 1)
		if err := slicer.RenderZSlices(materialNum, c, irmf.MaxToMin); err != nil {
			return fmt.Errorf("RenderZSlices: %v", err)
		}

		// // Process -Z
		// c.newNormal(0, 0, -1)
		// if err := slicer.RenderZSlices(materialNum, c, irmf.MaxToMin); err != nil {
		// 	return fmt.Errorf("RenderZSlices: %v", err)
		// }

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
	w TriWriter

	// Current normal vector
	n [3]float32

	// Last slice
	lastSlice *uvSlice

	// Current slice
	curSlice *uvSlice
}

// client implements the SliceProcessor interface.
var _ irmf.SliceProcessor = &client{}

// uvSlice represents a slice of voxels indexed by uv (integer) coordinates
// where the w value represents the third dimension of the current face.
//
// For example, when processing the +X normal vector, u represents Y,
// v represents Z, and w represents the current X value of the front
// face of the voxel.
type uvSlice struct {
	uSize int
	vSize int
	w     float32

	p map[int]struct{}
}

// new returns a new voxels to STL client.
func new(w TriWriter) *client {
	return &client{
		w: w,
	}
}

func (c *client) ProcessSlice(n int, z, voxelRadius float64, img image.Image) error {
	c.newSlice(1, 1, 10)
	return nil
}

// newNormal starts a new normal vector (e.g. +X,+Y,+Z,-X,-Y,-Z).
func (c *client) newNormal(x, y, z float32) {
	c.n = [3]float32{x, y, z}
}

// newSlice starts a new slice of voxels with the given dimensions and w (depth).
func (c *client) newSlice(uSize, vSize int, w float32) {
	c.lastSlice = c.curSlice
	c.curSlice = &uvSlice{
		uSize: uSize,
		vSize: vSize,
		w:     w,
		p:     map[int]struct{}{},
	}
}
