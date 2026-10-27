package irmf

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

func tslSource(t *testing.T, body string, materials ...string) []byte {
	t.Helper()
	mats := `"` + strings.Join(materials, `","`) + `"`
	if len(materials) == 0 {
		mats = `"PLA"`
	}
	return []byte(`/*{
  "irmf": "1.0",
  "language": "tsl",
  "tslVersion": "0.180.0",
  "materials": [` + mats + `],
  "min": [-5,-5,-5],
  "max": [5,5,5],
  "units": "mm"
}*/
` + body)
}

func TestNewModelAcceptsTSL(t *testing.T) {
	src := tslSource(t, "const mainModel4 = (xyz) => [float(1)];\n")
	m, err := newModel(src)
	if err != nil {
		t.Fatalf("newModel: %v", err)
	}
	if m.Language != "tsl" {
		t.Errorf("Language = %q, want tsl", m.Language)
	}
	if m.TSLVersion != "0.180.0" {
		t.Errorf("TSLVersion = %q, want 0.180.0", m.TSLVersion)
	}
	if !strings.Contains(m.Shader, "mainModel4") {
		t.Errorf("Shader body not preserved: %q", m.Shader)
	}
}

func TestNewModelRejectsTSLWithoutEntryPoint(t *testing.T) {
	src := tslSource(t, "const sphere = (r, p) => float(1);\n")
	if _, err := newModel(src); err == nil || !strings.Contains(err.Error(), "mainModel4") {
		t.Fatalf("newModel err = %v, want a mainModel4 error", err)
	}
}

func TestNewModelRejectsUnknownLanguage(t *testing.T) {
	src := []byte(`/*{
  "irmf": "1.0",
  "language": "python",
  "materials": ["PLA"],
  "min": [-5,-5,-5],
  "max": [5,5,5],
  "units": "mm"
}*/
x
`)
	if _, err := newModel(src); err == nil || !strings.Contains(err.Error(), "unsupported language") {
		t.Fatalf("newModel err = %v, want unsupported language", err)
	}
}

func TestSlicerSelectsTSLRenderer(t *testing.T) {
	s := Init(false, 10, 10, 10) // renderer selection only; keep the slice count sane
	defer s.Close()
	if err := s.NewModel(tslSource(t, "const mainModel4 = (xyz) => [float(1)];\n")); err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	if _, ok := s.renderer.(*TSLRenderer); !ok {
		t.Fatalf("renderer = %T, want *TSLRenderer", s.renderer)
	}
}

type failingCompiler struct{ err error }

func (c failingCompiler) Compile(context.Context, string, int) (string, error) { return "", c.err }

func TestTSLRendererReportsMissingCompiler(t *testing.T) {
	SetTSLCompiler(failingCompiler{err: ErrTSLCompilerUnavailable})
	defer SetTSLCompiler(nil)

	irmf := &IRMF{Language: "tsl", Materials: []string{"a"}, Shader: "const mainModel4 = (xyz) => [float(1)];"}
	r := &TSLRenderer{}
	err := r.Prepare(irmf, "", nil, mgl32.Ident4(), mgl32.Ident4(), mgl32.Ident4())
	if !errors.Is(err, ErrTSLCompilerUnavailable) {
		t.Fatalf("Prepare err = %v, want ErrTSLCompilerUnavailable", err)
	}
	if !strings.Contains(err.Error(), "IRMF_TSL_COMPILER") {
		t.Errorf("error message should mention IRMF_TSL_COMPILER: %v", err)
	}
}

func TestExternalTSLCompilerInvokesCLI(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-irmf-tsl-compile")
	body := `#!/bin/sh
cat >/dev/null
printf '%s' '{"ok":true,"source":"fn mainModel4(xyz: vec3f) -> vec4f { return vec4f(1.0); }"}'
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	c := &ExternalTSLCompiler{Path: script}
	got, err := c.Compile(context.Background(), "const mainModel4 = (xyz) => [float(1)];", 1)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(got, "fn mainModel4") {
		t.Errorf("Compile returned %q", got)
	}
}

func TestExternalTSLCompilerReportsDiagnostics(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-irmf-tsl-compile")
	body := `#!/bin/sh
cat >/dev/null
printf '%s' '{"ok":false,"diagnostics":[{"line":3,"col":2,"message":"boom"}]}'
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	c := &ExternalTSLCompiler{Path: script}
	_, err := c.Compile(context.Background(), "x", 1)
	if err == nil || !strings.Contains(err.Error(), "line 3:2: boom") {
		t.Fatalf("Compile err = %v, want a diagnostic with line 3:2", err)
	}
}

func TestExternalTSLCompilerUnavailable(t *testing.T) {
	t.Setenv("IRMF_TSL_COMPILER", "")
	c := &ExternalTSLCompiler{Path: filepath.Join(t.TempDir(), "does-not-exist")}
	_, err := c.Compile(context.Background(), "x", 1)
	if err == nil {
		t.Fatal("Compile succeeded, want an error")
	}
}
