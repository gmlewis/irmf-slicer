package volumetoirmf

import (
	"fmt"
	"strings"
)

// PrimitiveType represents the type of geometric primitive.
type PrimitiveType int

const (
	Sphere PrimitiveType = iota
	Cube
)

// Operation represents a boolean operation.
type Operation int

const (
	Union Operation = iota
	Difference
)

// Primitive represents a 3D geometric primitive in IRMF.
type Primitive struct {
	Type      PrimitiveType
	Center    [3]float32
	Size      float32 // Radius for sphere, half-extent for cube
	Operation Operation
}

// ToWGSL returns the WGSL code for this primitive.
func (p Primitive) ToWGSL() string {
	switch p.Type {
	case Sphere:
		return fmt.Sprintf("sdSphere(p - vec3f(%f, %f, %f), %f)", p.Center[0], p.Center[1], p.Center[2], p.Size)
	case Cube:
		return fmt.Sprintf("sdBox(p - vec3f(%f, %f, %f), vec3f(%f))", p.Center[0], p.Center[1], p.Center[2], p.Size)
	default:
		return "0.0"
	}
}

// Model represents a collection of primitives.
type Model struct {
	Primitives []Primitive
}

// ToWGSL generates the complete WGSL shader source for the model.
func (m *Model) ToWGSL() string {
	var sb strings.Builder

	sb.WriteString("fn sdSphere(p: vec3f, r: f32) -> f32 {\n")
	sb.WriteString("    return length(p) - r;\n")
	sb.WriteString("}\n\n")

	sb.WriteString("fn sdBox(p: vec3f, b: vec3f) -> f32 {\n")
	sb.WriteString("    let q = abs(p) - b;\n")
	sb.WriteString("    return length(max(q, vec3f(0.0))) + min(max(q.x, max(q.y, q.z)), 0.0);\n")
	sb.WriteString("}\n\n")

	sb.WriteString("fn mainModel4(p: vec3f) -> vec4f {\n")
	sb.WriteString("    var d = 1e10;\n")

	// We use SDF-style composition for simplicity, then convert to 0/1 for IRMF.
	// Actually, IRMF can just use the SDF value <= 0 check.

	for i, p := range m.Primitives {
		expr := p.ToWGSL()
		if i == 0 {
			sb.WriteString(fmt.Sprintf("    d = %s;\n", expr))
		} else {
			if p.Operation == Union {
				sb.WriteString(fmt.Sprintf("    d = min(d, %s);\n", expr))
			} else {
				sb.WriteString(fmt.Sprintf("    d = max(d, -(%s));\n", expr))
			}
		}
	}

	sb.WriteString("    let val = select(0.0, 1.0, d <= 0.0);\n")
	sb.WriteString("    return vec4f(val, 0.0, 0.0, 0.0);\n")
	sb.WriteString("}\n")

	return sb.String()
}
