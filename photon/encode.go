package photon

import (
	"encoding/binary"
	"image"
	"image/color"
	"log"
	"math"
)

const (
	// Default values from ChiTuBox
	previewWidth  = 0x190
	previewHeight = 0x12c

	screenWidth  = 0xa00
	screenHeight = 0x5a0

	thumbnailWidth  = 0xc8
	thumbnailHeight = 0x7d
)

// This is based on: github.com/Andoryuuta/photon
// LICENSE: Apache-2.0
// https://github.com/Andoryuuta/photon/blob/master/LICENSE

func (d *dlp) writeHeader(img image.Image) error {
	previewData := encodePreview(previewWidth, previewHeight, img.(*image.RGBA))
	thumbnailData := encodePreview(thumbnailWidth, thumbnailHeight, img.(*image.RGBA))

	pos := 0
	pos += binary.Size(binCompatFileHeader{})

	// Preview offsets
	previewHeaderOffset := pos
	pos += binary.Size(binCompatPreviewHeader{})
	previewDataOffset := pos
	pos += len(previewData)

	// Thumbnail offsets
	thumbnailHeaderOffset := pos
	pos += binary.Size(binCompatPreviewHeader{})
	thumbnailDataOffset := pos
	pos += len(thumbnailData)

	// Layer headers offsets
	var layerHeaderOffsets []int
	for i := 0; i < d.numSlices; i++ {
		layerHeaderOffsets = append(layerHeaderOffsets, pos)
		pos += binary.Size(binCompatLayerHeader{})
	}

	d.layerHeaderOffset0 = int64(layerHeaderOffsets[0])

	layer0 := encodeLayerImageData(img.(*image.RGBA))
	log.Printf("layer 0 is %v bytes", len(layer0))

	// Layer data offsets
	var layerDataOffsets []int
	for i := 0; i < d.numSlices; i++ {
		layerDataOffsets = append(layerDataOffsets, pos)
		if i == 0 {
			pos += len(layer0)
		} else {
			pos++ // These will be overwritten once the layer sizes are known.
		}
	}

	// Start forming and writing the file from here
	header := binCompatFileHeader{
		Magic1:                       0x12FD0019,
		Magic2:                       0x01,
		PlateX:                       68.04,  // default
		PlateY:                       120.96, // default
		PlateZ:                       150.0,  // default
		LayerThickness:               d.zRes / 1000.0,
		NormalExposureTime:           6,  // default
		BottomExposureTime:           50, // default
		OffTime:                      0,  // default
		BottomLayers:                 8,  // default
		ScreenHeight:                 screenHeight,
		ScreenWidth:                  screenWidth,
		PreviewHeaderOffset:          uint32(previewHeaderOffset),
		LayerHeadersOffset:           uint32(layerHeaderOffsets[0]),
		TotalLayers:                  uint32(d.numSlices),
		PreviewThumbnailHeaderOffset: uint32(thumbnailHeaderOffset),
		LightCuringType:              1, // default
	}

	previewHeader := binCompatPreviewHeader{
		Width:/*835, // */ uint32(img.Bounds().Max.X),
		Height:/*321, //*/ uint32(img.Bounds().Max.Y),
		PreviewDataOffset: uint32(previewDataOffset),
		PreviewDataSize:   uint32(len(previewData)),
	}

	thumbnailHeader := binCompatPreviewHeader{
		Width:/*199, //*/ uint32(img.Bounds().Max.X),
		Height:/*72,  //*/ uint32(img.Bounds().Max.Y),
		PreviewDataOffset: uint32(thumbnailDataOffset),
		PreviewDataSize:   uint32(len(thumbnailData)),
	}

	for i := 0; i < d.numSlices; i++ {
		expTime := header.NormalExposureTime
		if i < int(header.BottomLayers) {
			expTime = header.BottomExposureTime
		}
		imageDataSize := uint32(i)
		if i == 0 {
			imageDataSize = uint32(len(layer0))
		}
		d.layerHeaders = append(d.layerHeaders, binCompatLayerHeader{
			AbsoluteHeight:  float32(i) * d.zRes / 1000.0,
			ExposureTime:    expTime,
			PerLayerOffTime: 0,                           // default
			ImageDataOffset: uint32(layerDataOffsets[i]), // will be overwritten later.
			ImageDataSize:   imageDataSize,               // will be overwritten later.
		})
	}

	if err := binary.Write(d.w, binary.LittleEndian, header); err != nil {
		return err
	}

	if err := binary.Write(d.w, binary.LittleEndian, previewHeader); err != nil {
		return err
	}

	if err := binary.Write(d.w, binary.LittleEndian, previewData); err != nil {
		return err
	}

	if err := binary.Write(d.w, binary.LittleEndian, thumbnailHeader); err != nil {
		return err
	}

	if err := binary.Write(d.w, binary.LittleEndian, thumbnailData); err != nil {
		return err
	}

	if err := binary.Write(d.w, binary.LittleEndian, d.layerHeaders); err != nil {
		return err
	}

	if err := binary.Write(d.w, binary.LittleEndian, layer0); err != nil {
		return err
	}

	return nil
}

func (d *dlp) writeSlice(sliceNum int, img image.Image) error {
	layer := encodeLayerImageData(img.(*image.RGBA))
	layerSize := uint32(len(layer))
	log.Printf("layer %v is %v bytes", sliceNum, layerSize)

	d.layerHeaders[sliceNum].ImageDataOffset =
		d.layerHeaders[sliceNum-1].ImageDataOffset +
			d.layerHeaders[sliceNum-1].ImageDataSize
	d.layerHeaders[sliceNum].ImageDataSize = layerSize

	if err := binary.Write(d.w, binary.LittleEndian, layer); err != nil {
		return err
	}

	return nil
}

func encodeLayerImageData(img *image.RGBA) []byte {
	const FLAG_SET_PIXELS = 0x80
	var output []byte

	// center the original image in the resin basin.
	origWidth := img.Bounds().Max.X
	origHeight := img.Bounds().Max.Y
	xOffset, yOffset := 0, 0
	if origWidth < screenWidth {
		xOffset = (screenWidth - origWidth) >> 1
	}
	if origHeight < screenHeight {
		yOffset = (screenHeight - origHeight) >> 1
	}

	var unsetCount uint8 = 0
	var setCount uint8 = 0

	maxPixelIndex := screenWidth * screenHeight
	for pixelIndex := 0; pixelIndex < maxPixelIndex; pixelIndex++ {

		y := pixelIndex % screenHeight
		x := pixelIndex / screenHeight

		color := img.At(x-xOffset, y-yOffset)
		if r, _, _, _ := color.RGBA(); r == 0 {
			if setCount != 0 {
				// Previous pixels were set, this was not.
				output = append(output, setCount|FLAG_SET_PIXELS)
				setCount = 0
			}

			unsetCount++
			if unsetCount >= 0x7f-2 { // why -2?
				output = append(output, unsetCount)
				unsetCount = 0
			}
		} else {
			if unsetCount != 0 {
				// Previous pixels were unset, this was not.
				output = append(output, unsetCount)
				unsetCount = 0
			}

			setCount++
			if setCount >= 0x7f-2 { // why -2?
				output = append(output, setCount|FLAG_SET_PIXELS)
				setCount = 0
			}
		}
	}

	// Set any leftover data
	if setCount != 0 {
		// Previous pixels were set, this was not.
		output = append(output, setCount|FLAG_SET_PIXELS)
		setCount = 0
	}

	if unsetCount != 0 {
		// Previous pixels were unset, this was not.
		output = append(output, unsetCount)
		unsetCount = 0
	}

	return output
}

func changeRange(fromMin uint32, fromMax uint32, toMin uint32, toMax uint32, number uint32) uint32 {
	return uint32(math.Round(float64(number-fromMin)*float64(toMax-toMin)/float64(fromMax-fromMin) + float64(toMin)))
}

func combineRGB5515(r uint8, g uint8, b uint8, isFill bool) uint16 {
	// Scale colors from the range of 0-255 to 0-31
	rBits := uint16(changeRange(0, 255, 0, 31, uint32(r)))
	gBits := uint16(changeRange(0, 255, 0, 31, uint32(g)))
	bBits := uint16(changeRange(0, 255, 0, 31, uint32(b)))

	fillBit := uint16(0)
	if isFill {
		fillBit = 1
	}

	var x uint16
	x |= ((rBits & 0x1F) << 0)
	x |= ((fillBit & 0x1) << 5)
	x |= ((gBits & 0x1F) << 6)
	x |= ((bBits & 0x1F) << 11)

	return x
}

func encodePreview(imageWidth, imageHeight int, img *image.RGBA) []uint8 {
	var output []uint8

	origWidth := img.Bounds().Max.X
	origHeight := img.Bounds().Max.Y
	xScale := float32(origWidth) / float32(imageWidth)
	yScale := float32(origHeight) / float32(imageHeight)

	maxDim := imageWidth
	maxPixelIndex := imageHeight * imageWidth

	pixelAt := func(pi int) color.RGBA {
		x := pi % maxDim
		y := pi / maxDim
		newX := int(float32(x) * xScale)
		newY := int(float32(y) * yScale)
		return img.At(newX, newY).(color.RGBA)
	}

	for pixelIndex := 0; pixelIndex <= maxPixelIndex; pixelIndex++ {
		p := pixelAt(pixelIndex)

		if p != pixelAt(pixelIndex+1) || p != pixelAt(pixelIndex+2) || pixelIndex+2 >= maxPixelIndex {
			v := combineRGB5515(p.R, p.G, p.B, false)
			output = append(output, byte((v)&0xFF))
			output = append(output, byte((v>>8)&0xFF))
		} else {

			// Count skips
			var skipCount uint16 = 3
			for ; skipCount < 0xFFF && p == pixelAt(pixelIndex+int(skipCount)); skipCount++ {
			}

			v := combineRGB5515(p.R, p.G, p.B, true) | 0x20
			output = append(output, byte((v)&0xFF))
			output = append(output, byte((v>>8)&0xFF))

			v = skipCount - 1 | 0x3000
			output = append(output, byte((v)&0xFF))
			output = append(output, byte((v>>8)&0xFF))

			pixelIndex += int(skipCount - 1)
		}
	}

	return output
}
