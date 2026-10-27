package irmf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/go-gl/mathgl/mgl32"
)

// ErrTSLCompilerUnavailable is returned when a TSL model is sliced but no
// irmf-tsl-compile executable can be found.
var ErrTSLCompilerUnavailable = errors.New(
	"TSL compiler not available: set IRMF_TSL_COMPILER to the irmf-tsl-compile " +
		"executable or add `irmf-tsl-compile` to PATH (see irmf-examples/tools/tsl-compiler)")

// TSLCompiler compiles a TSL body into a standalone WGSL function.
type TSLCompiler interface {
	Compile(ctx context.Context, source string, materials int) (string, error)
}

// ExternalTSLCompiler invokes the shared Node compiler
// (irmf-examples/tools/tsl-compiler) as a subprocess.
type ExternalTSLCompiler struct {
	Path     string        // defaults to $IRMF_TSL_COMPILER or PATH
	CacheDir string        // defaults to $IRMF_TSL_CACHE_DIR
	Timeout  time.Duration // defaults to 30s
}

type tslDiagnostic struct {
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Message string `json:"message"`
}

type tslCompileResult struct {
	OK          bool            `json:"ok"`
	Source      string          `json:"source"`
	Diagnostics []tslDiagnostic `json:"diagnostics"`
}

// Compile implements TSLCompiler.
func (c *ExternalTSLCompiler) Compile(ctx context.Context, source string, materials int) (string, error) {
	path := c.Path
	if path == "" {
		path = locateTSLCompiler()
	}
	if path == "" {
		return "", ErrTSLCompilerUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cacheDir := c.CacheDir
	if cacheDir == "" {
		cacheDir = os.Getenv("IRMF_TSL_CACHE_DIR")
	}

	args := []string{"--json", "--materials", strconv.Itoa(materials), "-"}
	if cacheDir != "" {
		args = append(args, "--cache-dir", cacheDir)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = strings.NewReader(source)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("TSL compile timed out after %v", timeout)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("TSL compile failed: %v: %v", err, detail)
		}
		return "", fmt.Errorf("TSL compile failed: %v", err)
	}

	var res tslCompileResult
	if err := json.Unmarshal(out, &res); err != nil {
		return "", fmt.Errorf("TSL compiler returned invalid JSON: %v", err)
	}
	if !res.OK {
		if len(res.Diagnostics) > 0 {
			d := res.Diagnostics[0]
			return "", fmt.Errorf("TSL compile error at line %d:%d: %s", d.Line, d.Col, d.Message)
		}
		return "", errors.New("TSL compile error")
	}
	if res.Source == "" {
		return "", errors.New("TSL compiler returned an empty shader")
	}
	return res.Source, nil
}

func locateTSLCompiler() string {
	if p := os.Getenv("IRMF_TSL_COMPILER"); p != "" {
		return p
	}
	if p, err := exec.LookPath("irmf-tsl-compile"); err == nil {
		return p
	}
	return ""
}

var tslCompilerOverride TSLCompiler

// SetTSLCompiler overrides the compiler used by TSLRenderer. Pass nil to
// restore the default external compiler. Intended for tests.
func SetTSLCompiler(c TSLCompiler) {
	tslCompilerOverride = c
}

func activeTSLCompiler() TSLCompiler {
	if tslCompilerOverride != nil {
		return tslCompilerOverride
	}
	return &ExternalTSLCompiler{}
}

// TSLRenderer renders TSL models by compiling them to WGSL and reusing the
// WebGPU renderer.
type TSLRenderer struct {
	wgpu     *WebGPURenderer
	compiler TSLCompiler
}

// Init implements Renderer.
func (r *TSLRenderer) Init(width, height int, view bool) error {
	if r.wgpu == nil {
		r.wgpu = &WebGPURenderer{}
	}
	return r.wgpu.Init(width, height, view)
}

// Prepare implements Renderer.
func (r *TSLRenderer) Prepare(irmf *IRMF, vec3Str string, planeVertices []float32, projection, camera, model mgl32.Mat4) error {
	if r.wgpu == nil {
		r.wgpu = &WebGPURenderer{}
	}
	compiler := r.compiler
	if compiler == nil {
		compiler = activeTSLCompiler()
	}
	code, err := compiler.Compile(context.Background(), irmf.Shader, len(irmf.Materials))
	if err != nil {
		return err
	}
	// Reuse the WGSL path with the compiled function as the shader body.
	synthesized := *irmf
	synthesized.Language = "wgsl"
	synthesized.Shader = code
	return r.wgpu.Prepare(&synthesized, vec3Str, planeVertices, projection, camera, model)
}

// Render implements Renderer.
func (r *TSLRenderer) Render(sliceDepth float32, materialNum int) (image.Image, error) {
	return r.wgpu.Render(sliceDepth, materialNum)
}

// Close implements Renderer.
func (r *TSLRenderer) Close() {
	if r.wgpu != nil {
		r.wgpu.Close()
	}
}
