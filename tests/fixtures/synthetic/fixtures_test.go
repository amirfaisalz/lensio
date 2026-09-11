package synthetic_test

import (
	"os"
	"testing"

	"github.com/amirfaisalz/lensio/services/ocr"
	"github.com/amirfaisalz/lensio/tests/fixtures/synthetic"
)

func TestSyntheticFixtures(t *testing.T) {
	pngImg := synthetic.GenerateValidKTPImage()
	if len(pngImg) == 0 {
		t.Fatal("expected non-empty png image")
	}
	mime, err := ocr.ValidateImage(pngImg)
	if err != nil || mime != ocr.MIMEPNG {
		t.Fatalf("expected valid png, got mime=%s err=%v", mime, err)
	}

	jpegImg := synthetic.GenerateValidKTPJPEG()
	if len(jpegImg) == 0 {
		t.Fatal("expected non-empty jpeg image")
	}
	mime, err = ocr.ValidateImage(jpegImg)
	if err != nil || mime != ocr.MIMEJPEG {
		t.Fatalf("expected valid jpeg, got mime=%s err=%v", mime, err)
	}

	unsupported := synthetic.GenerateUnsupportedDocImage()
	if len(unsupported) == 0 {
		t.Fatal("expected non-empty unsupported doc image")
	}

	failure := synthetic.GenerateOCRFailureImage()
	if len(failure) == 0 {
		t.Fatal("expected non-empty failure image")
	}

	lowConf := synthetic.GenerateLowConfidenceImage()
	if len(lowConf) == 0 {
		t.Fatal("expected non-empty low confidence image")
	}

	corrupt := synthetic.GenerateCorruptedImage()
	if _, err := ocr.ValidateImage(corrupt); err == nil {
		t.Fatal("expected error validating corrupted image, got nil")
	}

	oversized := synthetic.GenerateOversizedImage()
	if _, err := ocr.ValidateImage(oversized); err == nil {
		t.Fatal("expected error validating oversized image, got nil")
	}

	// Write static fixtures for frontend / E2E test suites
	_ = os.WriteFile("valid_ktp.jpg", jpegImg, 0644)
	_ = os.WriteFile("valid_ktp.png", pngImg, 0644)
}
