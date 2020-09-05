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
	"math"
	"strings"

	"github.com/gmlewis/irmf-slicer/binvox"
	"github.com/gmlewis/irmf-slicer/irmf"
	"github.com/gmlewis/irmf-slicer/photon"
	"github.com/gmlewis/irmf-slicer/voxels"
	"github.com/gmlewis/irmf-slicer/zipper"
)

const defaultRes = 42

var (
	microns = flag.Float64("res", 0.0, "Resolution in microns for X, Y, and Z (default is 42.0)")
	resXovr = flag.Float64("resx", 0.0, "X resolution override in microns")
	resYovr = flag.Float64("resy", 0.0, "Y resolution override in microns")
	resZovr = flag.Float64("resz", 0.0, "Z resolution override in microns")
	view    = flag.Bool("view", false, "Render slicing to window")

	rotX = flag.Float64("rotx", 0.0, "Rotate object around X axis - first  (in degrees)")
	rotY = flag.Float64("roty", 0.0, "Rotate object around Y axis - second (in degrees)")
	rotZ = flag.Float64("rotz", 0.0, "Rotate object around Z axis - third  (in degrees)")

	writeBinvox = flag.Bool("binvox", false, "Write binvox files, one per material")
	writeDLP    = flag.Bool("dlp", false, "Write ChiTuBox .cbddlp files (same as AnyCubic .photon), one per material (default resolution is: X:47.25,Y:47.25,Z:50 microns)")
	writeSTL    = flag.Bool("stl", false, "Write stl files, one per material")
	writeZip    = flag.Bool("zip", false, "Write slices to zip files, one per material (default resolution is X:65,Y:60,Z:30 microns)")
)

func main() {
	flag.Parse()

	if !*writeDLP && !*writeSTL && !*writeZip {
		log.Printf("-dlp or -stl or -zip must be supplied to generate output. Testing IRMF shader compilation only.")
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

	if *resXovr > 0.0 {
		xRes = float32(*resXovr)
	}
	if *resYovr > 0.0 {
		yRes = float32(*resYovr)
	}
	if *resZovr > 0.0 {
		zRes = float32(*resZovr)
	}

	log.Printf("Resolution in microns: X: %v, Y: %v, Z: %v", xRes, yRes, zRes)

	rotx, roty, rotz := float32(*rotX*math.Pi/180.0), float32(*rotY*math.Pi/180.0), float32(*rotZ*math.Pi/180.0)
	slicer := irmf.Init(*view, xRes, yRes, zRes, rotx, roty, rotz)
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
