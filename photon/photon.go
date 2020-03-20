// Package photon is a SliceProcessor that writes its results to one or more
// ChiTuBox .cbddlp files (which are identical to AnyCubic .photon files).
//
// This is based on: github.com/Andoryuuta/photon
// with the major difference that this code does not hold the full
// model in-memory but instead streams the images to the output file.
package photon

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"strings"

	"github.com/gmlewis/irmf-slicer/irmf"
)

// Slicer represents a slicer that provides slices of voxels for multiple
// materials (from an IRMF file).
type Slicer interface {
	NumMaterials() int
	MaterialName(materialNum int) string // 1-based
	MBB() (min, max [3]float32)          // in millimeters

	PrepareRenderZ() error
	RenderZSlices(materialNum int, sp irmf.ZSliceProcessor, order irmf.Order) error
	NumZSlices() int
}

// Slice slices an IRMF shader into one or more .cbddlp files
// containing many voxel slices as PNG images (one file per material).
func Slice(baseFilename string, xRes, yRes, zRes float32, slicer Slicer) error {
	for materialNum := 1; materialNum <= slicer.NumMaterials(); materialNum++ {
		materialName := strings.ReplaceAll(slicer.MaterialName(materialNum), " ", "-")

		dlpName := fmt.Sprintf("%v-mat%02d-%v.cbddlp", baseFilename, materialNum, materialName)

		w, err := os.Create(dlpName)
		if err != nil {
			return fmt.Errorf("Create: %v", err)
		}

		min, max := slicer.MBB()
		log.Printf("MBB=(%v,%v,%v)-(%v,%v,%v)", min[0], min[1], min[2], max[0], max[1], max[2])

		if err := slicer.PrepareRenderZ(); err != nil {
			return fmt.Errorf("PrepareRenderZ: %v", err)
		}

		d := &dlp{w: w, numSlices: slicer.NumZSlices(), xRes: xRes, yRes: yRes, zRes: zRes}
		if err := slicer.RenderZSlices(materialNum, d, irmf.MinToMax); err != nil {
			return err
		}

		// Go back and write all the image offset data.
		if _, err := w.Seek(d.layerHeaderOffset0, io.SeekStart); err != nil {
			return fmt.Errorf("seek: %v", err)
		}
		if err := binary.Write(w, binary.LittleEndian, d.layerHeaders); err != nil {
			return err
		}

		if err := w.Close(); err != nil {
			return fmt.Errorf("Unable to close file: %v", err)
		}
	}
	return nil
}

// dlp represents a SliceProcessor that writes its results
// to a ChiTuBox .cbddlp (aka AnyCubic .photon) file.
type dlp struct {
	w io.Writer

	numSlices int
	xRes      float32
	yRes      float32
	zRes      float32

	layerHeaderOffset0 int64
	layerHeaders       []binCompatLayerHeader
}

// dlp implements the ZSliceProcessor interface.
var _ irmf.ZSliceProcessor = &dlp{}

func (d *dlp) ProcessZSlice(n int, z, voxelRadius float32, img image.Image) error {
	if n == 0 {
		return d.writeHeader(img)
	}

	return d.writeSlice(n, img)
}
