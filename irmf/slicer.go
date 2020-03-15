package irmf

import (
	"fmt"
	"image"
	"log"
	"runtime"
	"strings"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

func init() {
	// GLFW event handling must run on the main OS thread
	runtime.LockOSThread()
}

// Slicer represents a slicer context.
type Slicer struct {
	irmf          *IRMF
	width, height int
	window        *glfw.Window
	delta         float64 // millimeters (model units)
	view          bool

	program uint32
	model   mgl32.Mat4
	vao     uint32

	modelUniform        int32
	uMaterialNumUniform int32
	uZUniform           int32
}

// Init returns a new Slicer instance.
func Init(view bool, width, height int, micronsResolution float64) *Slicer {
	// TODO: Support units other than millimeters.
	return &Slicer{width: width, height: height, delta: micronsResolution / 1000.0, view: view}
}

// NewModel prepares the slicer to slice a new shader model.
func (s *Slicer) NewModel(shaderSrc []byte) error {
	irmf, err := newModel(shaderSrc)
	s.irmf = irmf
	return err
}

// Close closes the GLFW window and releases any Slicer resources.
func (s *Slicer) Close() {
	glfw.Terminate()
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
	if s.irmf == nil || n >= len(s.irmf.Materials) {
		return ""
	}
	return s.irmf.Materials[n-1]
}

// MBB returns the MBB of the IRMF model.
func (s *Slicer) MBB() (min, max [3]float64) {
	if s.irmf != nil {
		if len(s.irmf.Min) != 3 || len(s.irmf.Max) != 3 {
			log.Fatalf("Bad IRMF model: min=%#v, max=%#v", s.irmf.Min, s.irmf.Max)
		}
		min[0], min[1], min[2] = s.irmf.Min[0], s.irmf.Min[1], s.irmf.Min[2]
		max[0], max[1], max[2] = s.irmf.Max[0], s.irmf.Max[1], s.irmf.Max[2]
	}
	return min, max
}

func (s *Slicer) createOrResizeWindow(width, height int) {
	if s.window != nil {
		glfw.Terminate()
	}
	s.width = width
	s.height = height

	err := glfw.Init()
	check("glfw.Init: %v", err)

	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	if !s.view {
		glfw.WindowHint(glfw.Visible, glfw.False)
	}
	s.window, err = glfw.CreateWindow(width, height, "IRMF Slicer", nil, nil)
	check("CreateWindow(%v,%v): %v", width, height, err)
	s.window.MakeContextCurrent()

	err = gl.Init()
	check("gl.Init: %v", err)

	version := gl.GoStr(gl.GetString(gl.VERSION))
	fmt.Println("OpenGL version", version)
}

// SliceProcessor represents a slice processor.
type SliceProcessor interface {
	ProcessSlice(sliceNum int, depth float64, img image.Image) error
}

// Order represents the order of slice processing.
type Order byte

const (
	MinToMax Order = iota
	MaxToMin
)

// RenderZSlices slices the given materialNum (1-based index)
// to an image, calling the SliceProcessor for each slice.
func (s *Slicer) RenderZSlices(materialNum int, sp SliceProcessor, order Order) error {
	var (
		n     int
		min   float64
		max   float64
		delta float64
	)

	switch order {
	case MinToMax:
		min, max, delta = s.irmf.Min[2]+0.5*s.delta, s.irmf.Max[2], s.delta
	case MaxToMin:
		min, max, delta = s.irmf.Max[2]-0.5*s.delta, s.irmf.Min[2], -s.delta
	}

	for z := min; z <= max; z += delta {
		img, err := s.renderZSlice(z, materialNum)
		if err != nil {
			return fmt.Errorf("renderZSlice(%v,%v): %v", z, materialNum, err)
		}
		if err := sp.ProcessSlice(n, z, img); err != nil {
			return fmt.Errorf("ProcessSlice(%v,%v): %v", n, z, err)
		}
		n++
	}
	return nil
}

func (s *Slicer) renderZSlice(z float64, materialNum int) (image.Image, error) {
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// Render
	gl.UseProgram(s.program)
	gl.UniformMatrix4fv(s.modelUniform, 1, false, &s.model[0])
	gl.Uniform1f(s.uZUniform, float32(z))
	gl.Uniform1i(s.uMaterialNumUniform, int32(materialNum))

	gl.BindVertexArray(s.vao)

	gl.DrawArrays(gl.TRIANGLES, 0, 2*3) // 6*2*3)

	width, height := s.window.GetFramebufferSize()
	rgba := &image.RGBA{
		Pix:    make([]uint8, width*height*4),
		Stride: width * 4, // bytes between vertically adjacent pixels.
		Rect:   image.Rect(0, 0, width, height),
	}
	gl.ReadPixels(0, 0, int32(width), int32(height), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(&rgba.Pix[0]))

	// The error appears to be related to unused variables, but it doesn't affect anything.
	// if gl.GetError() != gl.NO_ERROR {
	// 	fmt.Println("GL ERROR Somewhere!")
	// }

	// Maintenance
	s.window.SwapBuffers()
	glfw.PollEvents()

	return rgba, nil
}

func (s *Slicer) PrepareRenderPlusZ() error {
	left := float32(s.irmf.Min[0])
	right := float32(s.irmf.Max[0])
	bottom := float32(s.irmf.Min[1])
	top := float32(s.irmf.Max[1])
	camera := mgl32.LookAtV(mgl32.Vec3{0, 0, 3}, mgl32.Vec3{0, 0, 0}, mgl32.Vec3{0, 1, 0})
	return s.prepareRender(left, right, bottom, top, camera)
}

func (s *Slicer) prepareRender(left, right, bottom, top float32, camera mgl32.Mat4) error {
	// Create or resize window if necessary.
	near, far := float32(0.1), float32(100.0)
	aspectRatio := (right - left) / (top - bottom)
	frustumSize := float32(s.height)
	resize := false
	if aspectRatio*frustumSize < float32(s.width) {
		frustumSize = float32(s.width) / aspectRatio
		resize = true
	}
	if s.window == nil || resize {
		s.createOrResizeWindow(int(aspectRatio*frustumSize), int(frustumSize))
	}

	// Configure the vertex and fragment shaders
	var err error
	if s.program, err = newProgram(vertexShader, fsHeader+s.irmf.Shader+genFooter(len(s.irmf.Materials))); err != nil {
		return fmt.Errorf("newProgram: %v", err)
	}

	gl.UseProgram(s.program)

	projection := mgl32.Ortho(left, right, bottom, top, near, far)
	projectionUniform := gl.GetUniformLocation(s.program, gl.Str("projection\x00"))
	gl.UniformMatrix4fv(projectionUniform, 1, false, &projection[0])

	cameraUniform := gl.GetUniformLocation(s.program, gl.Str("camera\x00"))
	gl.UniformMatrix4fv(cameraUniform, 1, false, &camera[0])

	s.model = mgl32.Ident4()
	s.modelUniform = gl.GetUniformLocation(s.program, gl.Str("model\x00"))
	gl.UniformMatrix4fv(s.modelUniform, 1, false, &s.model[0])

	// Set up uniforms needed by shaders:
	uZ := float32(0)
	s.uZUniform = gl.GetUniformLocation(s.program, gl.Str("u_z\x00"))
	gl.Uniform1f(s.uZUniform, uZ)
	uMaterialNum := int32(1)
	s.uMaterialNumUniform = gl.GetUniformLocation(s.program, gl.Str("u_materialNum\x00"))
	gl.Uniform1i(s.uMaterialNumUniform, uMaterialNum)

	gl.BindFragDataLocation(s.program, 0, gl.Str("outputColor\x00"))

	// Configure the vertex data
	gl.GenVertexArrays(1, &s.vao)
	gl.BindVertexArray(s.vao)

	planeVertices[0], planeVertices[10], planeVertices[25] = left, left, left
	planeVertices[5], planeVertices[15], planeVertices[20] = right, right, right
	planeVertices[1], planeVertices[6], planeVertices[16] = bottom, bottom, bottom
	planeVertices[11], planeVertices[21], planeVertices[26] = top, top, top

	var vbo uint32
	gl.GenBuffers(1, &vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(planeVertices)*4, gl.Ptr(planeVertices), gl.STATIC_DRAW)

	vertAttrib := uint32(gl.GetAttribLocation(s.program, gl.Str("vert\x00")))
	gl.EnableVertexAttribArray(vertAttrib)
	gl.VertexAttribPointer(vertAttrib, 3, gl.FLOAT, false, 5*4, gl.PtrOffset(0))

	// Configure global settings
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)
	gl.ClearColor(0.0, 0.0, 0.0, 0.0)

	return nil
}

func newProgram(vertexShaderSource, fragmentShaderSource string) (uint32, error) {
	vertexShader, err := compileShader(vertexShaderSource, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}

	fragmentShader, err := compileShader(fragmentShaderSource, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, err
	}

	program := gl.CreateProgram()

	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to link program: %v", log)
	}

	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program, nil
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to compile %v: %v", source, log)
	}

	return shader, nil
}

const vertexShader = `#version 300 es
uniform mat4 projection;
uniform mat4 camera;
uniform mat4 model;
in vec3 vert;
out vec3 fragVert;
void main() {
	gl_Position = projection * camera * model * vec4(vert, 1);
	fragVert = vert;
}
` + "\x00"

const fsHeader = `#version 300 es
precision highp float;
precision highp int;
in vec3 fragVert;
out vec4 outputColor;
uniform float u_z;
uniform int u_materialNum;
`

var planeVertices = []float32{
	//  X, Y, Z, U, V
	-1.0, -1.0, 0.0, 1.0, 0.0, // ll
	1.0, -1.0, 0.0, 0.0, 0.0, // lr
	-1.0, 1.0, 0.0, 1.0, 1.0, // ul
	1.0, -1.0, 0.0, 0.0, 0.0, // lr
	1.0, 1.0, 0.0, 0.0, 1.0, // ur
	-1.0, 1.0, 0.0, 1.0, 1.0, // ul
}

func check(fmtStr string, args ...interface{}) {
	err := args[len(args)-1]
	if err != nil {
		log.Fatalf(fmtStr, args...)
	}
}

func genFooter(numMaterials int) string {
	switch numMaterials {
	default:
		return fsFooterFmt4 + "\x00"
	case 5, 6, 7, 8, 9:
		return fsFooterFmt9 + "\x00"
	case 10, 11, 12, 13, 14, 15, 16:
		return fsFooterFmt16 + "\x00"
	}
}

const fsFooterFmt4 = `
void main() {
  vec4 m;
  mainModel4(m, vec3(fragVert.xy,u_z));
  switch(u_materialNum) {
  case 1:
    outputColor = vec4(m.x);
    break;
  case 2:
    outputColor = vec4(m.y);
    break;
  case 3:
    outputColor = vec4(m.z);
    break;
  case 4:
    outputColor = vec4(m.w);
    break;
  }
}
`

const fsFooterFmt9 = `
void main() {
  mat3 m;
  mainModel9(m, vec3(fragVert.xy,u_z));
  switch(u_materialNum) {
  case 1:
    outputColor = vec4(m[0][0]);
    break;
  case 2:
    outputColor = vec4(m[0][1]);
    break;
  case 3:
    outputColor = vec4(m[0][2]);
    break;
  case 4:
    outputColor = vec4(m[1][0]);
    break;
  case 5:
    outputColor = vec4(m[1][1]);
    break;
  case 6:
    outputColor = vec4(m[1][2]);
    break;
  case 7:
    outputColor = vec4(m[2][0]);
    break;
  case 8:
    outputColor = vec4(m[2][1]);
    break;
  case 9:
    outputColor = vec4(m[2][2]);
    break;
  }
}
`

const fsFooterFmt16 = `
void main() {
  mat4 m;
  mainModel16(m, vec3(fragVert.xy,u_z));
  switch(u_materialNum) {
  case 1:
    outputColor = vec4(m[0][0]);
    break;
  case 2:
    outputColor = vec4(m[0][1]);
    break;
  case 3:
    outputColor = vec4(m[0][2]);
    break;
  case 4:
    outputColor = vec4(m[0][3]);
    break;
  case 5:
    outputColor = vec4(m[1][0]);
    break;
  case 6:
    outputColor = vec4(m[1][1]);
    break;
  case 7:
    outputColor = vec4(m[1][2]);
    break;
  case 8:
    outputColor = vec4(m[1][3]);
    break;
  case 9:
    outputColor = vec4(m[2][0]);
    break;
  case 10:
    outputColor = vec4(m[2][1]);
    break;
  case 11:
    outputColor = vec4(m[2][2]);
    break;
  case 12:
    outputColor = vec4(m[2][3]);
    break;
  case 13:
    outputColor = vec4(m[3][0]);
    break;
  case 14:
    outputColor = vec4(m[3][1]);
    break;
  case 15:
    outputColor = vec4(m[3][2]);
    break;
  case 16:
    outputColor = vec4(m[3][3]);
    break;
  }
}
`
