// Package voxels converts voxels to STL.
package voxels

import "github.com/gmlewis/irmf-slicer/stl"

// TriWriter is a writer that accepts STL triangles.
type TriWriter interface {
	Write(t *stl.Tri) error
}

// Client represents a voxels-to-STL converter.
type Client struct {
	w TriWriter

	xSize int
	ySize int
	zSize int
	delta float64 // millimeters (model units)

	// Current normal vector
	n [3]float32

	// Last slice
	lastSlice *uvSlice

	// Current slice
	curSlice *uvSlice
}

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

// New returns a new voxels to STL client.
func New(w TriWriter, xSize, ySize, zSize int, micronsResolution float64) *Client {
	return &Client{
		w:     w,
		xSize: xSize,
		ySize: ySize,
		zSize: zSize,
		// TODO: Support units other than millimeters.
		delta: micronsResolution / 1000.0,
	}
}

// NewNormal starts a new normal vector (e.g. +X,+Y,+Z,-X,-Y,-Z).
func (c *Client) NewNormal(x, y, z float32) {
	c.n = [3]float32{x, y, z}
}

// NewSlice starts a new slice of voxels with the given dimensions and w (depth).
func (c *Client) NewSlice(uSize, vSize int, w float32) {
	c.lastSlice = c.curSlice
	c.curSlice = &uvSlice{
		uSize: uSize,
		vSize: vSize,
		w:     w,
		p:     map[int]struct{}{},
	}
}
