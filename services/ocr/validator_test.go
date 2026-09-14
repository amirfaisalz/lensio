package ocr_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/amirfaisalz/lensio/services/ocr"
)

func makeTestPNG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func makeTestJPEG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

func makeFakeWebP(length int) []byte {
	data := make([]byte, length)
	if length < 4 {
		return data
	}
	copy(data[0:4], []byte("RIFF"))
	if length >= 12 {
		binary.LittleEndian.PutUint32(data[4:8], uint32(length-8))
		copy(data[8:12], []byte("WEBP"))
	}
	if length >= 26 {
		copy(data[12:16], []byte("VP8 "))
		data[23], data[24], data[25] = 0x9d, 0x01, 0x2a
	}
	return data
}

func TestDetectMIMEType(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "short data",
			data:     []byte{0x01, 0x02},
			expected: "",
		},
		{
			name:     "jpeg data",
			data:     append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 20)...),
			expected: ocr.MIMEJPEG,
		},
		{
			name:     "png data",
			data:     append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 20)...),
			expected: ocr.MIMEPNG,
		},
		{
			name:     "webp data",
			data:     makeFakeWebP(32),
			expected: ocr.MIMEWebP,
		},
		{
			name:     "unrecognized data",
			data:     []byte("some random plain text longer than 12 bytes"),
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ocr.DetectMIMEType(tc.data)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestValidateImage(t *testing.T) {
	t.Run("empty buffer returns error", func(t *testing.T) {
		_, err := ocr.ValidateImage(nil)
		if !errors.Is(err, ocr.ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument, got %v", err)
		}
	})

	t.Run("oversized image returns error", func(t *testing.T) {
		oversized := make([]byte, ocr.MaxImageSizeBytes+1)
		copy(oversized[:3], []byte{0xFF, 0xD8, 0xFF})
		_, err := ocr.ValidateImage(oversized)
		if !errors.Is(err, ocr.ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument, got %v", err)
		}
	})

	t.Run("unrecognized format returns error", func(t *testing.T) {
		badData := []byte("plain text document not an image")
		_, err := ocr.ValidateImage(badData)
		if !errors.Is(err, ocr.ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument, got %v", err)
		}
	})

	t.Run("corrupt JPEG header returns error", func(t *testing.T) {
		corruptJPEG := append([]byte{0xFF, 0xD8, 0xFF}, []byte("not valid jpeg stream data")...)
		_, err := ocr.ValidateImage(corruptJPEG)
		if !errors.Is(err, ocr.ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument, got %v", err)
		}
	})

	t.Run("image too small returns error", func(t *testing.T) {
		smallPNG := makeTestPNG(50, 50)
		_, err := ocr.ValidateImage(smallPNG)
		if !errors.Is(err, ocr.ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument, got %v", err)
		}
	})

	t.Run("valid PNG returns MIMEPNG", func(t *testing.T) {
		validPNG := makeTestPNG(300, 200)
		mime, err := ocr.ValidateImage(validPNG)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mime != ocr.MIMEPNG {
			t.Fatalf("expected %s, got %s", ocr.MIMEPNG, mime)
		}
	})

	t.Run("valid JPEG returns MIMEJPEG", func(t *testing.T) {
		validJPEG := makeTestJPEG(300, 200)
		mime, err := ocr.ValidateImage(validJPEG)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mime != ocr.MIMEJPEG {
			t.Fatalf("expected %s, got %s", ocr.MIMEJPEG, mime)
		}
	})

	t.Run("truncated WebP returns error", func(t *testing.T) {
		truncatedWebP := makeFakeWebP(20)
		_, err := ocr.ValidateImage(truncatedWebP)
		if !errors.Is(err, ocr.ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument, got %v", err)
		}
	})

	t.Run("valid WebP header returns MIMEWebP", func(t *testing.T) {
		validWebP := makeFakeWebP(40)
		mime, err := ocr.ValidateImage(validWebP)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mime != ocr.MIMEWebP {
			t.Fatalf("expected %s, got %s", ocr.MIMEWebP, mime)
		}
	})

	t.Run("unknown WebP chunk returns error", func(t *testing.T) {
		badChunk := makeFakeWebP(40)
		copy(badChunk[12:16], []byte("XXXX"))
		if _, err := ocr.ValidateImage(badChunk); !errors.Is(err, ocr.ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument, got %v", err)
		}
	})

	t.Run("corrupt WebP VP8 frame returns error", func(t *testing.T) {
		badFrame := makeFakeWebP(40)
		badFrame[23], badFrame[24], badFrame[25] = 0x00, 0x00, 0x00
		if _, err := ocr.ValidateImage(badFrame); !errors.Is(err, ocr.ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument, got %v", err)
		}
	})

	t.Run("WebP RIFF size mismatch returns error", func(t *testing.T) {
		badSize := makeFakeWebP(40)
		binary.LittleEndian.PutUint32(badSize[4:8], uint32(4))
		if _, err := ocr.ValidateImage(badSize); !errors.Is(err, ocr.ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument, got %v", err)
		}
	})
}

func BenchmarkValidateImage(b *testing.B) {
	img := makeTestPNG(400, 250)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ocr.ValidateImage(img)
		if err != nil {
			b.Fatal(err)
		}
	}
}
