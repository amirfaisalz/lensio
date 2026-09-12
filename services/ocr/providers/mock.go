package providers

import (
	"bytes"
	"context"
	"sync"

	"github.com/amirfaisalz/lensio/services/ocr"
)

// Magic markers in image bytes to trigger specific mock behaviors during testing.
var (
	MarkerUnsupportedDoc = []byte("MOCK_UNSUPPORTED_DOC")
	MarkerOCRFailure     = []byte("MOCK_OCR_FAILURE")
	MarkerLowConfidence  = []byte("MOCK_LOW_CONFIDENCE")
)

// MockOCREngine provides deterministic OCR extraction for tests without external network calls.
type MockOCREngine struct {
	mu           sync.RWMutex
	customResult *ocr.OCRResult
	customErr    error
	callCount    int
}

// NewMockEngine creates a new MockOCREngine with default synthetic fixture responses.
func NewMockEngine() *MockOCREngine {
	return &MockOCREngine{}
}

// SetCustomResult overrides the extraction response.
func (m *MockOCREngine) SetCustomResult(res *ocr.OCRResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customResult = res
}

// SetCustomError overrides the engine to return a specified error.
func (m *MockOCREngine) SetCustomError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customErr = err
}

// Reset clears custom overrides.
func (m *MockOCREngine) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customResult = nil
	m.customErr = nil
	m.callCount = 0
}

// GetCallCount returns the total number of times Extract was called.
func (m *MockOCREngine) GetCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount
}

// Extract extracts KTP information from synthetic image bytes.
func (m *MockOCREngine) Extract(ctx context.Context, image []byte) (*ocr.OCRResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	m.mu.Lock()
	m.callCount++
	customRes := m.customResult
	customErr := m.customErr
	m.mu.Unlock()

	if customErr != nil {
		return nil, customErr
	}
	if customRes != nil {
		return customRes, nil
	}

	// Heuristic inspection of byte stream for testing triggers
	if bytes.Contains(image, MarkerOCRFailure) {
		return nil, ocr.ErrOCRFailed
	}
	if bytes.Contains(image, MarkerUnsupportedDoc) {
		return nil, ocr.ErrUnsupportedDocument
	}

	confidence := 0.98
	if bytes.Contains(image, MarkerLowConfidence) {
		confidence = 0.45
	}

	// Deterministic standard synthetic KTP data
	return &ocr.OCRResult{
		DocumentType: "ktp",
		Confidence:   confidence,
		RawText:      "REPUBLIK INDONESIA PROVINSI DKI JAKARTA NIK 3171010101900001 NAMA BUDI SANTOSO TEMPAT/TGL LAHIR JAKARTA 01-01-1990 JENIS KELAMIN LAKI-LAKI ALAMAT JL. MERDEKA NO. 10 RT/RW 001/002 KEL/DESA GAMBIR KECAMATAN GAMBIR AGAMA ISLAM STATUS PERKAWINAN KAWIN PEKERJAAN KARYAWAN SWASTA KEWARGANEGARAAN WNI",
		Data: &ocr.KTPData{
			NIK:              "3171010101900001",
			Nama:             "BUDI SANTOSO",
			TempatLahir:      "JAKARTA",
			TanggalLahir:     "1990-01-01",
			JenisKelamin:     "LAKI-LAKI",
			Alamat:           "JL. MERDEKA NO. 10",
			RTRW:             "001/002",
			Kelurahan:        "GAMBIR",
			Kecamatan:        "GAMBIR",
			Agama:            "ISLAM",
			StatusPerkawinan: "KAWIN",
			Pekerjaan:        "KARYAWAN SWASTA",
			Kewarganegaraan:  "WNI",
		},
	}, nil
}
