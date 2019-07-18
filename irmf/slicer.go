package irmf

import (
	"archive/zip"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"runtime"
	"strings"

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
	irmf   *IRMF
	window *glfw.Window
	delta  float64 // millimeters (model units)

	program      uint32
	modelUniform int32
	model        mgl32.Mat4
	vao          uint32
	uZUniform    int32
}

// Init initializes GLFW and OpenGL for rendering.
func Init(view bool, width, height int, micronsResolution float64) *Slicer {
	err := glfw.Init()
	check("glfw.Init: %v", err)

	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	window, err := glfw.CreateWindow(width, height, "IRMF Slicer", nil, nil)
	check("CreateWindow(%v,%v): %v", width, height, err)
	window.MakeContextCurrent()

	err = gl.Init()
	check("gl.Init: %v", err)

	version := gl.GoStr(gl.GetString(gl.VERSION))
	fmt.Println("OpenGL version", version)
	// TODO: Support units other than millimeters.
	return &Slicer{window: window, delta: micronsResolution / 1000.0}
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
	for z := s.irmf.Min[2] + s.delta; z <= s.irmf.Max[2]; z += s.delta {
		img, err := s.renderSlice(0.0)
		if err != nil {
			return fmt.Errorf("renderSlice: %v", err)
		}
		filename := fmt.Sprintf("slices/out%04d.png", n)
		f, err := w.Create(filename)
		if err != nil {
			return fmt.Errorf("Unable to create ZIP file %q: %v", filename, err)
		}
		if err := png.Encode(f, img); err != nil {
			return fmt.Errorf("PNG encode: %v", err)
		}
		n++

		s.window.SwapBuffers()
		glfw.PollEvents()
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("Unable to close ZIP: %v", err)
	}
	return nil
}

func (s *Slicer) renderSlice(z float64) (image.Image, error) {
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// Update
	// time := glfw.GetTime()
	// elapsed := time - previousTime
	// previousTime = time
	// angle += elapsed
	// model = mgl32.HomogRotate3D(float32(angle), mgl32.Vec3{0, 1, 0})

	// Render
	gl.UseProgram(s.program)
	gl.UniformMatrix4fv(s.modelUniform, 1, false, &s.model[0])

	gl.BindVertexArray(s.vao)

	// gl.ActiveTexture(gl.TEXTURE0)
	// gl.BindTexture(gl.TEXTURE_2D, texture)

	gl.DrawArrays(gl.TRIANGLES, 0, 2*3) // 6*2*3)

	width, height := s.window.GetFramebufferSize()
	rgba := &image.RGBA{
		Pix:    make([]uint8, width*height*4),
		Stride: width * 4, // bytes between vertically adjacent pixels.
		Rect:   image.Rect(0, 0, width, height),
	}
	gl.ReadPixels(0, 0, int32(width), int32(height), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(&rgba.Pix[0]))

	// Maintenance
	s.window.SwapBuffers()
	glfw.PollEvents()

	// Get the image from the buffer.
	return rgba, nil
}

func (s *Slicer) prepareRender() error {
	// Configure the vertex and fragment shaders
	var err error
	if s.program, err = newProgram(vertexShader, fsHeader+s.irmf.Shader+fsFooter); err != nil {
		return fmt.Errorf("newProgram: %v", err)
	}

	gl.UseProgram(s.program)

	left := float32(s.irmf.Min[0])
	right := float32(s.irmf.Max[0])
	bottom := float32(s.irmf.Min[1])
	top := float32(s.irmf.Max[1])
	// near, far := float32(0.1), float32(100.0)
	// camera := mgl32.Ortho(left, right, bottom, top, near, far)
	// cameraUniform := gl.GetUniformLocation(s.program, gl.Str("camera\x00"))
	// gl.UniformMatrix4fv(cameraUniform, 1, false, &camera[0])

	// s.model = mgl32.Ident4()
	// s.modelUniform = gl.GetUniformLocation(s.program, gl.Str("model\x00"))
	// gl.UniformMatrix4fv(s.modelUniform, 1, false, &s.model[0])

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

	// uniform vec4 u_color1;
	// uniform vec4 u_color2;
	// uniform vec4 u_color3;
	// uniform vec4 u_color4;
	// uniform vec4 u_color5;
	// uniform vec4 u_color6;
	// uniform vec4 u_color7;
	// uniform vec4 u_color8;
	// uniform vec4 u_color9;
	// uniform vec4 u_color10;
	// uniform vec4 u_color11;
	// uniform vec4 u_color12;
	// uniform vec4 u_color13;
	// uniform vec4 u_color14;
	// uniform vec4 u_color15;
	// uniform vec4 u_color16;

	gl.BindFragDataLocation(s.program, 0, gl.Str("out_FragColor\x00"))

	// Configure the vertex data
	gl.GenVertexArrays(1, &s.vao)
	gl.BindVertexArray(s.vao)

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
	gl.ClearColor(0.0, 0.0, 0.0, 1.0)

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
// uniform mat4 camera;
// uniform mat4 model;

in vec3 vert;
// in vec2 vertTexCoord;

// out vec2 fragTexCoord;
out vec4 v_xyz;

void main() {
  // fragTexCoord = vertTexCoord;
  // gl_Position = projection * camera * model * vec4(vert, 1);
  v_xyz = vec4(vert,1);
}
` + "\x00"

const fsHeader = `#version 300 es
precision highp float;
precision highp int;
uniform vec3 u_ll;
uniform vec3 u_ur;
uniform float u_z;
uniform int u_numMaterials;
uniform vec4 u_color1;
uniform vec4 u_color2;
uniform vec4 u_color3;
uniform vec4 u_color4;
uniform vec4 u_color5;
uniform vec4 u_color6;
uniform vec4 u_color7;
uniform vec4 u_color8;
uniform vec4 u_color9;
uniform vec4 u_color10;
uniform vec4 u_color11;
uniform vec4 u_color12;
uniform vec4 u_color13;
uniform vec4 u_color14;
uniform vec4 u_color15;
uniform vec4 u_color16;
in vec4 v_xyz;
out vec4 out_FragColor;
`

const fsFooter = `
void main() {
  // if (any(lessThanEqual(abs(v_xyz.xyz),u_ll))) {
  //   // out_FragColor = vec4(1);  // DEBUG
  //   return;
  // }
  // if (any(greaterThanEqual(abs(v_xyz.xyz),u_ur))) {
  //   // out_FragColor = vec4(1);  // DEBUG
  //   return;
  // }

  if (u_numMaterials <= 4) {
    vec4 materials;
    mainModel4(materials, v_xyz.xyz);
    switch(u_numMaterials) {
    case 1:
      out_FragColor = u_color1*materials.x;
      break;
    case 2:
      out_FragColor = u_color1*materials.x + u_color2*materials.y;
      break;
    case 3:
      out_FragColor = u_color1*materials.x + u_color2*materials.y + u_color3*materials.z;
      break;
    case 4:
      out_FragColor = u_color1*materials.x + u_color2*materials.y + u_color3*materials.z + u_color4*materials.w;
      break;
    }
    // out_FragColor = v_xyz/5.0 + 0.5;  // DEBUG
    // out_FragColor = vec4(vec3(d), 1.);  // DEBUG
  // } else if (u_numMaterials <= 9) {

  // } else if (u_numMaterials <= 16) {

	}
	out_FragColor = vec4(1,0,0, 1);  // DEBUG
}
` + "\x00"

var planeVertices = []float32{
	//  X, Y, Z, U, V
	-1.0, -1.0, 0.0, 1.0, 0.0,
	1.0, -1.0, 0.0, 0.0, 0.0,
	-1.0, 1.0, 0.0, 1.0, 1.0,
	1.0, -1.0, 0.0, 0.0, 0.0,
	1.0, 1.0, 0.0, 0.0, 1.0,
	-1.0, 1.0, 0.0, 1.0, 1.0,
}

func check(fmtStr string, args ...interface{}) {
	err := args[len(args)-1]
	if err != nil {
		log.Fatalf(fmtStr, args...)
	}
}
