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

// GenerateValidSIMImage returns a valid in-memory PNG image meeting dimensions and embedded with MarkerSIMDoc.
func GenerateValidSIMImage() []byte {
	return GenerateCustomImage(400, 250, color.RGBA{R: 240, G: 230, B: 200, A: 255}, providers.MarkerSIMDoc)
}

// GenerateValidSIMJPEG returns a valid in-memory JPEG image with SIM marker.
func GenerateValidSIMJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 400, 250))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 245, G: 235, B: 205, A: 255}}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	buf.Write(providers.MarkerSIMDoc)
	return buf.Bytes()
}

// GenerateSIMLowConfidenceImage returns an image containing the SIM low confidence test marker.
func GenerateSIMLowConfidenceImage() []byte {
	return GenerateCustomImage(300, 200, color.RGBA{R: 230, G: 230, B: 210, A: 255}, providers.MarkerSIMLowConfidence)
}

// GenerateValidPassportImage returns a valid in-memory PNG image meeting dimensions and embedded with MarkerPassportDoc.
func GenerateValidPassportImage() []byte {
	return GenerateCustomImage(400, 280, color.RGBA{R: 210, G: 240, B: 220, A: 255}, providers.MarkerPassportDoc)
}

// GenerateValidPassportJPEG returns a valid in-memory JPEG image with Passport marker.
func GenerateValidPassportJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 400, 280))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 215, G: 245, B: 225, A: 255}}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	buf.Write(providers.MarkerPassportDoc)
	return buf.Bytes()
}

// GeneratePassportLowConfidenceImage returns an image containing the passport low confidence test marker.
func GeneratePassportLowConfidenceImage() []byte {
	return GenerateCustomImage(300, 200, color.RGBA{R: 220, G: 235, B: 220, A: 255}, providers.MarkerPassportLowConfidence)
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
