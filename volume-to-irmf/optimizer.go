package volumetoirmf

import (
	"fmt"
	"math/rand"

	"github.com/cogentcore/webgpu/wgpu"
	"github.com/gmlewis/stldice/v4/binvox"
)

// Optimizer handles the RL-based optimization of the IRMF model.
type Optimizer struct {
	Target *binvox.BinVOX
	Model  *Model

	Evaluator *Evaluator

	MaxIterations int
	TargetError   float32

	// RL parameters
	Alpha float32 // MSE weight
	Beta  float32 // IoU weight
}

// NewOptimizer creates a new optimizer for a target voxel model.
func NewOptimizer(device *wgpu.Device, target *binvox.BinVOX) (*Optimizer, error) {
	nx, ny, nz := uint32(target.NX), uint32(target.NY), uint32(target.NZ)
	voxelSize := float32(target.Scale / float64(target.NZ)) // Scale is usually max dimension or similar
	min := [3]float32{float32(target.MinX), float32(target.MinY), float32(target.MinZ)}

	targetData := convertTargetData(target)

	eval, err := NewEvaluator(device, nx, ny, nz, voxelSize, min, targetData)
	if err != nil {
		return nil, err
	}

	return &Optimizer{
		Target:        target,
		Model:         &Model{},
		Evaluator:     eval,
		MaxIterations: 1000,
		TargetError:   0.01,
		Alpha:         1.0,
		Beta:          1.0,
	}, nil
}

func convertTargetData(target *binvox.BinVOX) []uint32 {
	totalVoxels := target.NX * target.NY * target.NZ
	numU32 := (totalVoxels + 31) / 32
	data := make([]uint32, numU32)

	for z := 0; z < target.NZ; z++ {
		for y := 0; y < target.NY; y++ {
			for x := 0; x < target.NX; x++ {
				if target.Get(x, y, z) {
					idx := z*(target.NX*target.NY) + y*target.NX + x
					data[idx/32] |= (1 << (uint(idx) % 32))
				}
			}
		}
	}
	return data
}

// Optimize runs the optimization process.
func (o *Optimizer) Optimize() error {
	// 1. Initialize with a single primitive that matches the bounding box.
	o.initialize()

	err, currentErr := o.evaluate()
	if err != nil {
		return err
	}

	// 2. Iterative refinement.
	for i := 0; i < o.MaxIterations; i++ {
		if currentErr < o.TargetError {
			break
		}

		currentErr = o.step(currentErr)
	}

	return nil
}

func (o *Optimizer) step(oldErr float32) float32 {
	// Propose a random mutation.
	if len(o.Model.Primitives) == 0 {
		return oldErr
	}

	idx := rand.Intn(len(o.Model.Primitives))
	oldP := o.Model.Primitives[idx]

	// Mutate
	mutationScale := float32(o.Target.Scale) * 0.05
	o.Model.Primitives[idx].Center[0] += (rand.Float32() - 0.5) * mutationScale
	o.Model.Primitives[idx].Center[1] += (rand.Float32() - 0.5) * mutationScale
	o.Model.Primitives[idx].Center[2] += (rand.Float32() - 0.5) * mutationScale
	o.Model.Primitives[idx].Size += (rand.Float32() - 0.5) * mutationScale
	if o.Model.Primitives[idx].Size < 0 {
		o.Model.Primitives[idx].Size = 0.01
	}

	// Evaluate
	err, newErr := o.evaluate()
	if err != nil {
		// Revert on error
		o.Model.Primitives[idx] = oldP
		return oldErr
	}

	if newErr >= oldErr {
		// Revert
		o.Model.Primitives[idx] = oldP
		return oldErr
	}

	// Occasionally add a new primitive if we are stuck.
	if rand.Float32() < 0.05 {
		o.addPrimitive()
	}

	return newErr
}

func (o *Optimizer) addPrimitive() {
	// Add a small sphere at a random location within the target.
	// In a real implementation, we'd pick a location with high error.
	o.Model.Primitives = append(o.Model.Primitives, Primitive{
		Type:      Sphere,
		Center:    o.Model.Primitives[0].Center,
		Size:      o.Model.Primitives[0].Size * 0.1,
		Operation: Union,
	})
}
