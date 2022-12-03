package irmf

import "testing"

func TestParseIncludeURL(t *testing.T) {
	tests := []struct {
		name    string
		trimmed string
		want    string
	}{
		{
			name: "empty",
		},
		{
			name:    "bogus",
			trimmed: `#include "bad/include.h"`,
		},
		{
			name:    "lygia normal",
			trimmed: `#include "lygia/math/decimation.glsl"`,
			want:    "https://lygia.xyz/math/decimation.glsl",
		},
		{
			name:    "lygia extra space",
			trimmed: `#include    "lygia/math/decimation.glsl"`,
			want:    "https://lygia.xyz/math/decimation.glsl",
		},
		{
			name:    "lygia accidental copy/paste",
			trimmed: `#include "lygia.xyz/math/decimation.glsl"`,
			want:    "https://lygia.xyz/math/decimation.glsl",
		},
		{
			name:    "github normal",
			trimmed: `#include "github.com/gmlewis/irmf-examples/blob/master/examples/012-bifilar-electromagnet/rotation.glsl"`,
			want:    "https://raw.githubusercontent.com/gmlewis/irmf-examples/master/examples/012-bifilar-electromagnet/rotation.glsl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIncludeURL(tt.trimmed)
			if got != tt.want {
				t.Errorf("parseIncludeURL got %v, want %v", got, tt.want)
			}
		})
	}
}
