// irmf-to-stl-stats is a program to generate a list of slicing
// resolutions and resulting STL file sizes so that a correlation
// might be inferred.
//
// This program uses:
//   * [irmf-slicer](https://github.com/gmlewis/irmf-slicer)
//   * [marching-cubes](https://github.com/gmlewis/stldice/tree/master/cmd/marching-cubes)
// to run.
//
// Some results using https://mycurvefit.com/ :
// sphere-2.irmf: y = 3373678 + 267656300*e^(-0.04062438*x)
// axial-ra.irmf: y = 0.7192801 + 291418500*e^(-0.01215068*x)
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const (
	outFile = "outfile-stats.stl"
)

var (
	fileRE = regexp.MustCompile(`Writing: (\S+)`)

	maxSize = flag.Int64("max", 50000000, "Stop searching once file size exceeds max")
)

func main() {
	flag.Parse()

	inputs := []int{200, 175, 150, 125, 100, 75, 60, 50, 45, 42}

	var pts []string
	for _, arg := range flag.Args() {
		for _, res := range inputs {
			args := []string{"irmf-slicer", "-binvox", "-res", fmt.Sprintf("%v", res), arg}
			cmd := exec.Command(args[0], args[1:]...)
			log.Printf("Running: %v", strings.Join(args, " "))
			buf, err := cmd.CombinedOutput()
			check("Unable to run command %v : %s\n%v", strings.Join(args, " "), buf, err)
			log.Printf("%s\n", buf)

			filenames := fileRE.FindAllStringSubmatch(string(buf), -1)
			var limitReached bool
			for _, filename := range filenames {
				fi, err := os.Stat(filename[1])
				check("os.Stat: %v", err)
				log.Printf("Found: %v - size: %v", filename[1], fi.Size())

				// Now, convert the binvox to stl:
				args := []string{"marching-cubes", filename[1], outFile}
				cmd := exec.Command(args[0], args[1:]...)
				log.Printf("Running: %v", strings.Join(args, " "))
				buf, err := cmd.CombinedOutput()
				check("Unable to run command %v : %s\n%v", strings.Join(args, " "), buf, err)
				log.Printf("%s\n", buf)

				fi, err = os.Stat(outFile)
				check("os.Stat: %v", err)
				log.Printf("Found: %v - size: %v", outFile, fi.Size())
				check("os.Remove: %v", os.Remove(outFile))
				pts = append(pts, fmt.Sprintf("%v\t%v", res, fi.Size()))

				if fi.Size() >= *maxSize {
					limitReached = true
				}
			}
			if limitReached {
				break
			}
		}
	}

	fmt.Printf("%v\n", strings.Join(pts, "\n"))
	log.Printf("Done.")
}

func check(fmtStr string, args ...interface{}) {
	if err := args[len(args)-1]; err != nil {
		log.Fatalf(fmtStr, args...)
	}
}
