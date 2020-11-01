package voxels

import (
	"fmt"
	"image"
	"log"
	"sort"
	"strconv"
	"strings"
)

// Label represents a connected component label.
type Label struct {
	xmin, ymin, xmax, ymax int
	// pixels are keyed by labelKey(x,y)
	pixels map[string]struct{}
}

func labelKey(x, y int) string {
	return fmt.Sprintf("%05d,%05d", y, x)
}

func parseKey(key string) (x, y int) {
	parts := strings.Split(key, ",")
	if len(parts) != 2 {
		log.Fatalf("unexpected key: %v", key)
	}
	var err error
	x, err = strconv.Atoi(parts[1])
	if err != nil {
		log.Fatalf("unexpected x value %q: %v", key, err)
	}
	y, err = strconv.Atoi(parts[0])
	if err != nil {
		log.Fatalf("unexpected y value %q: %v", key, err)
	}

	return x, y
}

func connectedComponentLabeling(img image.Image) map[int]*Label {
	b := img.Bounds()
	pixel := func(u, v int) bool {
		if u < b.Min.X || v < b.Min.Y || u > b.Max.X || v > b.Max.Y {
			return false
		}
		color := img.At(u, v)
		r, _, _, _ := color.RGBA()
		return r != 0
	}

	equivalent := map[int]map[int]bool{}
	setEquivalent := func(a, b int) {
		for k := range equivalent[a] {
			equivalent[k][a] = true
			equivalent[k][b] = true
			equivalent[a][k] = true
			equivalent[b][k] = true
		}
		for k := range equivalent[b] {
			equivalent[k][a] = true
			equivalent[k][b] = true
			equivalent[a][k] = true
			equivalent[b][k] = true
		}
	}

	var latestLabel int
	label := map[string]int{}
	for v := b.Min.Y; v <= b.Max.Y; v++ {
		for u := b.Min.X; u <= b.Max.X; u++ {
			here := pixel(u, v)
			log.Printf("(%v,%v) = %v", u, v, here)
			var minLabel int

			left, okLeft := label[labelKey(u-1, v)]
			if okLeft {
				minLabel = left
			}

			upperLeft, okUpperLeft := label[labelKey(u-1, v-1)]
			if okLeft && okUpperLeft && left != upperLeft {
				setEquivalent(left, upperLeft)
				if upperLeft < minLabel {
					minLabel = upperLeft
				}
			} else if okUpperLeft {
				minLabel = upperLeft
			}

			upper, okUpper := label[labelKey(u, v-1)]
			if okLeft && okUpper && left != upper {
				setEquivalent(left, upper)
				if upper < minLabel {
					minLabel = upper
				}
			} else if okUpper {
				minLabel = upper
			}
			if okUpperLeft && okUpper && upperLeft != upper {
				setEquivalent(upperLeft, upper)
				if minLabel == 0 {
					minLabel = upper
					if upperLeft < minLabel {
						minLabel = upperLeft
					}
				}
			}

			upperRight, okUpperRight := label[labelKey(u+1, v-1)]
			if here && okLeft && okUpperRight && left != upperRight {
				setEquivalent(left, upperRight)
				if upperRight < minLabel {
					minLabel = upperRight
				}
			} else if okUpperRight {
				minLabel = upperRight
			}
			if here && okUpperLeft && okUpperRight && upperLeft != upperRight {
				setEquivalent(upperLeft, upperRight)
				if minLabel == 0 {
					minLabel = upperLeft
					if upperRight < minLabel {
						minLabel = upperRight
					}
				}
			}
			if okUpper && okUpperRight && upper != upperRight {
				setEquivalent(upper, upperRight)
				if minLabel == 0 {
					minLabel = upper
					if upperRight < minLabel {
						minLabel = upperRight
					}
				}
			}

			if here {
				if minLabel == 0 {
					latestLabel++
					minLabel = latestLabel
					equivalent[minLabel] = map[int]bool{minLabel: true}
				}
				log.Printf("label[%v]=%v", labelKey(u, v), minLabel)
				label[labelKey(u, v)] = minLabel
			}
		}
	}

	minLabels := make(map[int]int, len(equivalent))
	uniqueLabels := map[int]*Label{}
	for k, equivs := range equivalent {
		log.Printf("equivalent[%v]=%#v", k, equivs)
		for v := range equivs {
			if minLabel, ok := minLabels[k]; ok {
				if v < minLabel {
					minLabels[k] = v
				}
			} else {
				minLabels[k] = v
			}
		}
		if _, ok := uniqueLabels[minLabels[k]]; !ok {
			uniqueLabels[minLabels[k]] = &Label{pixels: map[string]struct{}{}}
		}
	}

	log.Printf("Found %v unique labels", len(uniqueLabels))

	keys := make([]string, 0, len(label))
	for k := range label {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		labelNum := minLabels[label[k]]
		log.Printf("label[%q]=%v", k, labelNum)
		x, y := parseKey(k)

		label := uniqueLabels[labelNum]
		if len(label.pixels) == 0 {
			label.xmin = x
			label.xmax = x
			label.ymin = y
			label.ymax = y
		} else {
			if x < label.xmin {
				label.xmin = x
			}
			if x > label.xmax {
				label.xmax = x
			}
			if y < label.ymin {
				label.ymin = y
			}
			if y > label.ymax {
				label.ymax = y
			}
		}
		label.pixels[k] = struct{}{}
	}

	return uniqueLabels
}
