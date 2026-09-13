package providers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/amirfaisalz/lensio/services/ocr"
	"github.com/amirfaisalz/lensio/services/ocr/providers"
	"github.com/amirfaisalz/lensio/tests/fixtures/synthetic"
)

func TestMockOCREngine(t *testing.T) {
	ctx := context.Background()
	engine := providers.NewMockEngine()

	t.Run("default synthetic extraction", func(t *testing.T) {
		validImg := synthetic.GenerateValidKTPImage()
		res, err := engine.Extract(ctx, validImg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.DocumentType != "ktp" {
			t.Errorf("expected doc_type ktp, got %s", res.DocumentType)
		}
		if res.Confidence < 0.90 {
			t.Errorf("expected high confidence, got %f", res.Confidence)
		}
		if res.Data == nil || res.Data.NIK != "3171010101900001" {
			t.Errorf("expected NIK 3171010101900001, got %v", res.Data)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := engine.Extract(cancelledCtx, []byte("data"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("marker ocr failure", func(t *testing.T) {
		failImg := synthetic.GenerateOCRFailureImage()
		_, err := engine.Extract(ctx, failImg)
		if !errors.Is(err, ocr.ErrOCRFailed) {
			t.Fatalf("expected ErrOCRFailed, got %v", err)
		}
	})

	t.Run("marker unsupported doc", func(t *testing.T) {
		unsupportedImg := synthetic.GenerateUnsupportedDocImage()
		_, err := engine.Extract(ctx, unsupportedImg)
		if !errors.Is(err, ocr.ErrUnsupportedDocument) {
			t.Fatalf("expected ErrUnsupportedDocument, got %v", err)
		}
	})

	t.Run("marker low confidence", func(t *testing.T) {
		lowConfImg := synthetic.GenerateLowConfidenceImage()
		res, err := engine.Extract(ctx, lowConfImg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Confidence >= 0.50 {
			t.Errorf("expected low confidence < 0.50, got %f", res.Confidence)
		}
	})

	t.Run("custom error and custom result override", func(t *testing.T) {
		customErr := errors.New("custom test error")
		engine.SetCustomError(customErr)
		_, err := engine.Extract(ctx, []byte("dummy"))
		if !errors.Is(err, customErr) {
			t.Fatalf("expected custom error, got %v", err)
		}

		engine.Reset()
		customRes := &ocr.OCRResult{
			DocumentType: "ktp",
			Confidence:   0.99,
		}
		engine.SetCustomResult(customRes)
		res, err := engine.Extract(ctx, []byte("dummy"))
		if err != nil || res.Confidence != 0.99 {
			t.Fatalf("expected custom result, got %v, err=%v", res, err)
		}

		engine.Reset()
		res, err = engine.Extract(ctx, []byte("dummy"))
		if err != nil || res.Data.NIK != "3171010101900001" {
			t.Fatalf("expected reset to default fixture, got %v", res)
		}
	})

	t.Run("call count tracking", func(t *testing.T) {
		eng := providers.NewMockEngine()
		if count := eng.GetCallCount(); count != 0 {
			t.Errorf("expected initial call count 0, got %d", count)
		}
		_, _ = eng.Extract(ctx, []byte("call-1"))
		_, _ = eng.Extract(ctx, []byte("call-2"))
		if count := eng.GetCallCount(); count != 2 {
			t.Errorf("expected call count 2, got %d", count)
		}
		eng.Reset()
		if count := eng.GetCallCount(); count != 0 {
			t.Errorf("expected reset call count 0, got %d", count)
		}
	})

	t.Run("marker SIM doc extraction", func(t *testing.T) {
		res, err := engine.Extract(ctx, providers.MarkerSIMDoc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.DocumentType != "sim" {
			t.Errorf("expected doc_type sim, got %s", res.DocumentType)
		}
		if res.Confidence < 0.90 {
			t.Errorf("expected high confidence, got %f", res.Confidence)
		}
		if res.SIMData == nil || res.SIMData.NomorSIM != "123456789012" {
			t.Errorf("expected SIMData with NomorSIM 123456789012, got %v", res.SIMData)
		}
	})

	t.Run("marker SIM low confidence", func(t *testing.T) {
		res, err := engine.Extract(ctx, providers.MarkerSIMLowConfidence)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.DocumentType != "sim" {
			t.Errorf("expected doc_type sim, got %s", res.DocumentType)
		}
		if res.Confidence >= 0.50 {
			t.Errorf("expected low confidence < 0.50, got %f", res.Confidence)
		}
	})
}

