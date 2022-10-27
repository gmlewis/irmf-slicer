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
			name:    "normal",
			trimmed: `#include "lygia/math/decimation.glsl"`,
			want:    "https://lygia.xyz/math/decimation.glsl",
		},
		{
			name:    "extra space",
			trimmed: `#include    "lygia/math/decimation.glsl"`,
			want:    "https://lygia.xyz/math/decimation.glsl",
		},
		{
			name:    "accidental copy/paste",
			trimmed: `#include "lygia.xyz/math/decimation.glsl"`,
			want:    "https://lygia.xyz/math/decimation.glsl",
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
