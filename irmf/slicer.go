package irmf

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"runtime"

	"github.com/go-gl/mathgl/mgl32"
)

func init() {
	// GLFW event handling must run on the main OS thread
	runtime.LockOSThread()
}

// Slicer represents a slicer context.
type Slicer struct {
	irmf   *IRMF
	width  int
	height int
	deltaX float32 // millimeters (model units)
	deltaY float32
	deltaZ float32
	view   bool

	// allowRemoteIncludes permits #include URLs to be fetched while parsing.
	// Off by default; see SetRemoteIncludes.
	allowRemoteIncludes bool

	renderer Renderer
}

// Init returns a new Slicer instance.
func Init(view bool, umXRes, umYRes, umZRes float32) *Slicer {
	// TODO: Support units other than millimeters.
	return &Slicer{deltaX: umXRes / 1000.0, deltaY: umYRes / 1000.0, deltaZ: umZRes / 1000.0, view: view}
}

// SetRemoteIncludes controls whether #include directives may be fetched from
// the network while parsing models. It is disabled by default because remote
// includes are executable code fetched at load time. Enable it only for models
// you trust.
func (s *Slicer) SetRemoteIncludes(allow bool) {
	s.allowRemoteIncludes = allow
}

// validateDimensions rejects models whose slice counts would exhaust time or
// memory. Bounds come from the untrusted header, so they must be bounded
// independently of the resolution chosen by the user.
func (s *Slicer) validateDimensions(irmf *IRMF) error {
	if s.deltaX <= 0 || s.deltaY <= 0 || s.deltaZ <= 0 {
		return errors.New("resolution must be positive")
	}
	extents := [3]float64{
		float64(irmf.Max[0] - irmf.Min[0]),
		float64(irmf.Max[1] - irmf.Min[1]),
		float64(irmf.Max[2] - irmf.Min[2]),
	}
	deltas := [3]float64{float64(s.deltaX), float64(s.deltaY), float64(s.deltaZ)}
	var counts [3]int64
	for axis := 0; axis < 3; axis++ {
		n := int64(0.5 + extents[axis]/deltas[axis])
		if n < 1 {
			return fmt.Errorf("model is thinner than one voxel on axis %d", axis)
		}
		if n > maxSlices {
			return fmt.Errorf("model requires %d slices on axis %d (limit is %d)", n, axis, maxSlices)
		}
		counts[axis] = n
	}
	if pixels := counts[0] * counts[1]; pixels > maxSlicePixels {
		return fmt.Errorf("model requires %d samples per slice (limit is %d)", pixels, maxSlicePixels)
	}
	return nil
}

// NewModel prepares the slicer to slice a new shader model.
func (s *Slicer) NewModel(shaderSrc []byte) error {
	irmf, err := newModelWithOptions(shaderSrc, modelOptions{
		AllowRemoteIncludes: s.allowRemoteIncludes,
	})
	if err != nil {
		return err
	}
	if err := s.validateDimensions(irmf); err != nil {
		return err
	}
	s.irmf = irmf

	// Select renderer based on language.
	// We might want to delay this until PrepareRender, but for now we can do it here.
	switch irmf.Language {
	case "glsl", "":
		if _, ok := s.renderer.(*OpenGLRenderer); !ok {
			if s.renderer != nil {
				s.renderer.Close()
			}
			s.renderer = &OpenGLRenderer{}
		}
	case "wgsl":
		if _, ok := s.renderer.(*WebGPURenderer); !ok {
			if s.renderer != nil {
				s.renderer.Close()
			}
			s.renderer = &WebGPURenderer{}
		}
	case "tsl":
		if _, ok := s.renderer.(*TSLRenderer); !ok {
			if s.renderer != nil {
				s.renderer.Close()
			}
			s.renderer = &TSLRenderer{}
		}
	default:
		return fmt.Errorf("unsupported language: %q", irmf.Language)
	}

	return nil
}

func (s *Slicer) IRMF() *IRMF {
	return s.irmf
}

// CompileOnly compiles the current model without rendering. GLSL and WGSL are
// compiled by the GPU, so this is a no-op for them; TSL is compiled eagerly by
// the external compiler.
func (s *Slicer) CompileOnly() error {
	if s.irmf == nil {
		return errors.New("no model loaded")
	}
	if s.irmf.Language != "tsl" {
		return nil
	}
	_, err := activeTSLCompiler().Compile(context.Background(), s.irmf.Shader, len(s.irmf.Materials))
	return err
}

// Close closes the renderer and releases any Slicer resources.
func (s *Slicer) Close() {
	if s.renderer != nil {
		s.renderer.Close()
	}
}

// NumMaterials returns the number of materials in the most recent IRMF model.
func (s *Slicer) NumMaterials() int {
	if s.irmf == nil {
		return 0
	}
	return len(s.irmf.Materials)
}

// MaterialName returns the name of the n-th material (1-based).
func (s *Slicer) MaterialName(n int) string {
	if s.irmf == nil || n > len(s.irmf.Materials) {
		return ""
	}
	return s.irmf.Materials[n-1]
}

// MBB returns the MBB of the IRMF model.
func (s *Slicer) MBB() (min, max [3]float32) {
	if s.irmf != nil {
		if len(s.irmf.Min) != 3 || len(s.irmf.Max) != 3 {
			log.Fatalf("Bad IRMF model: min=%#v, max=%#v", s.irmf.Min, s.irmf.Max)
		}
		min[0], min[1], min[2] = s.irmf.Min[0], s.irmf.Min[1], s.irmf.Min[2]
		max[0], max[1], max[2] = s.irmf.Max[0], s.irmf.Max[1], s.irmf.Max[2]
	}
	return min, max
}

// XSliceProcessor represents a X slice processor.
type XSliceProcessor interface {
	ProcessXSlice(sliceNum int, x, voxelRadius float32, img image.Image) error
}

// YSliceProcessor represents a Y slice processor.
type YSliceProcessor interface {
	ProcessYSlice(sliceNum int, y, voxelRadius float32, img image.Image) error
}

// ZSliceProcessor represents a Z slice processor.
type ZSliceProcessor interface {
	ProcessZSlice(sliceNum int, z, voxelRadius float32, img image.Image) error
}

// Order represents the order of slice processing.
type Order byte

const (
	MinToMax Order = iota
	MaxToMin
)

// NumXSlices returns the number of slices in the X direction.
func (s *Slicer) NumXSlices() int {
	n := int(0.5 + (s.irmf.Max[0]-s.irmf.Min[0])/s.deltaX)
	if n%2 == 1 {
		n++
	}
	return n
}

// RenderXSlices slices the given materialNum (1-based index)
// to an image, calling the SliceProcessor for each slice.
func (s *Slicer) RenderXSlices(materialNum int, sp XSliceProcessor, order Order) error {
	numSlices := int(0.5 + (s.irmf.Max[0]-s.irmf.Min[0])/s.deltaX)
	voxelRadiusX := 0.5 * s.deltaX
	minVal := s.irmf.Min[0] + voxelRadiusX

	var xFunc func(n int) float32

	switch order {
	case MinToMax:
		xFunc = func(n int) float32 {
			return minVal + float32(n)*s.deltaX
		}
	case MaxToMin:
		xFunc = func(n int) float32 {
			return minVal + float32(numSlices-n-1)*s.deltaX
		}
	}

	for n := 0; n < numSlices; n++ {
		x := xFunc(n)

		img, err := s.renderSlice(x, materialNum)
		if err != nil {
			return fmt.Errorf("renderXSlice(%v,%v): %v", x, materialNum, err)
		}
		if err := sp.ProcessXSlice(n, x, voxelRadiusX, img); err != nil {
			return fmt.Errorf("ProcessSlice(%v,%v,%v): %v", n, x, voxelRadiusX, err)
		}
	}
	return nil
}

// NumYSlices returns the number of slices in the Y direction.
func (s *Slicer) NumYSlices() int {
	nx := int(0.5 + (s.irmf.Max[0]-s.irmf.Min[0])/s.deltaX)
	ny := int(0.5 + (s.irmf.Max[1]-s.irmf.Min[1])/s.deltaY)
	if nx%2 == 1 {
		ny++
	}
	return ny
}

// RenderYSlices slices the given materialNum (1-based index)
// to an image, calling the SliceProcessor for each slice.
func (s *Slicer) RenderYSlices(materialNum int, sp YSliceProcessor, order Order) error {
	numSlices := int(0.5 + (s.irmf.Max[1]-s.irmf.Min[1])/s.deltaY)
	voxelRadiusY := 0.5 * s.deltaY
	minVal := s.irmf.Min[1] + voxelRadiusY

	var yFunc func(n int) float32

	switch order {
	case MinToMax:
		yFunc = func(n int) float32 {
			return minVal + float32(n)*s.deltaY
		}
	case MaxToMin:
		yFunc = func(n int) float32 {
			return minVal + float32(numSlices-n-1)*s.deltaY
		}
	}

	for n := 0; n < numSlices; n++ {
		y := yFunc(n)

		img, err := s.renderSlice(y, materialNum)
		if err != nil {
			return fmt.Errorf("renderYSlice(%v,%v): %v", y, materialNum, err)
		}
		if err := sp.ProcessYSlice(n, y, voxelRadiusY, img); err != nil {
			return fmt.Errorf("ProcessSlice(%v,%v,%v): %v", n, y, voxelRadiusY, err)
		}
	}
	return nil
}

// NumZSlices returns the number of slices in the Z direction.
func (s *Slicer) NumZSlices() int {
	return int(0.5 + (s.irmf.Max[2]-s.irmf.Min[2])/s.deltaZ)
}

// RenderZSlices slices the given materialNum (1-based index)
// to an image, calling the SliceProcessor for each slice.
func (s *Slicer) RenderZSlices(materialNum int, sp ZSliceProcessor, order Order) error {
	numSlices := int(0.5 + (s.irmf.Max[2]-s.irmf.Min[2])/s.deltaZ)
	voxelRadiusZ := 0.5 * s.deltaZ
	minVal := s.irmf.Min[2] + voxelRadiusZ

	var zFunc func(n int) float32

	switch order {
	case MinToMax:
		zFunc = func(n int) float32 {
			return minVal + float32(n)*s.deltaZ
		}
	case MaxToMin:
		zFunc = func(n int) float32 {
			return minVal + float32(numSlices-n-1)*s.deltaZ
		}
	}

	for n := 0; n < numSlices; n++ {
		z := zFunc(n)

		img, err := s.renderSlice(z, materialNum)
		if err != nil {
			return fmt.Errorf("renderZSlice(%v,%v): %v", z, materialNum, err)
		}
		if err := sp.ProcessZSlice(n, z, voxelRadiusZ, img); err != nil {
			return fmt.Errorf("ProcessSlice(%v,%v,%v): %v", n, z, voxelRadiusZ, err)
		}
	}
	return nil
}

func (s *Slicer) renderSlice(sliceDepth float32, materialNum int) (image.Image, error) {
	if s.renderer == nil {
		return nil, fmt.Errorf("renderer not initialized")
	}
	return s.renderer.Render(sliceDepth, materialNum)
}

// PrepareRenderX prepares the GPU to render along the X axis.
func (s *Slicer) PrepareRenderX() error {
	left := float32(s.irmf.Min[1])
	right := float32(s.irmf.Max[1])
	bottom := float32(s.irmf.Min[2])
	top := float32(s.irmf.Max[2])
	camera := mgl32.LookAtV(mgl32.Vec3{3, 0, 0}, mgl32.Vec3{0, 0, 0}, mgl32.Vec3{0, 0, 1})
	vec3Str := "u_slice,fragVert.yz"

	xPlaneVertices[1], xPlaneVertices[11], xPlaneVertices[26] = left, left, left
	xPlaneVertices[6], xPlaneVertices[16], xPlaneVertices[21] = right, right, right
	xPlaneVertices[2], xPlaneVertices[7], xPlaneVertices[17] = bottom, bottom, bottom
	xPlaneVertices[12], xPlaneVertices[22], xPlaneVertices[27] = top, top, top

	aspectRatio := ((right - left) * s.deltaZ) / ((top - bottom) * s.deltaY)
	newWidth := int(0.5 + (right-left)/float32(s.deltaY))
	newHeight := int(0.5 + (top-bottom)/float32(s.deltaZ))
	if aspectRatio*float32(newHeight) < float32(newWidth) {
		newHeight = int(0.5 + float32(newWidth)/aspectRatio)
	}

	return s.prepareRender(newWidth, newHeight, left, right, bottom, top, camera, vec3Str, xPlaneVertices)
}

// PrepareRenderY prepares the GPU to render along the Y axis.
func (s *Slicer) PrepareRenderY() error {
	left := float32(s.irmf.Min[0])
	right := float32(s.irmf.Max[0])
	bottom := float32(s.irmf.Min[2])
	top := float32(s.irmf.Max[2])
	camera := mgl32.LookAtV(mgl32.Vec3{0, -3, 0}, mgl32.Vec3{0, 0, 0}, mgl32.Vec3{0, 0, 1})
	vec3Str := "fragVert.x,u_slice,fragVert.z"

	yPlaneVertices[0], yPlaneVertices[10], yPlaneVertices[25] = left, left, left
	yPlaneVertices[5], yPlaneVertices[15], yPlaneVertices[20] = right, right, right
	yPlaneVertices[2], yPlaneVertices[7], yPlaneVertices[17] = bottom, bottom, bottom
	yPlaneVertices[12], yPlaneVertices[22], yPlaneVertices[27] = top, top, top

	aspectRatio := ((right - left) * s.deltaZ) / ((top - bottom) * s.deltaX)
	newWidth := int(0.5 + (right-left)/float32(s.deltaX))
	newHeight := int(0.5 + (top-bottom)/float32(s.deltaZ))
	if aspectRatio*float32(newHeight) < float32(newWidth) {
		newHeight = int(0.5 + float32(newWidth)/aspectRatio)
	}

	return s.prepareRender(newWidth, newHeight, left, right, bottom, top, camera, vec3Str, yPlaneVertices)
}

// PrepareRenderZ prepares the GPU to render along the Z axis.
func (s *Slicer) PrepareRenderZ() error {
	left := float32(s.irmf.Min[0])
	right := float32(s.irmf.Max[0])
	bottom := float32(s.irmf.Min[1])
	top := float32(s.irmf.Max[1])
	camera := mgl32.LookAtV(mgl32.Vec3{0, 0, 3}, mgl32.Vec3{0, 0, 0}, mgl32.Vec3{0, 1, 0})
	vec3Str := "fragVert.xy,u_slice"

	zPlaneVertices[0], zPlaneVertices[10], zPlaneVertices[25] = left, left, left
	zPlaneVertices[5], zPlaneVertices[15], zPlaneVertices[20] = right, right, right
	zPlaneVertices[1], zPlaneVertices[6], zPlaneVertices[16] = bottom, bottom, bottom
	zPlaneVertices[11], zPlaneVertices[21], zPlaneVertices[26] = top, top, top

	aspectRatio := ((right - left) * s.deltaY) / ((top - bottom) * s.deltaX)
	newWidth := int(0.5 + (right-left)/float32(s.deltaX))
	newHeight := int(0.5 + (top-bottom)/float32(s.deltaY))
	if aspectRatio*float32(newHeight) < float32(newWidth) {
		newHeight = int(0.5 + float32(newWidth)/aspectRatio)
	}

	return s.prepareRender(newWidth, newHeight, left, right, bottom, top, camera, vec3Str, zPlaneVertices)
}

func (s *Slicer) prepareRender(newWidth, newHeight int, left, right, bottom, top float32, camera mgl32.Mat4, vec3Str string, planeVertices []float32) error {
	if newWidth%2 == 1 {
		newWidth++
		newHeight++
	}

	if s.renderer == nil {
		return fmt.Errorf("renderer not initialized")
	}

	if err := s.renderer.Init(newWidth, newHeight, s.view); err != nil {
		return err
	}

	near, far := float32(0.1), float32(100.0)
	projection := mgl32.Ortho(left, right, bottom, top, near, far)
	model := mgl32.Ident4()

	return s.renderer.Prepare(s.irmf, vec3Str, planeVertices, projection, camera, model)
}

var xPlaneVertices = []float32{
	//  X, Y, Z, U, V
	0.0, -1.0, -1.0, 1.0, 0.0, // ll
	0.0, 1.0, -1.0, 0.0, 0.0, // lr
	0.0, -1.0, 1.0, 1.0, 1.0, // ul
	0.0, 1.0, -1.0, 0.0, 0.0, // lr
	0.0, 1.0, 1.0, 0.0, 1.0, // ur
	0.0, -1.0, 1.0, 1.0, 1.0, // ul
}

var yPlaneVertices = []float32{
	//  X, Y, Z, U, V
	-1.0, 0.0, -1.0, 1.0, 0.0, // ll
	1.0, 0.0, -1.0, 0.0, 0.0, // lr
	-1.0, 0.0, 1.0, 1.0, 1.0, // ul
	1.0, 0.0, -1.0, 0.0, 0.0, // lr
	1.0, 0.0, 1.0, 0.0, 1.0, // ur
	-1.0, 0.0, 1.0, 1.0, 1.0, // ul
}

var zPlaneVertices = []float32{
	//  X, Y, Z, U, V
	-1.0, -1.0, 0.0, 1.0, 0.0, // ll
	1.0, -1.0, 0.0, 0.0, 0.0, // lr
	-1.0, 1.0, 0.0, 1.0, 1.0, // ul
	1.0, -1.0, 0.0, 0.0, 0.0, // lr
	1.0, 1.0, 0.0, 0.0, 1.0, // ur
	-1.0, 1.0, 0.0, 1.0, 1.0, // ul
}
