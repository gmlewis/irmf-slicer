// irmf-slicer slices one or more IRMF shaders into voxel image slices
// at the requested resolution.
//
// It then writes a ZIP of the slices or an STL file for each of
// the materials, or both.
//
// See https://github.com/gmlewis/irmf for more information about IRMF.
package main

import (
	"flag"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"github.com/gmlewis/irmf-slicer/irmf"
	"github.com/gmlewis/irmf-slicer/voxels"
	"github.com/gmlewis/irmf-slicer/zipper"
)

var (
	microns  = flag.Float64("res", 42.0, "Resolution in microns")
	view     = flag.Bool("view", false, "Render slicing to window")
	writeSTL = flag.Bool("stl", false, "Write stl files, one per material")
	writeZip = flag.Bool("zip", false, "Write slices to zip file")
)

func main() {
	flag.Parse()

	if !*writeSTL && !*writeZip {
		flag.Usage()
		log.Fatalf("Must use -stl or -zip or both")
	}

	slicer := irmf.Init(*view, *microns)
	defer slicer.Close()

	for _, arg := range flag.Args() {
		if !strings.HasSuffix(arg, ".irmf") {
			log.Printf("Skipping non-IRMF file %q", arg)
			continue
		}

		log.Printf("Processing IRMF shader %q...", filepath.Base(arg))
		buf, err := ioutil.ReadFile(arg)
		check("ReadFile: %v", err)

		err = slicer.NewModel(buf)
		check("slicer.New: %v", err)

		baseName := strings.TrimSuffix(filepath.Base(arg), ".irmf")

		if *writeSTL {
			log.Printf("Slicing %v materials into separate STL files...", slicer.NumMaterials())
			err = voxels.Slice(baseName, slicer)
			check("voxels.Slice: %v", err)
		}

		if *writeZip {
			zipName := baseName + ".irmf.zip"
			log.Printf("Slicing %v materials into file %q...", slicer.NumMaterials(), zipName)
			err = zipper.Slice(zipName, slicer)
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
