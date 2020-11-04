package voxels

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCorrectConcavity(t *testing.T) {
	tests := []struct {
		hullPath Path
		fullPath Path
		want     Path
	}{
		{}, // no outline
		{
			hullPath: Path{
				"00001,00001",
				"00001,00005",
				"00005,00005",
				"00005,00004",
				"00002,00001",
				"00001,00001",
			},
			fullPath: Path{
				"00001,00001",
				"00001,00002",
				"00001,00003",
				"00001,00004",
				"00001,00005",
				"00002,00005",
				"00003,00005",
				"00004,00005",
				"00005,00005",
				"00005,00004",
				"00004,00004",
				"00003,00004",
				"00002,00004",
				"00002,00003",
				"00002,00002",
				"00002,00001",
				"00001,00001",
			},
			want: Path{
				"00001,00001",
				"00001,00005",
				"00005,00005",
				"00005,00004",
				"00002,00004",
				"00002,00001",
				"00001,00001",
			},
		},
	}

	const threshold = 2
	for i, tt := range tests {
		t.Run(fmt.Sprintf("test #%v", i), func(t *testing.T) {
			got := correctConcavity(tt.hullPath, tt.fullPath, threshold)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("correctConcavity =\n%#v\nwant:\n%#v", got, tt.want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	cc := &concavityChecker{
		hullPath: Path{
			"00001,00001",
			"00001,00005",
			"00005,00005",
			"00005,00004",
			"00002,00001",
			"00001,00001",
		},
		fullPath: Path{
			"00001,00001",
			"00001,00002",
			"00001,00003",
			"00001,00004",
			"00001,00005",
			"00002,00005",
			"00003,00005",
			"00004,00005",
			"00005,00005",
			"00005,00004",
			"00004,00004",
			"00003,00004",
			"00002,00004",
			"00002,00003",
			"00002,00002",
			"00002,00001",
			"00001,00001",
		},
		finalPath: Path{"00001,00001"},
	}

	tests := []struct {
		inner     int
		outer     int
		want      int
		finalPath Path
	}{
		{
			inner:     1,
			outer:     1,
			want:      5,
			finalPath: Path{"00001,00001", "00001,00005"},
		},
		{
			inner:     5,
			outer:     2,
			want:      9,
			finalPath: Path{"00001,00001", "00001,00005", "00005,00005"},
		},
		{
			inner:     9,
			outer:     3,
			want:      10,
			finalPath: Path{"00001,00001", "00001,00005", "00005,00005", "00005,00004"},
		},
		{
			inner:     10,
			outer:     4,
			want:      16,
			finalPath: Path{"00001,00001", "00001,00005", "00005,00005", "00005,00004", "00002,00004", "00002,00001"},
		},
		{
			inner:     16,
			outer:     5,
			want:      17,
			finalPath: Path{"00001,00001", "00001,00005", "00005,00005", "00005,00004", "00002,00004", "00002,00001", "00001,00001"},
		},
	}

	const threshold = 2
	for i, tt := range tests {
		t.Run(fmt.Sprintf("test #%v", i), func(t *testing.T) {
			got := cc.check(tt.inner, tt.outer, threshold)

			if got != tt.want {
				t.Errorf("check(%v,%v) = %v, want %v", tt.inner, tt.outer, got, tt.want)
			}

			if !reflect.DeepEqual(cc.finalPath, tt.finalPath) {
				t.Errorf("cc.finalPath=\n%#v\nwant:\n%#v", cc.finalPath, tt.finalPath)
			}
		})
	}
}
