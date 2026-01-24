package volumetoirmf

import (
	"fmt"
	"unsafe"

	"github.com/cogentcore/webgpu/wgpu"
)

// GpuPrimitive matches the Primitive struct in WGSL.
type GpuPrimitive struct {
	Type      uint32
	Center    [3]float32
	Size      float32
	Operation uint32
}

// GpuModelData matches the ModelData struct in WGSL.
type GpuModelData struct {
	NumPrimitives uint32
	Padding       [3]uint32
	Primitives    [1024]GpuPrimitive
}

// GpuConfig matches the Config struct in WGSL.
type GpuConfig struct {
	NX        uint32
	NY        uint32
	NZ        uint32
	VoxelSize float32
	Min       [3]float32
	Padding   float32
}

// GpuStats matches the Stats struct in WGSL.
type GpuStats struct {
	MSE          float32
	Intersection float32
	Union        float32
}

type Evaluator struct {
	device *wgpu.Device
	queue  *wgpu.Queue

	pipeline *wgpu.ComputePipeline

	targetBuffer *wgpu.Buffer
	modelBuffer  *wgpu.Buffer
	resultBuffer *wgpu.Buffer
	configBuffer *wgpu.Buffer
	readBuffer   *wgpu.Buffer

	bindGroup *wgpu.BindGroup

	numWGs [3]uint32
}

func NewEvaluator(device *wgpu.Device, nx, ny, nz uint32, voxelSize float32, min [3]float32, targetData []uint32) (*Evaluator, error) {
	e := &Evaluator{
		device: device,
		queue:  device.GetQueue(),
	}

	shaderModule, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{
			Code: computeShaderSource,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create shader module: %w", err)
	}
	defer shaderModule.Release()

	// Buffers
	e.targetBuffer, err = device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "Target Voxels Buffer",
		Contents: wgpu.ToBytes(targetData),
		Usage:    wgpu.BufferUsageStorage,
	})
	if err != nil {
		return nil, err
	}

	e.modelBuffer, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "Model Buffer",
		Size:  uint64(unsafe.Sizeof(GpuModelData{})),
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, err
	}

	e.numWGs[0] = (nx + 7) / 8
	e.numWGs[1] = (ny + 7) / 8
	e.numWGs[2] = (nz + 7) / 8
	numWgsTotal := e.numWGs[0] * e.numWGs[1] * e.numWGs[2]

	e.resultBuffer, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "Result Buffer",
		Size:  uint64(numWgsTotal * uint32(unsafe.Sizeof(GpuStats{}))),
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopySrc,
	})
	if err != nil {
		return nil, err
	}

	e.readBuffer, err = device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "Read Buffer",
		Size:  uint64(numWgsTotal * uint32(unsafe.Sizeof(GpuStats{}))),
		Usage: wgpu.BufferUsageMapRead | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, err
	}

	config := GpuConfig{
		NX:        nx,
		NY:        ny,
		NZ:        nz,
		VoxelSize: voxelSize,
		Min:       min,
	}
	e.configBuffer, err = device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "Config Buffer",
		Contents: wgpu.ToBytes([]GpuConfig{config}),
		Usage:    wgpu.BufferUsageUniform,
	})
	if err != nil {
		return nil, err
	}

	// Bind Group Layout
	bindGroupLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			{Binding: 0, Visibility: wgpu.ShaderStageCompute, Buffer: wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage}},
			{Binding: 1, Visibility: wgpu.ShaderStageCompute, Buffer: wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeReadOnlyStorage}},
			{Binding: 2, Visibility: wgpu.ShaderStageCompute, Buffer: wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeStorage}},
			{Binding: 3, Visibility: wgpu.ShaderStageCompute, Buffer: wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform}},
		},
	})
	if err != nil {
		return nil, err
	}
	defer bindGroupLayout.Release()

	e.bindGroup, err = device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: bindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: e.targetBuffer, Size: e.targetBuffer.GetSize()},
			{Binding: 1, Buffer: e.modelBuffer, Size: e.modelBuffer.GetSize()},
			{Binding: 2, Buffer: e.resultBuffer, Size: e.resultBuffer.GetSize()},
			{Binding: 3, Buffer: e.configBuffer, Size: e.configBuffer.GetSize()},
		},
	})
	if err != nil {
		return nil, err
	}

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		BindGroupLayouts: []*wgpu.BindGroupLayout{bindGroupLayout},
	})
	if err != nil {
		return nil, err
	}
	defer pipelineLayout.Release()

	e.pipeline, err = device.CreateComputePipeline(&wgpu.ComputePipelineDescriptor{
		Layout: pipelineLayout,
		Compute: wgpu.ProgrammableStageDescriptor{
			Module:     shaderModule,
			EntryPoint: "main",
		},
	})
	if err != nil {
		return nil, err
	}

	return e, nil
}

func (e *Evaluator) Evaluate(model *Model) (mse, iou float32, err error) {
	data := GpuModelData{
		NumPrimitives: uint32(len(model.Primitives)),
	}
	for i, p := range model.Primitives {
		if i >= 1024 {
			break
		}
		data.Primitives[i] = GpuPrimitive{
			Type:      uint32(p.Type),
			Center:    p.Center,
			Size:      p.Size,
			Operation: uint32(p.Operation),
		}
	}

	e.queue.WriteBuffer(e.modelBuffer, 0, wgpu.ToBytes([]GpuModelData{data}))

	encoder, err := e.device.CreateCommandEncoder(nil)
	if err != nil {
		return 0, 0, err
	}

	computePass := encoder.BeginComputePass(nil)
	computePass.SetPipeline(e.pipeline)
	computePass.SetBindGroup(0, e.bindGroup, nil)
	computePass.DispatchWorkgroups(e.numWGs[0], e.numWGs[1], e.numWGs[2])
	computePass.End()

	encoder.CopyBufferToBuffer(e.resultBuffer, 0, e.readBuffer, 0, e.readBuffer.GetSize())

	commandBuffer, err := encoder.Finish(nil)
	if err != nil {
		return 0, 0, err
	}
	e.queue.Submit(commandBuffer)
	commandBuffer.Release()
	encoder.Release()

	// Read back
	done := make(chan struct{})
	var mapStatus wgpu.BufferMapAsyncStatus
	e.readBuffer.MapAsync(wgpu.MapModeRead, 0, e.readBuffer.GetSize(), func(status wgpu.BufferMapAsyncStatus) {
		mapStatus = status
		close(done)
	})

	for {
		e.device.Poll(false, nil)
		select {
		case <-done:
			goto mapped
		default:
		}
	}

mapped:
	if mapStatus != wgpu.BufferMapAsyncStatusSuccess {
		return 0, 0, fmt.Errorf("failed to map buffer: %v", mapStatus)
	}

	numWgsTotal := e.numWGs[0] * e.numWGs[1] * e.numWGs[2]
	mappedData := e.readBuffer.GetMappedRange(0, uint(e.readBuffer.GetSize()))
	statsSlice := (*[1 << 20]GpuStats)(unsafe.Pointer(&mappedData[0]))[:numWgsTotal:numWgsTotal]

	var totalMSE, totalInter, totalUnion float32
	for _, s := range statsSlice {
		totalMSE += s.MSE
		totalInter += s.Intersection
		totalUnion += s.Union
	}
	e.readBuffer.Unmap()

	mse = totalMSE
	if totalUnion > 0 {
		iou = totalInter / totalUnion
	} else {
		iou = 1.0
	}

	return mse, iou, nil
}

func (e *Evaluator) Close() {
	if e.pipeline != nil {
		e.pipeline.Release()
	}
	if e.targetBuffer != nil {
		e.targetBuffer.Release()
	}
	if e.modelBuffer != nil {
		e.modelBuffer.Release()
	}
	if e.resultBuffer != nil {
		e.resultBuffer.Release()
	}
	if e.configBuffer != nil {
		e.configBuffer.Release()
	}
	if e.readBuffer != nil {
		e.readBuffer.Release()
	}
	if e.bindGroup != nil {
		e.bindGroup.Release()
	}
}
