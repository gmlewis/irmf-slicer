package voxels

import (
	"image"
	"log"
	"sort"
)

func (c *client) optimizeSTL(sliceNum int, z, voxelRadius float32, img image.Image) error {
	if c.n[2] < 0 { // temporarily for debugging
		return nil
	}

	labels := connectedComponentLabeling(img)
	log.Printf("voxels.optimizeSTL(sliceNum=%v, z=%v, voxelRadius=%v): generated %v connected-component labels", sliceNum, z, voxelRadius, len(labels))

	// Generate labels in consistent, repeatable order.
	keys := make([]int, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Ints(keys)

	min, _ := c.slicer.MBB()
	depth := float32(z) + c.n[2]*float32(voxelRadius)
	vr := float32(voxelRadius)
	vr2 := float32(2.0 * voxelRadius)

	for _, key := range keys {
		label := labels[key]
		log.Printf("Processing label #%v: %v pixels", key, len(label.pixels))
		edges := findEdges(label)
		log.Printf("found %v edges", len(edges))
		paths := edgesToPaths(edges)
		log.Printf("found %v paths", len(paths))
		for i, path := range paths {
			hullPath := convexHull(path, i > 0)
			log.Printf("hull path #%v: %v points in path", i, len(hullPath))
			finalPath := correctConcavity(hullPath, path)
			log.Printf("final path #%v: %v points in path", i, len(finalPath))
			if err := c.writePath(finalPath, min, depth, vr, vr2); err != nil {
				return err
			}
		}
	}

	return nil
}
