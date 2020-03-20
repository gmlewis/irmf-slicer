package photon

import "image"

// This is based on: github.com/Andoryuuta/photon
// LICENSE: Apache-2.0
// https://github.com/Andoryuuta/photon/blob/master/LICENSE

type photonFile struct {
	PlateX             float32
	PlateY             float32
	PlateZ             float32
	LayerThickness     float32
	NormalExposureTime float32
	BottomExposureTime float32
	OffTime            float32
	BottomLayers       uint32
	ScreenHeight       uint32
	ScreenWidth        uint32
	LightCuringType    uint32 // ProjectionType

	PreviewImage   *image.RGBA
	ThumbnailImage *image.RGBA

	Layers []layer
}

type layer struct {
	RawData         []byte
	AbsoluteHeight  float32
	ExposureTime    float32
	PerLayerOffTime float32
}

type binCompatFileHeader struct {
	Magic1                       uint32 // Always 0x12FD0019
	Magic2                       uint32 // Always 0x01
	PlateX                       float32
	PlateY                       float32
	PlateZ                       float32
	Field_14                     uint32
	Field_18                     uint32
	Field_1C                     uint32
	LayerThickness               float32
	NormalExposureTime           float32
	BottomExposureTime           float32
	OffTime                      float32
	BottomLayers                 uint32
	ScreenHeight                 uint32
	ScreenWidth                  uint32
	PreviewHeaderOffset          uint32
	LayerHeadersOffset           uint32
	TotalLayers                  uint32
	PreviewThumbnailHeaderOffset uint32
	Field_4C                     uint32
	LightCuringType              uint32 // ProjectionType
	Field_54                     uint32
	Field_58                     uint32
	Field_60                     uint32
	Field_5C                     uint32
	Field_64                     uint32
	Field_68                     uint32
}

type binCompatPreviewHeader struct {
	Width             uint32
	Height            uint32
	PreviewDataOffset uint32
	PreviewDataSize   uint32
	Field_10          uint64 // Unused, always 0
	Field_18          uint64 // Unused, always 0
}

type binCompatLayerHeader struct {
	AbsoluteHeight  float32
	ExposureTime    float32
	PerLayerOffTime float32 // This is normally set to the file headers OffTime in all layers.

	// Most significant bit is seek type
	// switch(ImageDataOffset>>31)
	//		case 0: from start of file (Only seen this one actually being used.)
	//		case 1: relative (probably...)
	ImageDataOffset uint32
	ImageDataSize   uint32
	Field_14        uint64 // Unused, always 0
	Field_1C        uint64 // Unused, always 0
}
