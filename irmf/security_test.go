package irmf

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"
)

const glslIncludeSource = `/*{
  "irmf": "1.0",
  "language": "glsl",
  "materials": ["PLA"],
  "min": [-5,-5,-5],
  "max": [5,5,5],
  "units": "mm"
}*/

#include "lygia/math/decimation.glsl"
void mainModel4(out vec4 materials, in vec3 xyz) {
  materials[0] = 1.0;
}
`

func TestNewModelRejectsTSLIncludes(t *testing.T) {
	src := tslSource(t,
		"#include \"github.com/attacker/repo/blob/main/payload.glsl\"\n"+
			"const mainModel4 = (xyz) => [float(1)];\n")
	_, err := newModel(src)
	if err == nil || !strings.Contains(err.Error(), "#include") {
		t.Fatalf("newModel err = %v, want an #include rejection", err)
	}
}

func TestNewModelDisablesRemoteIncludesByDefault(t *testing.T) {
	_, err := newModel([]byte(glslIncludeSource))
	if err == nil || !strings.Contains(err.Error(), "remote includes are disabled") {
		t.Fatalf("newModel err = %v, want a disabled-includes error", err)
	}
}

func TestNewModelCanParseWithoutIncludes(t *testing.T) {
	src := []byte(strings.Replace(glslIncludeSource,
		"#include \"lygia/math/decimation.glsl\"\n", "", 1))
	m, err := newModel(src)
	if err != nil {
		t.Fatalf("newModel: %v", err)
	}
	if m.Language != "glsl" {
		t.Errorf("Language = %q, want glsl", m.Language)
	}
}

func TestHasIncludeDirective(t *testing.T) {
	tests := []struct {
		src  string
		want bool
	}{
		{"void mainModel4() {}\n", false},
		{"  #include \"lygia/x.glsl\"\n", true},
		{"// #include \"lygia/x.glsl\"\n", false},
		{"float s = 1.0; // #include not at start\n", false},
	}
	for _, tt := range tests {
		if got := hasIncludeDirective(tt.src); got != tt.want {
			t.Errorf("hasIncludeDirective(%q) = %v, want %v", tt.src, got, tt.want)
		}
	}
}

func TestValidateBounds(t *testing.T) {
	jsonBlob := `{"min":[-5,-5,-5],"max":[5,5,5]}`
	if _, err := validateBounds(jsonBlob, []float32{-5, -5, -5}, []float32{5, 5, 5}); err != nil {
		t.Errorf("validateBounds(good) = %v", err)
	}
	if _, err := validateBounds(jsonBlob,
		[]float32{float32(math.NaN()), 0, 0}, []float32{5, 5, 5}); err == nil {
		t.Error("validateBounds(NaN) = nil, want an error")
	}
	if _, err := validateBounds(jsonBlob,
		[]float32{0, 0, 0}, []float32{maxModelExtent + 1, 5, 5}); err == nil {
		t.Error("validateBounds(huge extent) = nil, want an error")
	}
}

func TestValidateDimensionsRejectsHugeModels(t *testing.T) {
	s := Init(false, 42, 42, 42) // 42 microns
	defer s.Close()

	good := &IRMF{Min: []float32{-5, -5, -5}, Max: []float32{5, 5, 5}}
	if err := s.validateDimensions(good); err != nil {
		t.Errorf("validateDimensions(good) = %v", err)
	}

	huge := &IRMF{Min: []float32{-100000, -5, -5}, Max: []float32{100000, 5, 5}}
	if err := s.validateDimensions(huge); err == nil {
		t.Error("validateDimensions(huge) = nil, want an error")
	}
}

func TestParseIncludeURLRequiresGitHubCommit(t *testing.T) {
	const sha = "6bb268f05fbe1c68a68ec1e66ce4c5d335bdf021"
	unpinned := `#include "github.com/gmlewis/irmf-examples/blob/master/examples/x.glsl"`
	if got := parseIncludeURL(unpinned); got != "" {
		t.Errorf("parseIncludeURL(unpinned) = %q, want rejection", got)
	}
	if _, err := processIncludes(unpinned + "\n"); err == nil || !strings.Contains(err.Error(), "unpinned") {
		t.Errorf("processIncludes(unpinned) err = %v, want an unpinned error", err)
	}

	pinned := `#include "github.com/gmlewis/irmf-examples/blob/` + sha + `/examples/x.glsl"`
	want := "https://raw.githubusercontent.com/gmlewis/irmf-examples/" + sha + "/examples/x.glsl"
	if got := parseIncludeURL(pinned); got != want {
		t.Errorf("parseIncludeURL(pinned) = %q, want %q", got, want)
	}
}

func TestSafeMaterialName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"AISI 1018 steel", "AISI-1018-steel"},
		{"../../.ssh/authorized_keys", "ssh-authorized_keys"},
		{"PLA", "PLA"},
		{"", "material"},
		{"..", "material"},
		{"a/b", "a-b"},
		{"a\nb", "a-b"},
		{"日本語 PLA", "日本語-PLA"},
		{"AISI 1018 钢", "AISI-1018-钢"},
		{"naïve", "naïve"},
		{"PLA·金属", "PLA·金属"},
	}
	for _, tt := range tests {
		if got := SafeMaterialName(tt.in); got != tt.want {
			t.Errorf("SafeMaterialName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if got := SafeMaterialName(strings.Repeat("x", 200)); len(got) != 64 {
		t.Errorf("SafeMaterialName(long) length = %d, want 64", len(got))
	}
	for _, in := range []string{strings.Repeat("日", 100), strings.Repeat("😀", 100)} {
		got := SafeMaterialName(in)
		if !utf8.ValidString(got) {
			t.Errorf("SafeMaterialName produced invalid UTF-8: %q", got)
		}
		if len(got) > 200 {
			t.Errorf("SafeMaterialName byte length = %d, want <= 200", len(got))
		}
	}
}
