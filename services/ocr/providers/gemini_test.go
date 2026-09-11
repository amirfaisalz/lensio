package providers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amirfaisalz/lensio/services/ocr"
	"github.com/amirfaisalz/lensio/services/ocr/providers"
	"github.com/amirfaisalz/lensio/tests/fixtures/synthetic"
)

func TestGeminiOCREngine(t *testing.T) {
	ctx := context.Background()
	validImg := synthetic.GenerateValidKTPImage()

	t.Run("missing api key returns error", func(t *testing.T) {
		engine := providers.NewGeminiEngine("", "gemini-2.0-flash")
		_, err := engine.Extract(ctx, validImg)
		if !errors.Is(err, ocr.ErrOCRFailed) {
			t.Fatalf("expected ErrOCRFailed, got %v", err)
		}
	})

	t.Run("successful extraction", func(t *testing.T) {
		mockGeminiResponse := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{
								"text": `{
									"document_type": "ktp",
									"confidence": 0.98,
									"raw_text": "REPUBLIK INDONESIA NIK 3171010101900001 BUDI SANTOSO",
									"nik": "3171010101900001",
									"nama": "BUDI SANTOSO",
									"tempat_lahir": "JAKARTA",
									"tanggal_lahir": "1990-01-01",
									"jenis_kelamin": "LAKI-LAKI",
									"alamat": "JL. MERDEKA NO. 10",
									"rt_rw": "001/002",
									"kelurahan": "GAMBIR",
									"kecamatan": "GAMBIR",
									"agama": "ISLAM",
									"status_perkawinan": "KAWIN",
									"pekerjaan": "KARYAWAN SWASTA",
									"kewarganegaraan": "WNI"
								}`,
							},
						},
					},
				},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "bad method", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockGeminiResponse)
		}))
		defer server.Close()

		engine := providers.NewGeminiEngine("test-api-key", "",
			providers.WithBaseURL(server.URL),
			providers.WithHTTPClient(server.Client()),
		)

		res, err := engine.Extract(ctx, []byte("raw-non-magic-bytes"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.DocumentType != "ktp" {
			t.Errorf("expected doc_type ktp, got %s", res.DocumentType)
		}
		if res.Data.NIK != "3171010101900001" {
			t.Errorf("expected NIK 3171010101900001, got %s", res.Data.NIK)
		}
		if res.Confidence < 0.90 {
			t.Errorf("expected high confidence, got %f", res.Confidence)
		}
	})

	t.Run("unsupported document returns ErrUnsupportedDocument", func(t *testing.T) {
		mockGeminiResponse := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{
								"text": `{"document_type": "unsupported"}`,
							},
						},
					},
				},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockGeminiResponse)
		}))
		defer server.Close()

		engine := providers.NewGeminiEngine("test-api-key", "gemini-2.0-flash",
			providers.WithBaseURL(server.URL),
			providers.WithHTTPClient(server.Client()),
		)

		_, err := engine.Extract(ctx, validImg)
		if !errors.Is(err, ocr.ErrUnsupportedDocument) {
			t.Fatalf("expected ErrUnsupportedDocument, got %v", err)
		}
	})

	t.Run("upstream http error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}))
		defer server.Close()

		engine := providers.NewGeminiEngine("test-api-key", "gemini-2.0-flash",
			providers.WithBaseURL(server.URL),
			providers.WithHTTPClient(server.Client()),
		)

		_, err := engine.Extract(ctx, validImg)
		if !errors.Is(err, ocr.ErrOCRFailed) {
			t.Fatalf("expected ErrOCRFailed, got %v", err)
		}
	})

	t.Run("upstream gemini error in json body", func(t *testing.T) {
		mockGeminiErr := map[string]any{
			"error": map[string]any{
				"code":    400,
				"message": "API key not valid",
				"status":  "INVALID_ARGUMENT",
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockGeminiErr)
		}))
		defer server.Close()

		engine := providers.NewGeminiEngine("test-api-key", "gemini-2.0-flash",
			providers.WithBaseURL(server.URL),
			providers.WithHTTPClient(server.Client()),
		)

		_, err := engine.Extract(ctx, validImg)
		if !errors.Is(err, ocr.ErrOCRFailed) {
			t.Fatalf("expected ErrOCRFailed, got %v", err)
		}
	})

	t.Run("empty candidates returned", func(t *testing.T) {
		mockEmpty := map[string]any{
			"candidates": []any{},
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(mockEmpty)
		}))
		defer server.Close()

		engine := providers.NewGeminiEngine("test-api-key", "gemini-2.0-flash",
			providers.WithBaseURL(server.URL),
			providers.WithHTTPClient(server.Client()),
		)

		_, err := engine.Extract(ctx, validImg)
		if !errors.Is(err, ocr.ErrOCRFailed) {
			t.Fatalf("expected ErrOCRFailed, got %v", err)
		}
	})

	t.Run("invalid json in model candidate text", func(t *testing.T) {
		mockBadJSON := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{
								"text": "not-valid-json-string",
							},
						},
					},
				},
			},
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(mockBadJSON)
		}))
		defer server.Close()

		engine := providers.NewGeminiEngine("test-api-key", "gemini-2.0-flash",
			providers.WithBaseURL(server.URL),
			providers.WithHTTPClient(server.Client()),
		)

		_, err := engine.Extract(ctx, validImg)
		if !errors.Is(err, ocr.ErrOCRFailed) {
			t.Fatalf("expected ErrOCRFailed, got %v", err)
		}
	})

	t.Run("network connection failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		server.Close() // Close immediately to trigger connection failure

		engine := providers.NewGeminiEngine("test-api-key", "gemini-2.0-flash",
			providers.WithBaseURL(server.URL),
			providers.WithHTTPClient(server.Client()),
		)

		_, err := engine.Extract(ctx, validImg)
		if !errors.Is(err, ocr.ErrOCRFailed) {
			t.Fatalf("expected ErrOCRFailed, got %v", err)
		}
	})
}
