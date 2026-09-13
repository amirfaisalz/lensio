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

	simPng := synthetic.GenerateValidSIMImage()
	if len(simPng) == 0 {
		t.Fatal("expected non-empty sim png image")
	}
	mime, err = ocr.ValidateImage(simPng)
	if err != nil || mime != ocr.MIMEPNG {
		t.Fatalf("expected valid sim png, got mime=%s err=%v", mime, err)
	}

	simJpeg := synthetic.GenerateValidSIMJPEG()
	if len(simJpeg) == 0 {
		t.Fatal("expected non-empty sim jpeg image")
	}
	mime, err = ocr.ValidateImage(simJpeg)
	if err != nil || mime != ocr.MIMEJPEG {
		t.Fatalf("expected valid sim jpeg, got mime=%s err=%v", mime, err)
	}

	simLowConf := synthetic.GenerateSIMLowConfidenceImage()
	if len(simLowConf) == 0 {
		t.Fatal("expected non-empty sim low confidence image")
	}

	passPng := synthetic.GenerateValidPassportImage()
	if len(passPng) == 0 {
		t.Fatal("expected non-empty passport png image")
	}
	mime, err = ocr.ValidateImage(passPng)
	if err != nil || mime != ocr.MIMEPNG {
		t.Fatalf("expected valid passport png, got mime=%s err=%v", mime, err)
	}

	passJpeg := synthetic.GenerateValidPassportJPEG()
	if len(passJpeg) == 0 {
		t.Fatal("expected non-empty passport jpeg image")
	}
	mime, err = ocr.ValidateImage(passJpeg)
	if err != nil || mime != ocr.MIMEJPEG {
		t.Fatalf("expected valid passport jpeg, got mime=%s err=%v", mime, err)
	}

	passLowConf := synthetic.GeneratePassportLowConfidenceImage()
	if len(passLowConf) == 0 {
		t.Fatal("expected non-empty passport low confidence image")
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

	// Write static fixtures for frontend / E2E test suites only if they don't already exist
	if _, err := os.Stat("valid_ktp.jpg"); os.IsNotExist(err) {
		_ = os.WriteFile("valid_ktp.jpg", jpegImg, 0644)
	}
	if _, err := os.Stat("valid_ktp.png"); os.IsNotExist(err) {
		_ = os.WriteFile("valid_ktp.png", pngImg, 0644)
	}
}
