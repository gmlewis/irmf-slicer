// -*- compile-command: "go run main.go"; -*-

// test-voxels writes out simple STL example files.
package main

import (
	"log"

	"github.com/gmlewis/irmf-slicer/stl"
	"github.com/gmlewis/irmf-slicer/voxels"
)

func main() {
	w, err := stl.New("cube.stl")
	check("stl.New: %v", err)

	c := voxels.New(w, 1, 1, 1, 10000)

	// Process +X
	c.NewNormal(1, 0, 0)
	c.NewSlice(1, 1, 10)

	w.Close()
	log.Printf("Done.")
}

func check(fmtStr string, args ...interface{}) {
	if err := args[len(args)-1]; err != nil {
		log.Fatalf(fmtStr, args...)
	}
}
