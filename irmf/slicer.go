package irmf

import (
	"archive/zip"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
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

	program      uint32
	modelUniform int32
	model        mgl32.Mat4
	vao          uint32
	uZUniform    int32

	texture uint32
	// time         float64
	// angle        float64
	// previousTime float64
}

// Init returns a new Slicer instance.
func Init(view bool, width, height int, micronsResolution float64) *Slicer {
	// TODO: Support units other than millimeters.
	return &Slicer{width: width, height: height, delta: micronsResolution / 1000.0}
}

// New prepares the slicer to slice a new shader model.
func (s *Slicer) New(shaderSrc string) (*IRMF, error) {
	var err error
	s.irmf, err = newModel(shaderSrc)
	return s.irmf, err
}

// Close closes the GLFW window and releases any Slicer resources.
func (s *Slicer) Close() {
	glfw.Terminate()
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
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	s.window, err = glfw.CreateWindow(width, height, "IRMF Slicer", nil, nil)
	check("CreateWindow(%v,%v): %v", width, height, err)
	s.window.MakeContextCurrent()

	err = gl.Init()
	check("gl.Init: %v", err)

	version := gl.GoStr(gl.GetString(gl.VERSION))
	fmt.Println("OpenGL version", version)
}

// Slice slices an IRMF shader into a ZIP containing many voxel slices
func (s *Slicer) Slice(zipName string) error {
	zf, err := os.Create(zipName)
	check("Create: %v", err)
	defer func() {
		err := zf.Close()
		check("zip close: %v", err)
	}()
	w := zip.NewWriter(zf)

	if err := s.prepareRender(); err != nil {
		return fmt.Errorf("compile shader: %v", err)
	}

	var n int
	for z := s.irmf.Min[2] + 0.5*s.delta; z <= s.irmf.Max[2]; z += s.delta {
		img, err := s.renderSlice(z)
		if err != nil {
			return fmt.Errorf("renderSlice: %v", err)
		}
		filename := fmt.Sprintf("slices/out%04d.png", n)
		fh := &zip.FileHeader{
			Name:     filename,
			Comment:  fmt.Sprintf("z=%0.2f", z),
			Modified: time.Now(),
		}
		f, err := w.CreateHeader(fh)
		if err != nil {
			return fmt.Errorf("Unable to create ZIP file %q: %v", filename, err)
		}
		if err := png.Encode(f, img); err != nil {
			return fmt.Errorf("PNG encode: %v", err)
		}
		n++
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("Unable to close ZIP: %v", err)
	}
	return nil
}

func (s *Slicer) renderSlice(z float64) (image.Image, error) {
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// Render
	gl.UseProgram(s.program)
	gl.UniformMatrix4fv(s.modelUniform, 1, false, &s.model[0])
	gl.Uniform1f(s.uZUniform, float32(z))

	gl.BindVertexArray(s.vao)

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, s.texture)

	gl.DrawArrays(gl.TRIANGLES, 0, 2*3) // 6*2*3)

	width, height := s.window.GetFramebufferSize()
	rgba := &image.RGBA{
		Pix:    make([]uint8, width*height*4),
		Stride: width * 4, // bytes between vertically adjacent pixels.
		Rect:   image.Rect(0, 0, width, height),
	}
	gl.ReadPixels(0, 0, int32(width), int32(height), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(&rgba.Pix[0]))

	if gl.GetError() != gl.NO_ERROR {
		fmt.Println("GL ERROR Somewhere!")
	}

	// Maintenance
	s.window.SwapBuffers()
	glfw.PollEvents()

	return rgba, nil
}

func (s *Slicer) prepareRender() error {
	// Create or resize window if necessary.
	left := float32(s.irmf.Min[0])
	right := float32(s.irmf.Max[0])
	bottom := float32(s.irmf.Min[1])
	top := float32(s.irmf.Max[1])
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
	log.Printf("MBB=(%v,%v)-(%v,%v), frustumSize=%v, aspectRatio=%v", left, bottom, right, top, frustumSize, aspectRatio)

	// Configure the vertex and fragment shaders
	var err error
	if s.program, err = newProgram(vertexShader, fsHeader+s.irmf.Shader+fsFooter); err != nil {
		// if s.program, err = newProgram(vertexShader, fsHeader); err != nil {
		return fmt.Errorf("newProgram: %v", err)
	}

	gl.UseProgram(s.program)

	projection := mgl32.Ortho(left, right, bottom, top, near, far)
	// projection := mgl32.Ortho(-aspectRatio*frustumSize, aspectRatio*frustumSize, -frustumSize, frustumSize, near, far)
	// projection := mgl32.Perspective(mgl32.DegToRad(45.0), float32(width)/float32(height), 0.1, 10.0)
	projectionUniform := gl.GetUniformLocation(s.program, gl.Str("projection\x00"))
	gl.UniformMatrix4fv(projectionUniform, 1, false, &projection[0])

	camera := mgl32.LookAtV(mgl32.Vec3{0, 0, 3}, mgl32.Vec3{0, 0, 0}, mgl32.Vec3{0, 1, 0})
	// cameraUniform := gl.GetUniformLocation(s.program, gl.Str("camera\x00"))
	// gl.UniformMatrix4fv(cameraUniform, 1, false, &camera[0])

	// camera := mgl32.Ortho(left, right, bottom, top, near, far)
	cameraUniform := gl.GetUniformLocation(s.program, gl.Str("camera\x00"))
	gl.UniformMatrix4fv(cameraUniform, 1, false, &camera[0])

	s.model = mgl32.Ident4()
	s.modelUniform = gl.GetUniformLocation(s.program, gl.Str("model\x00"))
	gl.UniformMatrix4fv(s.modelUniform, 1, false, &s.model[0])

	// Set up uniforms needed by shaders:
	uLL := mgl32.Vec3{left, bottom, float32(s.irmf.Min[2])}
	uLLUniform := gl.GetUniformLocation(s.program, gl.Str("u_ll\x00"))
	gl.Uniform3fv(uLLUniform, 1, &uLL[0])
	uUR := mgl32.Vec3{right, top, float32(s.irmf.Max[2])}
	uURUniform := gl.GetUniformLocation(s.program, gl.Str("u_ur\x00"))
	gl.Uniform3fv(uURUniform, 1, &uUR[0])
	uZ := float32(0)
	s.uZUniform = gl.GetUniformLocation(s.program, gl.Str("u_z\x00"))
	gl.Uniform1f(s.uZUniform, uZ)
	uNumMaterials := int32(len(s.irmf.Materials))
	uNumMaterialsUniform := gl.GetUniformLocation(s.program, gl.Str("u_numMaterials\x00"))
	gl.Uniform1i(uNumMaterialsUniform, uNumMaterials)

	// uniform vec4
	// uniform vec4
	// uniform vec4
	// uniform vec4
	// uniform vec4
	// uniform vec4
	// uniform vec4
	// uniform vec4
	// uniform vec4
	// uniform vec4 ;
	// uniform vec4 ;
	// uniform vec4 ;
	// uniform vec4 ;
	// uniform vec4 ;
	// uniform vec4 ;
	// uniform vec4 ;

	// gl.BindFragDataLocation(s.program, 0, gl.Str("outputColor\x00"))
	gl.BindFragDataLocation(s.program, 0, gl.Str("outputColor\x00"))

	// Load the texture
	s.texture, err = newTexture("square.png")
	if err != nil {
		log.Fatalln(err)
	}

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

	texCoordAttrib := uint32(gl.GetAttribLocation(s.program, gl.Str("vertTexCoord\x00")))
	gl.EnableVertexAttribArray(texCoordAttrib)
	gl.VertexAttribPointer(texCoordAttrib, 2, gl.FLOAT, false, 5*4, gl.PtrOffset(3*4))

	// Configure global settings
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)
	gl.ClearColor(0.0, 0.0, 0.0, 0.0)

	// s.angle = 0.0
	// s.previousTime = glfw.GetTime()

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

func newTexture(file string) (uint32, error) {
	imgFile, err := os.Open(file)
	if err != nil {
		return 0, fmt.Errorf("texture %q not found on disk: %v", file, err)
	}
	img, _, err := image.Decode(imgFile)
	if err != nil {
		return 0, err
	}

	rgba := image.NewRGBA(img.Bounds())
	if rgba.Stride != rgba.Rect.Size().X*4 {
		return 0, fmt.Errorf("unsupported stride")
	}
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{0, 0}, draw.Src)

	var texture uint32
	gl.GenTextures(1, &texture)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, texture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.RGBA,
		int32(rgba.Rect.Size().X),
		int32(rgba.Rect.Size().Y),
		0,
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		gl.Ptr(rgba.Pix))

	return texture, nil
}

const vertexShader = `#version 300 es
uniform mat4 projection;
uniform mat4 camera;
uniform mat4 model;
in vec3 vert;
in vec2 vertTexCoord;
out vec2 fragTexCoord;
out vec3 fragVert;
void main() {
	fragTexCoord = vertTexCoord;
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
`

const fsFooter = `
void main() {
	vec4 materials;
	mainModel4(materials, vec3(fragVert.xy,u_z));
	outputColor = vec4(materials.x);
}
` + "\x00"

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
