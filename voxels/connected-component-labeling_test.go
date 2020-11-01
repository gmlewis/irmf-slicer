package voxels

import (
	"image"
	"image/color"
	"reflect"
	"testing"
)

func TestConnectedComponentLabeling(t *testing.T) {
	testImage := &TestImage{
		data: []byte{
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1, 0,
			0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 1, 1, 1, 1, 0, 0,
			0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0,
			0, 0, 1, 1, 1, 1, 0, 0, 0, 1, 1, 1, 0, 0, 1, 1, 0,
			0, 1, 1, 1, 0, 0, 1, 1, 0, 0, 0, 1, 1, 1, 0, 0, 0,
			0, 0, 1, 1, 0, 0, 0, 0, 0, 1, 1, 0, 0, 0, 1, 1, 0,
			0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 1, 1, 1, 1, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		},
		width:  17,
		height: 9,
	}
	labels := connectedComponentLabeling(testImage)

	want := map[int]*Label{
		1: &Label{
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
		3: &Label{
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
	}

	if !reflect.DeepEqual(labels, want) {
		t.Errorf("labels = %v, want %v", labels, want)
	}
}

// TestImage is an example taken from:
// https://en.wikipedia.org/wiki/Connected-component_labeling
type TestImage struct {
	data   []byte
	width  int
	height int
}

func (t *TestImage) At(u, v int) color.Color {
	index := v*t.width + u
	return color.Gray16{Y: uint16(t.data[index]) * color.White.Y}
}

func (t *TestImage) Bounds() image.Rectangle {
	return image.Rectangle{Min: image.Point{}, Max: image.Point{X: t.width - 1, Y: t.height - 1}}
}

func (t *TestImage) ColorModel() color.Model {
	return nil
}
