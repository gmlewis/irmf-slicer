package voxels

import (
	"fmt"
	"math"
	"testing"
)

func TestCalcNormal(t *testing.T) {
	tests := []struct {
		x2, y2 float32
		wantX  float32
		wantY  float32
	}{
		{
			x2:    100,
			y2:    0,
			wantX: 0,
			wantY: 1,
		},
		{
			x2:    0,
			y2:    100,
			wantX: 1,
			wantY: 6.123234e-17, // close to zero
		},
		{
			x2:    100,
			y2:    100,
			wantX: float32(0.5 * math.Sqrt(2)),
			wantY: float32(0.5 * math.Sqrt(2)),
		},
	}

	const x1, y1 = 0, 0
	for i, tt := range tests {
		t.Run(fmt.Sprintf("test #%v", i), func(t *testing.T) {
			// length := math.Sqrt(float64((tt.x2-x1)*(tt.x2-x1) + (tt.y2-y1)*(tt.y2-y1)))
			angle := math.Atan2(float64(tt.y2-y1), float64(tt.x2-x1))
			n := [3]float32{float32(math.Sin(angle)), float32(math.Cos(angle)), 0}

			if n[0] != tt.wantX || n[1] != tt.wantY {
				t.Errorf("(%v,%v): normal = (%v,%v), want (%v,%v)", tt.x2, tt.y2, n[0], n[1], tt.wantX, tt.wantY)
			}
		})
	}
}
