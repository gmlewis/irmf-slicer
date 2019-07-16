// irmf-slicer slices one or more IRMF shaders into voxel image slices
// at the requested resolution.
//
// See https://github.com/gmlewis/irmf for more information about IRMF.
package main

import (
	"flag"
	"io/ioutil"
	"log"
	"strings"
)

func main() {
	flag.Parse()

	for _, arg := range flag.Args() {
		if !strings.HasSuffix(arg, ".irmf") {
			log.Printf("Skipping non-IRMF file %q", arg)
			continue
		}
		log.Printf("Processing IRMF shader %q...", arg)
		buf, err := ioutil.ReadFile(arg)
		check("ReadFile: %v", err)
		log.Printf("buf=%s", buf)
	}

	log.Println("Done.")
}

func check(fmtStr string, args ...interface{}) {
	err := args[len(args)-1]
	if err != nil {
		log.Fatalf(fmtStr, args...)
	}
}
