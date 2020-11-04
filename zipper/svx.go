package zipper

import (
	"archive/zip"
	"fmt"
	"time"
)

// SVXSlice slices an IRMF shader into one or more SVX files
// containing many voxel slices as PNG images (one file per material).
func SVXSlice(baseFilename string, slicer Slicer) error {
	zp := &zipper{fmtStr: "density/slice%04d.png", suffix: "svx", manifest: true}
	return processMaterials(baseFilename, slicer, zp)
}

func (zp *zipper) writeManifest(slicer Slicer) error {
	fh := &zip.FileHeader{
		Name:     "manifest.xml",
		Modified: time.Now(),
	}
	f, err := zp.w.CreateHeader(fh)
	if err != nil {
		return fmt.Errorf("Unable to create ZIP file %q: %v", fh.Name, err)
	}

	min, max := slicer.MBB()
	voxelSize := (max[2] - min[2]) / float32(slicer.NumZSlices())

	fmt.Fprintf(f, manifestFmt,
		slicer.NumXSlices(),
		slicer.NumYSlices(),
		slicer.NumZSlices(),
		voxelSize/1000.0, // voxelSize in meters
		zp.irmf.Author,
		zp.irmf.Date)
	return nil
}

var manifestFmt = `<?xml version="1.0"?>

<grid version="1.0" gridSizeX="%v" gridSizeY="%v" gridSizeZ="%v"
   voxelSize="%v" subvoxelBits="8" slicesOrientation="Z" >

    <channels>
        <channel type="DENSITY" bits="8" slices="density/slice%%04d.png" />
    </channels>

    <materials>
        <material id="1" urn="urn:shapeways:materials/1" />
    </materials>

    <metadata>
        <entry key="author" value=%q />
        <entry key="creationDate" value=%q />
    </metadata>
</grid>`
