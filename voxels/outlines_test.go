package voxels

import (
	"fmt"
	"reflect"
	"testing"
)

func TestFindEdges(t *testing.T) {
	tests := []struct {
		label *Label
		want  Outline
	}{
		{
			label: &Label{
				xmin: 1,
				ymin: 1,
				xmax: 8,
				ymax: 6,
				pixels: map[string]struct{}{
					"00001,00002": struct{}{},
					"00001,00003": struct{}{},
					"00001,00006": struct{}{},
					"00001,00007": struct{}{},
					"00002,00001": struct{}{},
					"00002,00002": struct{}{},
					"00002,00003": struct{}{},
					"00002,00004": struct{}{},
					"00002,00005": struct{}{},
					"00002,00006": struct{}{},
					"00002,00007": struct{}{},
					"00002,00008": struct{}{},
					"00003,00003": struct{}{},
					"00003,00004": struct{}{},
					"00003,00005": struct{}{},
					"00003,00006": struct{}{},
					"00004,00002": struct{}{},
					"00004,00003": struct{}{},
					"00004,00004": struct{}{},
					"00004,00005": struct{}{},
					"00005,00001": struct{}{},
					"00005,00002": struct{}{},
					"00005,00003": struct{}{},
					"00005,00006": struct{}{},
					"00005,00007": struct{}{},
					"00006,00002": struct{}{},
					"00006,00003": struct{}{}}},
			want: Outline{
				"00001,00002": 0x3,
				"00001,00003": 0x9,
				"00001,00006": 0x3,
				"00001,00007": 0x9,
				"00002,00001": 0x7,
				"00002,00002": 0x4,
				"00002,00004": 0x1,
				"00002,00005": 0x1,
				"00002,00007": 0x4,
				"00002,00008": 0xd,
				"00003,00003": 0x2,
				"00003,00006": 0xc,
				"00004,00002": 0x3,
				"00004,00004": 0x4,
				"00004,00005": 0xc,
				"00005,00001": 0x7,
				"00005,00003": 0x8,
				"00005,00006": 0x7,
				"00005,00007": 0xd,
				"00006,00002": 0x6,
				"00006,00003": 0xc,
			},
		},
		{
			label: &Label{
				xmin: 6,
				ymin: 1,
				xmax: 15,
				ymax: 7,
				pixels: map[string]struct{}{
					"00001,00010": struct{}{},
					"00001,00011": struct{}{},
					"00001,00014": struct{}{},
					"00001,00015": struct{}{},
					"00002,00011": struct{}{},
					"00002,00012": struct{}{},
					"00002,00013": struct{}{},
					"00002,00014": struct{}{},
					"00003,00010": struct{}{},
					"00003,00011": struct{}{},
					"00003,00012": struct{}{},
					"00003,00013": struct{}{},
					"00004,00009": struct{}{},
					"00004,00010": struct{}{},
					"00004,00011": struct{}{},
					"00004,00014": struct{}{},
					"00004,00015": struct{}{},
					"00005,00011": struct{}{},
					"00005,00012": struct{}{},
					"00005,00013": struct{}{},
					"00006,00009": struct{}{},
					"00006,00010": struct{}{},
					"00006,00014": struct{}{},
					"00006,00015": struct{}{},
					"00007,00006": struct{}{},
					"00007,00007": struct{}{},
					"00007,00008": struct{}{},
					"00007,00009": struct{}{},
					"00007,00012": struct{}{},
					"00007,00013": struct{}{},
					"00007,00014": struct{}{},
					"00007,00015": struct{}{}}},
			want: Outline{
				"00001,00010": 0x7,
				"00001,00011": 0x9,
				"00001,00014": 0x3,
				"00001,00015": 0xd,
				"00002,00011": 0x2,
				"00002,00012": 0x1,
				"00002,00013": 0x1,
				"00002,00014": 0xc,
				"00003,00010": 0x3,
				"00003,00012": 0x4,
				"00003,00013": 0xc,
				"00004,00009": 0x7,
				"00004,00010": 0x4,
				"00004,00011": 0x8,
				"00004,00014": 0x7,
				"00004,00015": 0xd,
				"00005,00011": 0x6,
				"00005,00012": 0x5,
				"00005,00013": 0xd,
				"00006,00009": 0x3,
				"00006,00010": 0xd,
				"00006,00014": 0x3,
				"00006,00015": 0x9,
				"00007,00006": 0x7,
				"00007,00007": 0x5,
				"00007,00008": 0x5,
				"00007,00009": 0xc,
				"00007,00012": 0x7,
				"00007,00013": 0x5,
				"00007,00014": 0x4,
				"00007,00015": 0xc,
			},
		},
		{
			label: &Label{
				xmin: 0,
				ymin: 0,
				xmax: 4,
				ymax: 2,
				pixels: map[string]struct{}{
					"00000,00001": struct{}{},
					"00000,00002": struct{}{},
					"00000,00003": struct{}{},
					"00001,00000": struct{}{},
					"00001,00001": struct{}{},
					"00001,00003": struct{}{},
					"00001,00004": struct{}{},
					"00002,00001": struct{}{},
					"00002,00002": struct{}{},
					"00002,00003": struct{}{}}},
			want: Outline{
				"00000,00001": 0x3,
				"00000,00002": 0x5,
				"00000,00003": 0x9,
				"00001,00000": 0x7,
				"00001,00001": 0x8,
				"00001,00003": 0x2,
				"00001,00004": 0xd,
				"00002,00001": 0x6,
				"00002,00002": 0x5,
				"00002,00003": 0xc,
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test #%v", i), func(t *testing.T) {
			got := findEdges(tt.label)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findEdges =\n%#v\nwant:\n%#v", got, tt.want)
			}
		})
	}
}
