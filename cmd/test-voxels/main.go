// -*- compile-command: "go run main.go"; -*-

// test-voxels writes out simple STL example files.
package main

import (
	"fmt"
	"image"
	"image/color"
	"log"

	"github.com/gmlewis/irmf-slicer/irmf"
	"github.com/gmlewis/irmf-slicer/voxels"
)

func main() {
	fs := &fakeSlicer{cubeSize: 10.0, uSize: 2, vSize: 2}
	err := voxels.Slice("cube", fs)
	check("Slice: %v", err)

	log.Printf("Done.")
}

func check(fmtStr string, args ...interface{}) {
	if err := args[len(args)-1]; err != nil {
		log.Fatalf(fmtStr, args...)
	}
}

type fakeSlicer struct {
	cubeSize float64 // in millimeters
	uSize    int
	vSize    int
}

func (fs *fakeSlicer) NumMaterials() int                   { return 1 }
func (fs *fakeSlicer) MaterialName(materialNum int) string { return "pla" }
func (fs *fakeSlicer) MBB() (min, max [3]float64) {
	return [3]float64{0, 0, 0}, [3]float64{fs.cubeSize, fs.cubeSize, fs.cubeSize}
}

func (fs *fakeSlicer) PrepareRenderX() error { return nil }
func (fs *fakeSlicer) RenderXSlices(materialNum int, sp irmf.SliceProcessor, order irmf.Order) error {
	return nil
}
func (fs *fakeSlicer) PrepareRenderY() error { return nil }
func (fs *fakeSlicer) RenderYSlices(materialNum int, sp irmf.SliceProcessor, order irmf.Order) error {
	return nil
}
func (fs *fakeSlicer) PrepareRenderZ() error { return nil }
func (fs *fakeSlicer) RenderZSlices(materialNum int, sp irmf.SliceProcessor, order irmf.Order) error {
	mbbMin, mbbMax := fs.MBB()
	img := &fakeImage{fs: fs}

	var (
		n     int
		min   float64
		while func(z float64) bool
		delta float64
	)

	cubeDelta := fs.cubeSize / float64(fs.uSize)
	voxelRadius := 0.5 * cubeDelta
	log.Printf("fakeSlicer.RenderZSlices: cubeDelta=%v, voxelRadius=%v, order=%v", cubeDelta, voxelRadius, order)

	switch order {
	case irmf.MinToMax:
		min, while, delta = mbbMin[2]+voxelRadius, func(z float64) bool { return z <= mbbMax[2] }, cubeDelta
	case irmf.MaxToMin:
		min, while, delta = mbbMax[2]-voxelRadius, func(z float64) bool { return z >= mbbMin[2] }, -cubeDelta
	}

	log.Printf("fakeSlicer.RenderZSlices: min=%v, delta=%v", min, delta)
	for z := min; while(z); z += delta {
		if err := sp.ProcessSlice(n, z, voxelRadius, img); err != nil {
			return fmt.Errorf("ProcessSlice(%v,%v,%v): %v", n, z, voxelRadius, err)
		}
		n++
	}
	return nil
}

type fakeImage struct {
	fs *fakeSlicer
}

func (fi *fakeImage) ColorModel() color.Model { return color.RGBAModel }

func (fi *fakeImage) Bounds() image.Rectangle {
	return image.Rectangle{Max: image.Point{X: fi.fs.uSize, Y: fi.fs.vSize}}
}

func (fi *fakeImage) At(x, y int) color.Color { return color.White }
