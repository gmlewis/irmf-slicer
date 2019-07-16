package irmf

import (
	"archive/zip"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
)

// Slice slices an IRMF shader into a ZIP containing many voxel slices
func (i *IRMF) Slice(zipName string, microns float64) error {
	zf, err := os.Create(zipName)
	defer func() {
		if err := zf.Close(); err != nil {
			log.Fatalf("Unable to close %v: %v", zipName, err)
		}
	}()
	w := zip.NewWriter(zf)

	img, err := i.renderSlice(0.0, microns)
	if err != nil {
		return fmt.Errorf("renderSlice: %v", err)
	}

	n := 0
	filename := fmt.Sprintf("slices/out%04d.png", n)
	f, err := w.Create(filename)
	if err != nil {
		return fmt.Errorf("Unable to create ZIP file %q: %v", filename, err)
	}
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("PNG encode: %v", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("Unable to close ZIP: %v", err)
	}
	return nil
}

func (i *IRMF) renderSlice(z, microns float64) (image.Image, error) {
	return nil, nil
}
