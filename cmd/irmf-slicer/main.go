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

	"github.com/gmlewis/irmf-slicer/binvox"
	"github.com/gmlewis/irmf-slicer/irmf"
	"github.com/gmlewis/irmf-slicer/photon"
	"github.com/gmlewis/irmf-slicer/voxels"
	"github.com/gmlewis/irmf-slicer/zipper"
)

const defaultRes = 42

var (
	microns = flag.Float64("res", 0.0, "Resolution in microns (default is 42.0)")
	view    = flag.Bool("view", false, "Render slicing to window")

	writeBinvox = flag.Bool("binvox", false, "Write binvox files, one per material")
	writeDLP    = flag.Bool("dlp", false, "Write ChiTuBox .cbddlp files (same as AnyCubic .photon), one per material (default resolution is: X:47.25,Y:47.25,Z:50 microns)")
	writeSTL    = flag.Bool("stl", false, "Write stl files, one per material")
	writeSVX    = flag.Bool("svx", false, "Write slices to svx voxel files, one per material (default resolution is 42 microns)")
	writeZip    = flag.Bool("zip", false, "Write slices to zip files, one per material (default resolution is X:65,Y:60,Z:30 microns)")
)

func main() {
	flag.Parse()

	if !*writeBinvox && !*writeDLP && !*writeSTL && !*writeSVX && !*writeZip {
		log.Printf("-binvox, -dlp, -stl, -svx, or -zip must be supplied to generate output. Testing IRMF shader compilation only.")
	}

	var xRes, yRes, zRes float32
	switch {
	case *writeDLP && *microns == 0.0:
		xRes, yRes, zRes = 47.25, 47.25, 50.0
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

		if *writeBinvox {
			log.Printf("Slicing %v materials into separate binvox files (%v slices each)...", slicer.NumMaterials(), slicer.NumZSlices())
			err = binvox.Slice(baseName, slicer)
			check("binvox.Slice: %v", err)
		}

		if *writeDLP {
			log.Printf("Slicing %v materials into separate cdbdlp files (%v slices each)...", slicer.NumMaterials(), slicer.NumZSlices())
			err = photon.Slice(baseName, xRes, yRes, zRes, slicer)
			check("photon.Slice: %v", err)
		}

		if *writeSTL {
			log.Printf("Slicing %v materials into separate STL files (%v slices each)...", slicer.NumMaterials(), slicer.NumZSlices())
			err = voxels.Slice(baseName, slicer)
			check("voxels.Slice: %v", err)
		}

		if *writeSVX {
			log.Printf("Slicing %v materials into separate SVX files (%v slices each)...", slicer.NumMaterials(), slicer.NumZSlices())
			err = zipper.SVXSlice(baseName, slicer)
			check("zipper.SVXSlice: %v", err)
		}

		if *writeZip {
			log.Printf("Slicing %v materials into separate ZIP files (%v slices each)...", slicer.NumMaterials(), slicer.NumZSlices())
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
