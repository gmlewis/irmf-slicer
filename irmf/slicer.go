package irmf

import (
	"archive/zip"
	"fmt"
	"log"
	"os"

	"github.com/faiface/glhf"
	"github.com/faiface/mainthread"
	"github.com/go-gl/glfw/v3.1/glfw"
)

// Slice slices an IRMF shader into a ZIP containing many voxel slices
func (i *IRMF) Slice(zipName string, microns float64) error {
	zf, err := os.Create(zipName)
	if err != nil {
		return fmt.Errorf("Create: %v", err)
	}
	defer func() {
		if err := zf.Close(); err != nil {
			log.Fatalf("Unable to close %v: %v", zipName, err)
		}
	}()
	w := zip.NewWriter(zf)

	// Set up the global variables for the mainthread to run...
	shaderSrc = i.Shader
	mainthread.Run(run)

	// img, err := i.renderSlice(0.0, microns)
	// if err != nil {
	// 	return fmt.Errorf("renderSlice: %v", err)
	// }
	// n := 0
	// filename := fmt.Sprintf("slices/out%04d.png", n)
	// f, err := w.Create(filename)
	// if err != nil {
	// 	return fmt.Errorf("Unable to create ZIP file %q: %v", filename, err)
	// }
	// if err := png.Encode(f, img); err != nil {
	// 	return fmt.Errorf("PNG encode: %v", err)
	// }

	if err := w.Close(); err != nil {
		return fmt.Errorf("Unable to close ZIP: %v", err)
	}
	return nil
}

// Because of mainthread, we need to pass the values in as global
// variables. Fix this.
var shaderSrc string

func run() {
	var win *glfw.Window

	defer func() {
		mainthread.Call(func() {
			glfw.Terminate()
		})
	}()

	mainthread.Call(func() {
		glfw.Init()

		glfw.WindowHint(glfw.ContextVersionMajor, 3)
		glfw.WindowHint(glfw.ContextVersionMinor, 3)
		glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
		glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
		glfw.WindowHint(glfw.Resizable, glfw.False)

		var err error

		win, err = glfw.CreateWindow(512, 512, "IRMF Shader Slicer", nil, nil)
		if err != nil {
			log.Fatalf("glfw.CreateWindow: %v", err)
		}

		win.MakeContextCurrent()

		glhf.Init()
	})

	var (
		// The vertex format consists of names and types of the attributes. The name is the
		// name that the attribute is referenced by inside a shader.
		vertexFormat = glhf.AttrFormat{
			{Name: "position", Type: glhf.Vec2},
			{Name: "texture", Type: glhf.Vec2},
		}

		shader  *glhf.Shader
		texture *glhf.Texture
		slice   *glhf.VertexSlice
	)

	mainthread.Call(func() {
		var err error

		uniforms := glhf.AttrFormat{
			// From threejs:
			{Name: "modelMatrix", Type: glhf.Mat4},      // = object.matrixWorld
			{Name: "modelViewMatrix", Type: glhf.Mat4},  // = camera.matrixWorldInverse * object.matrixWorld
			{Name: "projectionMatrix", Type: glhf.Mat4}, // = camera.projectionMatrix
			{Name: "viewMatrix", Type: glhf.Mat4},       // = camera.matrixWorldInverse
			{Name: "normalMatrix", Type: glhf.Mat3},     // = inverse transpose of modelViewMatrix
			{Name: "cameraPosition", Type: glhf.Vec3},   // = camera position in world space
			// end from threejs.
			{Name: "u_z", Type: glhf.Float},
			{Name: "u_ll", Type: glhf.Vec3},
			{Name: "u_ur", Type: glhf.Vec3},
			{Name: "u_z", Type: glhf.Float},
			{Name: "u_numMaterials", Type: glhf.Int},
			{Name: "u_color1", Type: glhf.Vec4},
			{Name: "u_color2", Type: glhf.Vec4},
			{Name: "u_color3", Type: glhf.Vec4},
			{Name: "u_color4", Type: glhf.Vec4},
			{Name: "u_color5", Type: glhf.Vec4},
			{Name: "u_color6", Type: glhf.Vec4},
			{Name: "u_color7", Type: glhf.Vec4},
			{Name: "u_color8", Type: glhf.Vec4},
			{Name: "u_color9", Type: glhf.Vec4},
			{Name: "u_color10", Type: glhf.Vec4},
			{Name: "u_color11", Type: glhf.Vec4},
			{Name: "u_color12", Type: glhf.Vec4},
			{Name: "u_color13", Type: glhf.Vec4},
			{Name: "u_color14", Type: glhf.Vec4},
			{Name: "u_color15", Type: glhf.Vec4},
			{Name: "u_color16", Type: glhf.Vec4},
		}
		shader, err = glhf.NewShader(vertexFormat, uniforms, vertexShader, fsHeader+shaderSrc+fsFooter)
		if err != nil {
			log.Fatalf("unable to compile shaders: %v", err)
		}

		// And finally, we make a vertex slice, which is basically a dynamically sized
		// vertex array. The length of the slice is 6 and the capacity is the same.
		//
		// The slice inherits the vertex format of the supplied shader. Also, it should
		// only be used with that shader.
		slice = glhf.MakeVertexSlice(shader, 6, 6)

		// Before we use a slice, we need to Begin it. The same holds for all objects in
		// GLHF.
		slice.Begin()

		// We assign data to the vertex slice. The values are in the order as in the vertex
		// format of the slice (shader). Each two floats correspond to an attribute of type
		// glhf.Vec2.
		slice.SetVertexData([]float32{
			-1, -1, 0, 1,
			+1, -1, 1, 1,
			+1, +1, 1, 0,

			-1, -1, 0, 1,
			+1, +1, 1, 0,
			-1, +1, 0, 0,
		})

		// When we're done with the slice, we End it.
		slice.End()
	})

	// shouldQuit := false
	// for !shouldQuit {
	mainthread.Call(func() {
		// if win.ShouldClose() {
		// shouldQuit = true
		// }

		// Clear the window.
		glhf.Clear(1, 1, 1, 1)

		// Here we Begin/End all necessary objects and finally draw the vertex
		// slice.
		shader.Begin()
		texture.Begin()
		slice.Begin()
		slice.Draw()
		slice.End()
		texture.End()
		shader.End()

		win.SwapBuffers()
		glfw.PollEvents()
	})
	// }
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
