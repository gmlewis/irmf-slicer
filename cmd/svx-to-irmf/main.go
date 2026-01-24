package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cogentcore/webgpu/wgpu"
	"github.com/gmlewis/irmf-slicer/v3/irmf"
	volumetoirmf "github.com/gmlewis/irmf-slicer/v3/volume-to-irmf"
	"github.com/gmlewis/irmf-slicer/v3/zipper"
)

func main() {
	input := flag.String("input", "", "Input .svx file")
	output := flag.String("output", "", "Output .irmf file")
																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																							1000, 0.01, "mm"

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

	// 2. Load SVX
	b, err := zipper.ReadSVX(*input)
	if err != nil {
		log.Fatalf("failed to read SVX: %v", err)
	}

	// 3. Optimize
	opt, err := volumetoirmf.NewOptimizer(device, b)
	if err != nil {
		log.Fatalf("failed to create optimizer: %v", err)
	}
	opt.MaxIterations = *iters
	opt.TargetError = float32(*targetErr)

	log.Printf("Optimizing %v...", *input)
	if err := opt.Optimize(); err != nil {
		log.Fatalf("optimization failed: %v", err)
	}

	// 4. Generate IRMF
	shader := opt.Model.ToWGSL()
	irmfModel := &irmf.IRMF{
		IRMFVersion: "1.0",
		Language:    "wgsl",
		Materials:   []string{"Material 1"},
		Min:         []float32{float32(b.MinX), float32(b.MinY), float32(b.MinZ)},
		Max:         []float32{float32(b.MinX + b.Scale), float32(b.MinY + b.Scale), float32(b.MinZ + b.Scale)},
		Units:       "mm",
	}

	f, err := os.Create(*output)
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "/*{\n")
	jsonBuf, _ := json.MarshalIndent(irmfModel, "", "  ")
	fmt.Fprintf(f, "%s\n*/\n", jsonBuf)
	fmt.Fprintf(f, "%s", shader)

	log.Printf("Saved to %v", *output)
}
