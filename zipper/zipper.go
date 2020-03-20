// Package zipper is a SliceProcessor that writes its results to one or more ZIP files.
package zipper

import (
	"archive/zip"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"strings"
	"time"

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
}

// Slice slices an IRMF shader into one or more ZIP files
// containing many voxel slices as PNG images (one file per material).
func Slice(baseFilename string, slicer Slicer) error {
	for materialNum := 1; materialNum <= slicer.NumMaterials(); materialNum++ {
		materialName := strings.ReplaceAll(slicer.MaterialName(materialNum), " ", "-")

		zipName := fmt.Sprintf("%v-mat%02d-%v.zip", baseFilename, materialNum, materialName)

		zf, err := os.Create(zipName)
		if err != nil {
			return fmt.Errorf("Create: %v", err)
		}
		w := zip.NewWriter(zf)

		min, max := slicer.MBB()
		log.Printf("MBB=(%v,%v,%v)-(%v,%v,%v)", min[0], min[1], min[2], max[0], max[1], max[2])

		if err := slicer.PrepareRenderZ(); err != nil {
			return fmt.Errorf("PrepareRenderZ: %v", err)
		}

		zp := &zipper{w: w}
		if err := slicer.RenderZSlices(materialNum, zp, irmf.MinToMax); err != nil {
			return err
		}

		if err := w.Close(); err != nil {
			return fmt.Errorf("Unable to close ZIP writer: %v", err)
		}

		if err := zf.Close(); err != nil {
			return fmt.Errorf("Unable to close ZIP file: %v", err)
		}
	}
	return nil
}

// zipper represents a SliceProcessor that writes its results to a ZIP file.
type zipper struct {
	w *zip.Writer
}

// zipper implements the ZSliceProcessor interface.
var _ irmf.ZSliceProcessor = &zipper{}

func (zp *zipper) ProcessZSlice(n int, z, voxelRadius float32, img image.Image) error {
	filename := fmt.Sprintf("out%04d.png", n)
	fh := &zip.FileHeader{
		Name:     filename,
		Comment:  fmt.Sprintf("z=%0.2f", z),
		Modified: time.Now(),
	}
	f, err := zp.w.CreateHeader(fh)
	if err != nil {
		return fmt.Errorf("Unable to create ZIP file %q: %v", filename, err)
	}
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("PNG encode: %v", err)
	}

	return nil
}
