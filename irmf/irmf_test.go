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
			name:    "github pinned commit",
			trimmed: `#include "github.com/gmlewis/irmf-examples/blob/6bb268f05fbe1c68a68ec1e66ce4c5d335bdf021/examples/012-bifilar-electromagnet/rotation.glsl"`,
			want:    "https://raw.githubusercontent.com/gmlewis/irmf-examples/6bb268f05fbe1c68a68ec1e66ce4c5d335bdf021/examples/012-bifilar-electromagnet/rotation.glsl",
		},
		{
			name:    "github branch is rejected",
			trimmed: `#include "github.com/gmlewis/irmf-examples/blob/master/examples/012-bifilar-electromagnet/rotation.glsl"`,
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
