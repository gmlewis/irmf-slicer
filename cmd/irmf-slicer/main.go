// irmf-slicer slices one or more IRMF shaders into voxel image slices
// at the requested resolution.
//
// It then writes a ZIP of the slices or an STL file for each of
// the materials, or both.
//
// By default, irmf-slicer tests IRMF shader compilation only.
// To generate output, at least one of -stl or -zip must be supplied.
//
// See https://github.com/gmlewis/irmf for more information about IRMF.
package main

import (
	"flag"
	"io/ioutil"
	"log"
	"strings"

	"github.com/gmlewis/irmf-slicer/irmf"
	"github.com/gmlewis/irmf-slicer/voxels"
	"github.com/gmlewis/irmf-slicer/zipper"
)

const defaultRes = 42

var (
	microns  = flag.Float64("res", 0.0, "Resolution in microns (default is 42.0)")
	view     = flag.Bool("view", false, "Render slicing to window")
	writeSTL = flag.Bool("stl", false, "Write stl files, one per material")
	writeZip = flag.Bool("zip", false, "Write slices to zip files, one per material (default resolution is X:65,Y:60,Z:30 microns)")
)

func main() {
	flag.Parse()

	if !*writeSTL && !*writeZip {
		log.Printf("-stl or -zip must be supplied to generate output. Testing IRMF shader compilation only.")
	}

	var xRes, yRes, zRes float32
	switch {
	case *writeZip && *microns == 0.0: // use 65, 60, 30 microns
		xRes, yRes, zRes = 65.0, 60.0, 30.0
	case *microns == 0.0: // use defaultRes
		xRes, yRes, zRes = defaultRes, defaultRes, defaultRes
	default:
		xRes, yRes, zRes = float32(*microns), float32(*microns), float32(*microns)
	}
	log.Printf("Resolution in microns: X: %v, Y: %v, Z: %v", xRes, yRes, zRes)

	slicer := irmf.Init(*view, xRes, yRes, zRes)
	defer slicer.Close()

	for _, arg := range flag.Args() {
		if !strings.HasSuffix(arg, ".irmf") {
			log.Printf("Skipping non-IRMF file %q", arg)
			continue
		}

		log.Printf("Processing IRMF shader %q...", arg)
		buf, err := ioutil.ReadFile(arg)
		check("ReadFile: %v", err)

		err = slicer.NewModel(buf)
		check("%v: %v", arg, err)

		baseName := strings.TrimSuffix(arg, ".irmf")

		if *writeSTL {
			log.Printf("Slicing %v materials into separate STL files...", slicer.NumMaterials())
			err = voxels.Slice(baseName, slicer)
			check("voxels.Slice: %v", err)
		}

		if *writeZip {
			log.Printf("Slicing %v materials into separate ZIP files...", slicer.NumMaterials())
			err = zipper.Slice(baseName, slicer)
			check("zipper.Slice: %v", err)
		}
	}

	log.Println("Done.")
}

func check(fmtStr string, args ...interface{}) {
	err := args[len(args)-1]
	if err != nil {
		log.Fatalf(fmtStr, args...)
	}
}
