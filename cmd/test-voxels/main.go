// -*- compile-command: "go run main.go"; -*-

// test-voxels writes out simple STL example files.
package main

import (
	"log"

	"github.com/gmlewis/irmf-slicer/irmf"
	"github.com/gmlewis/irmf-slicer/voxels"
)

func main() {
	fs := &fakeSlicer{}
	err := voxels.Slice("cube", fs)
	check("Slice: %v", err)

	log.Printf("Done.")
}

func check(fmtStr string, args ...interface{}) {
	if err := args[len(args)-1]; err != nil {
		log.Fatalf(fmtStr, args...)
	}
}

type fakeSlicer struct{}

func (fs *fakeSlicer) NumMaterials() int                   { return 1 }
func (fs *fakeSlicer) MaterialName(materialNum int) string { return "pla" }
func (fs *fakeSlicer) MBB() (min, max [3]float64) {
	return [3]float64{0, 0, 0}, [3]float64{1, 1, 1}
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
	return nil
}
