package zipper

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sort"

	"github.com/gmlewis/stldice/v4/binvox"
)

// Grid manifest structure
type Grid struct {
	XMLName   xml.Name  `xml:"grid"`
	GridSizeX int       `xml:"gridSizeX,attr"`
	GridSizeY int       `xml:"gridSizeY,attr"`
	GridSizeZ int       `xml:"gridSizeZ,attr"`
	VoxelSize float64   `xml:"voxelSize,attr"`
	Channels  []Channel `xml:"channels>channel"`
}

type Channel struct {
	Slices string `xml:"slices,attr"`
}

// ReadSVX reads an SVX/ZIP file and returns a BinVOX model.
func ReadSVX(filename string) (*binvox.BinVOX, error) {
	r, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var manifest Grid
	var manifestFound bool
	for _, f := range r.File {
		if f.Name == "manifest.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			if err := xml.NewDecoder(rc).Decode(&manifest); err != nil {
				rc.Close()
				return nil, err
			}
			rc.Close()
			manifestFound = true
			break
		}
	}

	if !manifestFound {
		return nil, fmt.Errorf("manifest.xml not found in %v", filename)
	}

	b := binvox.New(
		manifest.GridSizeX,
		manifest.GridSizeY,
		manifest.GridSizeZ,
		0, 0, 0, // Min values might not be in manifest, assume 0 for now.
		float64(manifest.GridSizeZ)*manifest.VoxelSize*1000.0, // scale in mm
		false,
	)

	// Collect all PNG slices
	sliceMap := make(map[int]*zip.File)
	for _, f := range r.File {
		if filepath.Ext(f.Name) == ".png" {
			var sliceNum int
			_, err := fmt.Sscanf(filepath.Base(f.Name), "slice%d.png", &sliceNum)
			if err == nil {
				sliceMap[sliceNum] = f
			}
		}
	}

	keys := make([]int, 0, len(sliceMap))
	for k := range sliceMap {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	for _, z := range keys {
		f := sliceMap[z]
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		img, err := png.Decode(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}

		bounds := img.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				color := img.At(x, y)
				r, _, _, _ := color.RGBA()
				if r > 32768 { // Assume > 50% density
					b.Add(x, y, z)
				}
			}
		}
	}

	return b, nil
}

// ReadZip reads a simple ZIP of images (without manifest) and returns a BinVOX model.
func ReadZip(filename string, nx, ny, nz int, scale float64) (*binvox.BinVOX, error) {
	r, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	b := binvox.New(nx, ny, nz, 0, 0, 0, scale, false)

	// Collect all images
	var files []*zip.File
	for _, f := range r.File {
		ext := filepath.Ext(f.Name)
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			files = append(files, f)
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	for z, f := range files {
		if z >= nz {
			break
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		img, _, err := image.Decode(rc)
		rc.Close()
		if err != nil {
			continue
		}

		bounds := img.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				if x >= nx || y >= ny {
					continue
				}
				color := img.At(x, y)
				r, g, b_val, _ := color.RGBA()
				if (r+g+b_val)/3 > 32768 {
					b.Add(x, y, z)
				}
			}
		}
	}

	return b, nil
}

// ReadDir reads a directory of images and returns a BinVOX model.
func ReadDir(dirpath string, nx, ny, nz int, scale float64) (*binvox.BinVOX, error) {
	entries, err := os.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}

	b := binvox.New(nx, ny, nz, 0, 0, 0, scale, false)

	var files []string
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			ext := filepath.Ext(entry.Name())
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				files = append(files, filepath.Join(dirpath, entry.Name()))
			}
		}
	}

	sort.Strings(files)

	for z, path := range files {
		if z >= nz {
			break
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			continue
		}

		bounds := img.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				if x >= nx || y >= ny {
					continue
				}
				color := img.At(x, y)
				r, g, b_val, _ := color.RGBA()
				if (r+g+b_val)/3 > 32768 {
					b.Add(x, y, z)
				}
			}
		}
	}

	return b, nil
}
