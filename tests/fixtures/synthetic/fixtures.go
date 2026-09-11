package synthetic

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"

	"github.com/amirfaisalz/lensio/services/ocr/providers"
)

// GenerateValidKTPImage returns a valid in-memory PNG image meeting dimensions and format requirements.
func GenerateValidKTPImage() []byte {
	return GenerateCustomImage(400, 250, color.RGBA{R: 200, G: 220, B: 240, A: 255}, nil)
}

// GenerateValidKTPJPEG returns a valid in-memory JPEG image.
func GenerateValidKTPJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 400, 250))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 210, G: 230, B: 250, A: 255}}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return buf.Bytes()
}

// GenerateUnsupportedDocImage returns an image containing the unsupported doc test marker.
func GenerateUnsupportedDocImage() []byte {
	return GenerateCustomImage(300, 200, color.RGBA{R: 255, G: 240, B: 240, A: 255}, providers.MarkerUnsupportedDoc)
}

// GenerateOCRFailureImage returns an image containing the OCR failure test marker.
func GenerateOCRFailureImage() []byte {
	return GenerateCustomImage(300, 200, color.RGBA{R: 250, G: 240, B: 200, A: 255}, providers.MarkerOCRFailure)
}

// GenerateLowConfidenceImage returns an image containing the low confidence test marker.
func GenerateLowConfidenceImage() []byte {
	return GenerateCustomImage(300, 200, color.RGBA{R: 220, G: 220, B: 220, A: 255}, providers.MarkerLowConfidence)
}

// GenerateCustomImage creates an in-memory PNG with specific dimensions, background color, and optional embedded payload bytes.
func GenerateCustomImage(width, height int, bg color.Color, extraPayload []byte) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)

	// Append custom marker payload at end of PNG (safe PNG comment/tail)
	if len(extraPayload) > 0 {
		buf.Write(extraPayload)
	}

	return buf.Bytes()
}

// GenerateCorruptedImage returns invalid bytes simulating corrupted image data.
func GenerateCorruptedImage() []byte {
	return []byte{0xFF, 0xD8, 0xFF, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
}

// GenerateOversizedImage returns byte slice exceeding 5MB limit.
func GenerateOversizedImage() []byte {
	// 5MB + 1KB
	data := make([]byte, 5*1024*1024+1024)
	copy(data[:3], []byte{0xFF, 0xD8, 0xFF}) // fake JPEG header
	return data
}
