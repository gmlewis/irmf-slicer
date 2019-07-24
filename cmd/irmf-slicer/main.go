// irmf-slicer slices one or more IRMF shaders into voxel image slices
// at the requested resolution.
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
)

var (
	microns = flag.Float64("res", 42.0, "Resolution in microns")
	view    = flag.Bool("view", true, "Render slicing to window")
	width   = flag.Int("width", 640, "Initial window width")
	height  = flag.Int("height", 480, "Initial window height")
)

func main() {
	flag.Parse()

	slicer := irmf.Init(*view, *width, *height, *microns)
	defer slicer.Close()

	for _, arg := range flag.Args() {
		if !strings.HasSuffix(arg, ".irmf") {
			log.Printf("Skipping non-IRMF file %q", arg)
			continue
		}
		log.Printf("Processing IRMF shader %q...", filepath.Base(arg))
		buf, err := ioutil.ReadFile(arg)
		check("ReadFile: %v", err)
		irmf, err := slicer.New(buf)
		check("slicer.New: %v", err)

		zipName := filepath.Base(arg) + ".zip"
		log.Printf("Slicing %v materials into file %q...", len(irmf.Materials), zipName)
		err = slicer.Slice(zipName)
		check("Slice: %v", err)
	}

	log.Println("Done.")
}

func check(fmtStr string, args ...interface{}) {
	err := args[len(args)-1]
	if err != nil {
		log.Fatalf(fmtStr, args...)
	}
}
