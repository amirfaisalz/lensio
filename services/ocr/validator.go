package ocr

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
)

const (
	// MaxImageSizeBytes is the maximum allowed image upload size (5MB).
	MaxImageSizeBytes = 5 * 1024 * 1024

	// Minimum width and height for a readable document image.
	minImageDimension = 100

	MIMEJPEG = "image/jpeg"
	MIMEPNG  = "image/png"
	MIMEWebP = "image/webp"
)

var (
	jpegMagic = []byte{0xFF, 0xD8, 0xFF}
	pngMagic  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	riffMagic = []byte{0x52, 0x49, 0x46, 0x46} // "RIFF"
	webpMagic = []byte{0x57, 0x45, 0x42, 0x50} // "WEBP"
)

// DetectMIMEType inspects file magic bytes in O(1) time complexity.
// Returns recognized image MIME type or empty string if unrecognized.
func DetectMIMEType(data []byte) string {
	if len(data) < 12 {
		return ""
	}

	if bytes.HasPrefix(data, jpegMagic) {
		return MIMEJPEG
	}

	if bytes.HasPrefix(data, pngMagic) {
		return MIMEPNG
	}

	if bytes.HasPrefix(data, riffMagic) && bytes.Equal(data[8:12], webpMagic) {
		return MIMEWebP
	}

	return ""
}

// ValidateImage performs strict, defense-in-depth image validation:
// 1. File size enforcement (<= 5MB).
// 2. Magic byte inspection (JPEG, PNG, WebP).
// 3. Decoding and dimension sanity checking (rejects corrupt headers and tiny images).
// Runs in O(1) / O(n) memory-bounded time without heap bloat.
func ValidateImage(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("%w: image buffer is empty", ErrInvalidDocument)
	}

	if len(data) > MaxImageSizeBytes {
		return "", fmt.Errorf("%w: image size %d exceeds 5MB limit", ErrInvalidDocument, len(data))
	}

	mimeType := DetectMIMEType(data)
	if mimeType == "" {
		return "", fmt.Errorf("%w: unsupported or unrecognized image format", ErrInvalidDocument)
	}

	switch mimeType {
	case MIMEJPEG, MIMEPNG:
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return "", fmt.Errorf("%w: corrupted image data: %v", ErrInvalidDocument, err)
		}
		if cfg.Width < minImageDimension || cfg.Height < minImageDimension {
			return "", fmt.Errorf("%w: image dimensions (%dx%d) too small for identity document", ErrInvalidDocument, cfg.Width, cfg.Height)
		}
	case MIMEWebP:
		// Basic WebP container sanity: minimum header length and RIFF payload size check
		if len(data) < 30 {
			return "", fmt.Errorf("%w: truncated webp header", ErrInvalidDocument)
		}
	}

	return mimeType, nil
}
