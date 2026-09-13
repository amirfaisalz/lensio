package providers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
			if key := r.Header.Get("x-goog-api-key"); key != "test-api-key" {
				http.Error(w, "missing or invalid x-goog-api-key header", http.StatusUnauthorized)
				return
			}
			if r.URL.Query().Get("key") != "" {
				http.Error(w, "api key must not be passed in query parameters", http.StatusBadRequest)
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

	t.Run("successful SIM extraction", func(t *testing.T) {
		mockGeminiResponse := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{
								"text": `{
									"document_type": "sim",
									"confidence": 0.98,
									"raw_text": "SURAT IZIN MENGEMUDI SIM A 1234-5678-9012 BUDI SANTOSO",
									"nomor_sim": "1234-5678-9012",
									"golongan": "A",
									"nama": "BUDI SANTOSO",
									"tempat_lahir": "JAKARTA",
									"tanggal_lahir": "1990-01-01",
									"golongan_darah": "O",
									"jenis_kelamin": "PRIA",
									"alamat": "JL. MERDEKA NO. 10",
									"pekerjaan": "KARYAWAN SWASTA",
									"polda": "METRO JAYA",
									"masa_berlaku": "2029-01-01"
								}`,
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

		res, err := engine.Extract(ctx, []byte("sim-image-bytes"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.DocumentType != "sim" {
			t.Errorf("expected doc_type sim, got %s", res.DocumentType)
		}
		if res.SIMData == nil || res.SIMData.NomorSIM != "123456789012" {
			t.Errorf("expected SIMData with NomorSIM 123456789012, got %v", res.SIMData)
		}
		if res.Confidence < 0.90 {
			t.Errorf("expected high confidence, got %f", res.Confidence)
		}
	})

	t.Run("successful Passport extraction", func(t *testing.T) {
		mockGeminiResponse := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{
								"text": `{
									"document_type": "passport",
									"confidence": 0.98,
									"raw_text": "PASPOR REPUBLIK INDONESIA PASSPORT X1234567 BUDI SANTOSO",
									"passport_number": "X1234567",
									"full_name": "BUDI SANTOSO",
									"nationality": "IDN",
									"place_of_birth": "JAKARTA",
									"date_of_birth": "1990-01-01",
									"gender": "LAKI-LAKI",
									"issue_date": "2020-01-01",
									"expiry_date": "2030-01-01",
									"issuing_office": "KANIM JAKARTA SELATAN",
									"mrz_line1": "P<IDNSANTOSO<<BUDI<<<<<<<<<<<<<<<<<<<<<<<<<<",
									"mrz_line2": "X1234567<7IDN9001011M3001019<<<<<<<<<<<<<<<2"
								}`,
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

		res, err := engine.Extract(ctx, validImg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.DocumentType != "passport" {
			t.Errorf("expected doc_type passport, got %s", res.DocumentType)
		}
		if res.PassportData == nil || res.PassportData.PassportNumber != "X1234567" {
			t.Errorf("expected PassportNumber X1234567, got %v", res.PassportData)
		}
		if res.Confidence < 0.90 {
			t.Errorf("expected high confidence, got %f", res.Confidence)
		}
	})

	t.Run("successful NPWP extraction", func(t *testing.T) {
		mockGeminiResponse := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{
								"text": `{
									"document_type": "npwp",
									"confidence": 0.98,
									"raw_text": "KEMENTERIAN KEUANGAN DIREKTORAT JENDERAL PAJAK NOMOR POKOK WAJIB PAJAK NPWP 09.254.294.3-407.000 NAMA BUDI SANTOSO NIK 3171010101900001 ALAMAT JL. JENDERAL SUDIRMAN KAV. 21 KPP PRATAMA JAKARTA SETIABUDI SATU TERDAFTAR 17-08-2015",
									"npwp": "09.254.294.3-407.000",
									"nama": "BUDI SANTOSO",
									"nik": "3171010101900001",
									"alamat": "JL. JENDERAL SUDIRMAN KAV. 21",
									"kelurahan": "KARET KUNINGAN",
									"kecamatan": "SETIABUDI",
									"kota_kabupaten": "JAKARTA SELATAN",
									"provinsi": "DKI JAKARTA",
									"kpp": "KPP PRATAMA JAKARTA SETIABUDI SATU",
									"tanggal_daftar": "2015-08-17"
								}`,
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

		res, err := engine.Extract(ctx, validImg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.DocumentType != "npwp" {
			t.Errorf("expected doc_type npwp, got %s", res.DocumentType)
		}
		if res.NPWPData == nil || res.NPWPData.NPWP != "092542943407000" {
			t.Errorf("expected NPWP 092542943407000, got %v", res.NPWPData)
		}
		if res.NPWPData.Nama != "BUDI SANTOSO" {
			t.Errorf("expected Nama BUDI SANTOSO, got %s", res.NPWPData.Nama)
		}
		if res.Confidence < 0.90 {
			t.Errorf("expected high confidence, got %f", res.Confidence)
		}
	})

	t.Run("successful KK extraction", func(t *testing.T) {
		mockGeminiResponse := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{
								"text": `{
									"document_type": "kk",
									"confidence": 0.99,
									"raw_text": "REPUBLIK INDONESIA KARTU KELUARGA NO. KK 3171010101200001 KEPALA KELUARGA BUDI SANTOSO",
									"nomor_kk": "3171010101200001",
									"kepala_keluarga": "BUDI SANTOSO",
									"alamat": "JL. SUDIRMAN NO. 12",
									"rt_rw": "001/002",
									"kode_pos": "12190",
									"kelurahan_desa": "SENAYAN",
									"kecamatan": "KEBAYORAN BARU",
									"kabupaten_kota": "JAKARTA SELATAN",
									"provinsi": "DKI JAKARTA",
									"tanggal_dikeluarkan": "2020-01-01",
									"anggota_keluarga": [
										{
											"nama": "BUDI SANTOSO",
											"nik": "3171010101900001",
											"jenis_kelamin": "LAKI-LAKI",
											"tempat_lahir": "JAKARTA",
											"tanggal_lahir": "1990-01-01",
											"agama": "ISLAM",
											"pendidikan": "STRATA I",
											"jenis_pekerjaan": "KARYAWAN SWASTA",
											"status_perkawinan": "KAWIN",
											"status_hubungan": "KEPALA KELUARGA"
										}
									]
								}`,
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

		res, err := engine.Extract(ctx, validImg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.DocumentType != "kk" {
			t.Errorf("expected doc_type kk, got %s", res.DocumentType)
		}
		if res.KKData == nil || res.KKData.NomorKK != "3171010101200001" {
			t.Errorf("expected NomorKK 3171010101200001, got %v", res.KKData)
		}
		if res.KKData.KepalaKeluarga != "BUDI SANTOSO" {
			t.Errorf("expected KepalaKeluarga BUDI SANTOSO, got %s", res.KKData.KepalaKeluarga)
		}
		if len(res.KKData.AnggotaKeluarga) != 1 {
			t.Errorf("expected 1 family member, got %d", len(res.KKData.AnggotaKeluarga))
		}
		if res.Confidence < 0.90 {
			t.Errorf("expected high confidence, got %f", res.Confidence)
		}
	})

	t.Run("successful invoice extraction", func(t *testing.T) {
		mockGeminiResponse := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{
								"text": `{
									"document_type": "invoice",
									"confidence": 0.98,
									"raw_text": "FAKTUR PAJAK INV-2026-0001",
									"invoice_number": "INV-2026-0001",
									"invoice_date": "2026-03-15",
									"due_date": "2026-04-15",
									"seller_name": "PT TECH UTAMA SYNTHETIC",
									"seller_npwp": "092542943407000",
									"buyer_name": "PT MAJU MUNDUR SYNTHETIC",
									"buyer_npwp": "092542943407000",
									"currency": "IDR",
									"subtotal": 10000000,
									"dpp": 10000000,
									"ppn": 1100000,
									"grand_total": 11100000,
									"line_items": [
										{
											"description": "Cloud Hosting",
											"quantity": 1,
											"unit_price": 10000000,
											"total_price": 10000000
										}
									]
								}`,
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

		res, err := engine.Extract(ctx, validImg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.DocumentType != "invoice" {
			t.Errorf("expected doc_type invoice, got %s", res.DocumentType)
		}
		if res.InvoiceData == nil || res.InvoiceData.InvoiceNumber != "INV-2026-0001" {
			t.Errorf("expected InvoiceData with InvoiceNumber INV-2026-0001, got %v", res.InvoiceData)
		}
		if res.InvoiceData.GrandTotal != 11100000 {
			t.Errorf("expected GrandTotal 11100000, got %.2f", res.InvoiceData.GrandTotal)
		}
		if len(res.InvoiceData.LineItems) != 1 {
			t.Errorf("expected 1 line item, got %d", len(res.InvoiceData.LineItems))
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

	t.Run("marshal error", func(t *testing.T) {
		reset := providers.SetJSONMarshalForTest(func(v any) ([]byte, error) {
			return nil, errors.New("forced marshal fail")
		})
		defer reset()

		engine := providers.NewGeminiEngine("test-api-key", "gemini-2.0-flash")
		_, err := engine.Extract(ctx, validImg)
		if !errors.Is(err, ocr.ErrOCRFailed) {
			t.Fatalf("expected ErrOCRFailed, got %v", err)
		}
	})

	t.Run("invalid base url creating request error", func(t *testing.T) {
		engine := providers.NewGeminiEngine("test-api-key", "gemini-2.0-flash",
			providers.WithBaseURL("::://invalid-url"),
		)
		_, err := engine.Extract(ctx, validImg)
		if !errors.Is(err, ocr.ErrOCRFailed) {
			t.Fatalf("expected ErrOCRFailed, got %v", err)
		}
	})

	t.Run("response body read error", func(t *testing.T) {
		client := &http.Client{
			Transport: &mockErrorTransport{},
		}
		engine := providers.NewGeminiEngine("test-api-key", "gemini-2.0-flash",
			providers.WithHTTPClient(client),
		)
		_, err := engine.Extract(ctx, validImg)
		if !errors.Is(err, ocr.ErrOCRFailed) {
			t.Fatalf("expected ErrOCRFailed, got %v", err)
		}
	})

	t.Run("malformed json response body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not valid json"))
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

	t.Run("candidate with empty text parts", func(t *testing.T) {
		mockEmptyText := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{"text": ""},
						},
					},
				},
			},
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockEmptyText)
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

	t.Run("with custom timeout option", func(t *testing.T) {
		engine := providers.NewGeminiEngine("test-api-key", "",
			providers.WithTimeout(10*time.Second),
		)
		if engine == nil {
			t.Fatal("expected engine instance")
		}
	})
}

type mockErrorTransport struct{}

func (m *mockErrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(&errReader{}),
		Header:     make(http.Header),
	}, nil
}

type errReader struct{}

func (e *errReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}
