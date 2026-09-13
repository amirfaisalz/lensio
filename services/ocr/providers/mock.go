package providers

import (
	"bytes"
	"context"
	"sync"

	"github.com/amirfaisalz/lensio/services/ocr"
)

// Magic markers in image bytes to trigger specific mock behaviors during testing.
var (
	MarkerUnsupportedDoc   = []byte("MOCK_UNSUPPORTED_DOC")
	MarkerOCRFailure       = []byte("MOCK_OCR_FAILURE")
	MarkerLowConfidence    = []byte("MOCK_LOW_CONFIDENCE")
	MarkerSIMDoc           = []byte("MOCK_SIM_DOC")
	MarkerSIMLowConfidence = []byte("MOCK_SIM_LOW_CONFIDENCE")
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

	if bytes.Contains(image, MarkerSIMDoc) || bytes.Contains(image, MarkerSIMLowConfidence) {
		confidence := 0.98
		var simData *ocr.SIMData
		if bytes.Contains(image, MarkerSIMLowConfidence) {
			confidence = 0.45
			simData = &ocr.SIMData{
				NomorSIM: "123456789012",
			}
		} else {
			simData = &ocr.SIMData{
				NomorSIM:      "123456789012",
				Golongan:      "A",
				Nama:          "BUDI SANTOSO",
				TempatLahir:   "JAKARTA",
				TanggalLahir:  "1990-01-01",
				GolonganDarah: "O",
				JenisKelamin:  "PRIA",
				Alamat:        "JL. MERDEKA NO. 10",
				Pekerjaan:     "KARYAWAN SWASTA",
				Polda:         "METRO JAYA",
				MasaBerlaku:   "2029-01-01",
			}
		}
		return &ocr.OCRResult{
			DocumentType: "sim",
			Confidence:   confidence,
			RawText:      "KEPOLISIAN NEGARA REPUBLIK INDONESIA SURAT IZIN MENGEMUDI DRIVING LICENSE SIM A No. SIM: 1234-5678-9012 1. NAMA: BUDI SANTOSO 2. TEMPAT/TGL LAHIR: JAKARTA, 01-01-1990 3. GOL. DARAH: O - JENIS KELAMIN: PRIA 4. ALAMAT: JL. MERDEKA NO. 10 5. PEKERJAAN: KARYAWAN SWASTA POLDA: METRO JAYA BERLAKU S/D: 01-01-2029",
			SIMData:      simData,
		}, nil
	}

	confidence := 0.98
	var ktpData *ocr.KTPData
	if bytes.Contains(image, MarkerLowConfidence) {
		confidence = 0.45
		ktpData = &ocr.KTPData{
			NIK: "3171010101900001",
		}
	} else {
		ktpData = &ocr.KTPData{
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
		}
	}

	// Deterministic standard synthetic KTP data
	return &ocr.OCRResult{
		DocumentType: "ktp",
		Confidence:   confidence,
		RawText:      "REPUBLIK INDONESIA PROVINSI DKI JAKARTA NIK 3171010101900001 NAMA BUDI SANTOSO TEMPAT/TGL LAHIR JAKARTA 01-01-1990 JENIS KELAMIN LAKI-LAKI ALAMAT JL. MERDEKA NO. 10 RT/RW 001/002 KEL/DESA GAMBIR KECAMATAN GAMBIR AGAMA ISLAM STATUS PERKAWINAN KAWIN PEKERJAAN KARYAWAN SWASTA KEWARGANEGARAAN WNI",
		Data:         ktpData,
	}, nil
}
