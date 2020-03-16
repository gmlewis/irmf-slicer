// Package zipper is a SliceProcessor that writes its results to a ZIP file.
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
	MBB() (min, max [3]float64)          // in millimeters

	PrepareRenderZ() error
	RenderZSlices(materialNum int, sp irmf.ZSliceProcessor, order irmf.Order) error
}

// Slice slices an IRMF shader into a ZIP containing many voxel slices.
func Slice(zipName string, slicer Slicer) (newErr error) {
	zf, err := os.Create(zipName)
	if err != nil {
		return fmt.Errorf("Create: %v", err)
	}
	defer func() {
		if newErr == nil {
			if err := zf.Close(); err != nil {
				newErr = fmt.Errorf("Close: %v", err)
			}
		}
	}()
	w := zip.NewWriter(zf)

	min, max := slicer.MBB()
	log.Printf("MBB=(%v,%v,%v)-(%v,%v,%v)", min[0], min[1], min[2], max[0], max[1], max[2])

	if err := slicer.PrepareRenderZ(); err != nil {
		return fmt.Errorf("PrepareRenderZ: %v", err)
	}

	zp := &zipper{w: w}
	for zp.materialNum = 1; zp.materialNum <= slicer.NumMaterials(); zp.materialNum++ {
		zp.materialName = strings.ReplaceAll(slicer.MaterialName(zp.materialNum), " ", "-")

		if err := slicer.RenderZSlices(zp.materialNum, zp, irmf.MinToMax); err != nil {
			return err
		}
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("Unable to close ZIP: %v", err)
	}
	return nil
}

// zipper represents a SliceProcessor that writes its results to a ZIP file.
type zipper struct {
	w *zip.Writer

	materialNum  int
	materialName string
}

// zipper implements the ZSliceProcessor interface.
var _ irmf.ZSliceProcessor = &zipper{}

func (zp *zipper) ProcessZSlice(n int, z, voxelRadius float64, img image.Image) error {
	filename := fmt.Sprintf("mat%02d-%v/out%04d.png", zp.materialNum, zp.materialName, n)
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
