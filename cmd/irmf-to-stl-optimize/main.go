// irmf-to-stl-optimize is a program to generate an STL file that
// is as close to `maxSize` as possible without exceeding it.
//
// This program uses:
//   * [irmf-slicer](https://github.com/gmlewis/irmf-slicer)
//   * [marching-cubes](https://github.com/gmlewis/stldice/tree/master/cmd/marching-cubes)
// to run.
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

	maxSize = flag.Int64("max", 50000000, "Maximum STL file size")
	start   = flag.Int("start", 400, "Starting resolution")
)

func main() {
	flag.Parse()

	var pts []string
	for _, arg := range flag.Args() {

		res := *start
		stepSize := res / 2

		visited := map[int]int64{}
		var hitMax bool
		for {
			var largestSTLSize int64
			var smallestValidRes int
			if v, ok := visited[res]; ok {
				largestSTLSize = v
				log.Printf("\n\nAlready processed res=%v, skipping...", res)
			} else {
				args := []string{"irmf-slicer", "-binvox", "-res", fmt.Sprintf("%v", res), arg}
				cmd := exec.Command(args[0], args[1:]...)
				log.Printf("Running: %v", strings.Join(args, " "))
				buf, err := cmd.CombinedOutput()
				check("Unable to run command %v : %s\n%v", strings.Join(args, " "), buf, err)
				log.Printf("%s\n", buf)

				filenames := fileRE.FindAllStringSubmatch(string(buf), -1)
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
					check("os.Remove: %v", os.Remove(filename[1]))

					fi, err = os.Stat(outFile)
					check("os.Stat: %v", err)
					log.Printf("Found: %v - size: %v", outFile, fi.Size())

					if fi.Size() > largestSTLSize {
						largestSTLSize = fi.Size()
						visited[res] = largestSTLSize
					}

					if fi.Size() >= *maxSize {
						check("os.Remove: %v", os.Remove(outFile))
					} else if smallestValidRes == 0 || res <= smallestValidRes {
						// Move file to destination
						newFilename := fmt.Sprintf("%v.stl", strings.TrimSuffix(filename[1], ".binvox"))
						log.Printf("Moving %v to %v (at res=%v)", outFile, newFilename, res)
						err := os.Rename(outFile, newFilename)
						check("os.Rename: %v", err)
						smallestValidRes = res
					}
				}
			}

			if largestSTLSize >= *maxSize {
				if stepSize == 1 {
					hitMax = true
				}
				res += stepSize
				if smallestValidRes != 0 {
					stepSize /= 2
					if stepSize < 1 {
						stepSize = 1
					}
				}
				log.Printf("\n\nRaised res up to %v, stepSize=%v", res, stepSize)
			} else {
				if hitMax {
					break
				} else {
					res -= stepSize
					stepSize /= 2
					if stepSize < 1 {
						stepSize = 1
					}
					if res < 1 {
						res = 1
						stepSize = 1
					}
					log.Printf("\n\nDropped res down to %v, stepSize=%v", res, stepSize)
				}
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
