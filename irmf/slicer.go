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
	irmf    *IRMF
	window  *glfw.Window
	microns float64

	program      uint32
	modelUniform int32
	model        mgl32.Mat4
	vao          uint32
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
	return &Slicer{window: window, microns: micronsResolution}
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

	img, err := s.renderSlice(0.0)
	if err != nil {
		return fmt.Errorf("renderSlice: %v", err)
	}
	n := 0
	filename := fmt.Sprintf("slices/out%04d.png", n)
	f, err := w.Create(filename)
	if err != nil {
		return fmt.Errorf("Unable to create ZIP file %q: %v", filename, err)
	}
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("PNG encode: %v", err)
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

	// Maintenance
	s.window.SwapBuffers()
	glfw.PollEvents()

	// Get the image from the buffer.
	return nil, nil
}

func (s *Slicer) prepareRender() error {
	// Configure the vertex and fragment shaders
	var err error
	if s.program, err = newProgram(vertexShader, fsHeader+s.irmf.Shader+fsFooter); err != nil {
		return fmt.Errorf("newProgram: %v", err)
	}

	gl.UseProgram(s.program)

	width, height := s.window.GetFramebufferSize()
	projection := mgl32.Perspective(mgl32.DegToRad(45.0), float32(width)/float32(height), 0.1, 100.0)
	projectionUniform := gl.GetUniformLocation(s.program, gl.Str("projection\x00"))
	gl.UniformMatrix4fv(projectionUniform, 1, false, &projection[0])

	camera := mgl32.LookAtV(mgl32.Vec3{3, 3, 3}, mgl32.Vec3{0, 0, 0}, mgl32.Vec3{0, 1, 0})
	cameraUniform := gl.GetUniformLocation(s.program, gl.Str("camera\x00"))
	gl.UniformMatrix4fv(cameraUniform, 1, false, &camera[0])

	s.model = mgl32.Ident4()
	s.modelUniform = gl.GetUniformLocation(s.program, gl.Str("model\x00"))
	gl.UniformMatrix4fv(s.modelUniform, 1, false, &s.model[0])

	// textureUniform := gl.GetUniformLocation(s.program, gl.Str("tex\x00"))
	// gl.Uniform1i(textureUniform, 0)

	gl.BindFragDataLocation(s.program, 0, gl.Str("outputColor\x00"))

	// // Load the texture
	// texture, err := newTexture("square.png")
	// if err != nil {
	// 	log.Fatalln(err)
	// }

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

	// texCoordAttrib := uint32(gl.GetAttribLocation(program, gl.Str("vertTexCoord\x00")))
	// gl.EnableVertexAttribArray(texCoordAttrib)
	// gl.VertexAttribPointer(texCoordAttrib, 2, gl.FLOAT, false, 5*4, gl.PtrOffset(3*4))

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
// from threejs:
// #define varying in
//
// #define gl_FragDepthEXT gl_FragDepth
// #define texture2D texture
// #define textureCube texture
// #define texture2DProj textureProj
// #define texture2DLodEXT textureLod
// #define texture2DProjLodEXT textureProjLod
// #define textureCubeLodEXT textureLod
// #define texture2DGradEXT textureGrad
// #define texture2DProjGradEXT textureProjGrad
// #define textureCubeGradEXT textureGrad
// precision highp float;
// precision highp int;
// #define SHADER_NAME ShaderMaterial
// #define GAMMA_FACTOR 2
// #define DOUBLE_SIDED
// uniform mat4 viewMatrix;
// uniform vec3 cameraPosition;
// #define TONE_MAPPING
// #ifndef saturate
// 	#define saturate(a) clamp( a, 0.0, 1.0 )
// #endif
// uniform float toneMappingExposure;
// uniform float toneMappingWhitePoint;
// vec3 LinearToneMapping( vec3 color ) {
// 	return toneMappingExposure * color;
// }
// vec3 ReinhardToneMapping( vec3 color ) {
// 	color *= toneMappingExposure;
// 	return saturate( color / ( vec3( 1.0 ) + color ) );
// }
// #define Uncharted2Helper( x ) max( ( ( x * ( 0.15 * x + 0.10 * 0.50 ) + 0.20 * 0.02 ) / ( x * ( 0.15 * x + 0.50 ) + 0.20 * 0.30 ) ) - 0.02 / 0.30, vec3( 0.0 ) )
// vec3 Uncharted2ToneMapping( vec3 color ) {
// 	color *= toneMappingExposure;
// 	return saturate( Uncharted2Helper( color ) / Uncharted2Helper( vec3( toneMappingWhitePoint ) ) );
// }
// vec3 OptimizedCineonToneMapping( vec3 color ) {
// 	color *= toneMappingExposure;
// 	color = max( vec3( 0.0 ), color - 0.004 );
// 	return pow( ( color * ( 6.2 * color + 0.5 ) ) / ( color * ( 6.2 * color + 1.7 ) + 0.06 ), vec3( 2.2 ) );
// }
// vec3 ACESFilmicToneMapping( vec3 color ) {
// 	color *= toneMappingExposure;
// 	return saturate( ( color * ( 2.51 * color + 0.03 ) ) / ( color * ( 2.43 * color + 0.59 ) + 0.14 ) );
// }
// vec3 toneMapping( vec3 color ) { return LinearToneMapping( color ); }

// vec4 LinearToLinear( in vec4 value ) {
// 	return value;
// }
// vec4 GammaToLinear( in vec4 value, in float gammaFactor ) {
// 	return vec4( pow( value.rgb, vec3( gammaFactor ) ), value.a );
// }
// vec4 LinearToGamma( in vec4 value, in float gammaFactor ) {
// 	return vec4( pow( value.rgb, vec3( 1.0 / gammaFactor ) ), value.a );
// }
// vec4 sRGBToLinear( in vec4 value ) {
// 	return vec4( mix( pow( value.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), value.rgb * 0.0773993808, vec3( lessThanEqual( value.rgb, vec3( 0.04045 ) ) ) ), value.a );
// }
// vec4 LinearTosRGB( in vec4 value ) {
// 	return vec4( mix( pow( value.rgb, vec3( 0.41666 ) ) * 1.055 - vec3( 0.055 ), value.rgb * 12.92, vec3( lessThanEqual( value.rgb, vec3( 0.0031308 ) ) ) ), value.a );
// }
// vec4 RGBEToLinear( in vec4 value ) {
// 	return vec4( value.rgb * exp2( value.a * 255.0 - 128.0 ), 1.0 );
// }
// vec4 LinearToRGBE( in vec4 value ) {
// 	float maxComponent = max( max( value.r, value.g ), value.b );
// 	float fExp = clamp( ceil( log2( maxComponent ) ), -128.0, 127.0 );
// 	return vec4( value.rgb / exp2( fExp ), ( fExp + 128.0 ) / 255.0 );
// }
// vec4 RGBMToLinear( in vec4 value, in float maxRange ) {
// 	return vec4( value.rgb * value.a * maxRange, 1.0 );
// }
// vec4 LinearToRGBM( in vec4 value, in float maxRange ) {
// 	float maxRGB = max( value.r, max( value.g, value.b ) );
// 	float M = clamp( maxRGB / maxRange, 0.0, 1.0 );
// 	M = ceil( M * 255.0 ) / 255.0;
// 	return vec4( value.rgb / ( M * maxRange ), M );
// }
// vec4 RGBDToLinear( in vec4 value, in float maxRange ) {
// 	return vec4( value.rgb * ( ( maxRange / 255.0 ) / value.a ), 1.0 );
// }
// vec4 LinearToRGBD( in vec4 value, in float maxRange ) {
// 	float maxRGB = max( value.r, max( value.g, value.b ) );
// 	float D = max( maxRange / maxRGB, 1.0 );
// 	D = min( floor( D ) / 255.0, 1.0 );
// 	return vec4( value.rgb * ( D * ( 255.0 / maxRange ) ), D );
// }
// const mat3 cLogLuvM = mat3( 0.2209, 0.3390, 0.4184, 0.1138, 0.6780, 0.7319, 0.0102, 0.1130, 0.2969 );
// vec4 LinearToLogLuv( in vec4 value )  {
// 	vec3 Xp_Y_XYZp = cLogLuvM * value.rgb;
// 	Xp_Y_XYZp = max( Xp_Y_XYZp, vec3( 1e-6, 1e-6, 1e-6 ) );
// 	vec4 vResult;
// 	vResult.xy = Xp_Y_XYZp.xy / Xp_Y_XYZp.z;
// 	float Le = 2.0 * log2(Xp_Y_XYZp.y) + 127.0;
// 	vResult.w = fract( Le );
// 	vResult.z = ( Le - ( floor( vResult.w * 255.0 ) ) / 255.0 ) / 255.0;
// 	return vResult;
// }
// const mat3 cLogLuvInverseM = mat3( 6.0014, -2.7008, -1.7996, -1.3320, 3.1029, -5.7721, 0.3008, -1.0882, 5.6268 );
// vec4 LogLuvToLinear( in vec4 value ) {
// 	float Le = value.z * 255.0 + value.w;
// 	vec3 Xp_Y_XYZp;
// 	Xp_Y_XYZp.y = exp2( ( Le - 127.0 ) / 2.0 );
// 	Xp_Y_XYZp.z = Xp_Y_XYZp.y / value.y;
// 	Xp_Y_XYZp.x = value.x * Xp_Y_XYZp.z;
// 	vec3 vRGB = cLogLuvInverseM * Xp_Y_XYZp.rgb;
// 	return vec4( max( vRGB, 0.0 ), 1.0 );
// }
// vec4 mapTexelToLinear( vec4 value ) { return LinearToLinear( value ); }
// vec4 matcapTexelToLinear( vec4 value ) { return LinearToLinear( value ); }
// vec4 envMapTexelToLinear( vec4 value ) { return LinearToLinear( value ); }
// vec4 emissiveMapTexelToLinear( vec4 value ) { return LinearToLinear( value ); }
// vec4 linearToOutputTexel( vec4 value ) { return LinearToLinear( value ); }
// end from threejs.

uniform vec3 u_ll;
uniform vec3 u_ur;
out vec4 v_xyz;
void main() {
  gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
  v_xyz = modelMatrix * vec4( position, 1.0 );
}
`

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
  if (any(lessThanEqual(abs(v_xyz.xyz),u_ll))) {
    // out_FragColor = vec4(1);  // DEBUG
    return;
  }
  if (any(greaterThanEqual(abs(v_xyz.xyz),u_ur))) {
    // out_FragColor = vec4(1);  // DEBUG
    return;
  }

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
}
`

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
