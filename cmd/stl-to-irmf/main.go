package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cogentcore/webgpu/wgpu"
	"github.com/fogleman/fauxgl"
	"github.com/gmlewis/irmf-slicer/v3/irmf"
	volumetoirmf "github.com/gmlewis/irmf-slicer/v3/volume-to-irmf"
	"github.com/gmlewis/stldice/v4/binvox"
)

func main() {
	input := flag.String("input", "", "Input .stl file")
	output := flag.String("output", "", "Output .irmf file")
	res := flag.Int("res", 256, "Resolution (voxels in max dimension)")
	iters := flag.Int("iters", 1000, "Maximum iterations")
	targetErr := flag.Float64("err", 0.01, "Target error")
	flag.Parse()

	if *input == "" {
		log.Fatal("-input is required")
	}
	if *output == "" {
		*output = strings.TrimSuffix(*input, ".stl") + ".irmf"
	}

	// 1. Initialize WebGPU
	instance := wgpu.CreateInstance(nil)
	if instance == nil {
		log.Fatal("failed to create wgpu instance")
	}
	defer instance.Release()

	adapter, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{})
	if err != nil {
		log.Fatalf("failed to request adapter: %v", err)
	}
	defer adapter.Release()

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		log.Fatalf("failed to request device: %v", err)
	}
	defer device.Release()

	// 2. Load STL and voxelize
	mesh, err := fauxgl.LoadSTL(*input)
	if err != nil {
		log.Fatalf("failed to load STL: %v", err)
	}

	mbb := mesh.BoundingBox()
	size := mbb.Max.Sub(mbb.Min)
	maxSize := size.X
	if size.Y > maxSize {
		maxSize = size.Y
	}
	if size.Z > maxSize {
		maxSize = size.Z
	}

	nx := int(size.X / maxSize * float64(*res))
	ny := int(size.Y / maxSize * float64(*res))
	nz := int(size.Z / maxSize * float64(*res))
	if nx < 1 {
		nx = 1
	}
	if ny < 1 {
		ny = 1
	}
	if nz < 1 {
		nz = 1
	}

	b := binvox.New(nx, ny, nz, mbb.Min.X, mbb.Min.Y, mbb.Min.Z, maxSize, false)
	log.Printf("Voxelizing %v at resolution %vx%vx%v...", *input, nx, ny, nz)
	if err := b.Voxelize(mesh); err != nil {
		log.Fatalf("voxelization failed: %v", err)
	}

	// 3. Optimize
	opt, err := volumetoirmf.NewOptimizer(device, b)
	if err != nil {
		log.Fatalf("failed to create optimizer: %v", err)
	}
	opt.MaxIterations = *iters
	opt.TargetError = float32(*targetErr)

	log.Printf("Optimizing...")
	if err := opt.Optimize(); err != nil {
		log.Fatalf("optimization failed: %v", err)
	}

	// 4. Generate IRMF
	shader := opt.Model.ToWGSL()
	irmfModel := &irmf.IRMF{
		IRMFVersion: "1.0",
		Language:    "wgsl",
		Materials:   []string{"Material 1"},
		Min:         []float32{float32(b.TX), float32(b.TY), float32(b.TZ)},
		Max:         []float32{float32(b.TX + b.Scale), float32(b.TY + b.Scale), float32(b.TZ + b.Scale)},
		Units:       "mm",
	}

	f, err := os.Create(*output)
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "/*{\n")
	jsonBuf, _ := json.MarshalIndent(irmfModel, "", "  ")
	fmt.Fprintf(f, "%s\n}*/\n", jsonBuf)
	fmt.Fprintf(f, "%s", shader)

	log.Printf("Saved to %v", *output)
}
