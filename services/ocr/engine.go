package ocr

import (
	"context"
	"errors"
)

var (
	// ErrInvalidDocument indicates corrupted or unreadable image data.
	ErrInvalidDocument = errors.New("invalid or corrupted image document")

	// ErrUnsupportedDocument indicates the image is not an Indonesian KTP.
	ErrUnsupportedDocument = errors.New("uploaded document was not identified as an Indonesian KTP")

	// ErrOCRFailed indicates upstream OCR engine processing error or timeout.
	ErrOCRFailed = errors.New("upstream ocr engine failure")

	// ErrLowConfidence indicates extraction confidence score fell below acceptable threshold.
	ErrLowConfidence = errors.New("ocr extraction confidence below threshold")

	// ErrCircuitOpen indicates the circuit breaker is open and fast-failing incoming requests.
	ErrCircuitOpen = errors.New("ocr circuit breaker is open: upstream service unavailable")
)

// KTPData represents structured fields extracted from an Indonesian KTP.
type KTPData struct {
	NIK              string `json:"nik"`
	Nama             string `json:"nama"`
	TempatLahir      string `json:"tempat_lahir"`
	TanggalLahir     string `json:"tanggal_lahir"`     // YYYY-MM-DD
	JenisKelamin     string `json:"jenis_kelamin"`     // LAKI-LAKI | PEREMPUAN
	Alamat           string `json:"alamat"`
	RTRW             string `json:"rt_rw"`             // e.g. 001/002
	Kelurahan        string `json:"kelurahan"`
	Kecamatan        string `json:"kecamatan"`
	Agama            string `json:"agama"`             // ISLAM | KRISTEN | KATOLIK | HINDU | BUDDHA | KHONGHUCU
	StatusPerkawinan string `json:"status_perkawinan"` // BELUM KAWIN | KAWIN | CERAI HIDUP | CERAI MATI
	Pekerjaan        string `json:"pekerjaan"`
	Kewarganegaraan  string `json:"kewarganegaraan"`   // WNI | WNA
}

// OCRResult represents the complete structured result of an OCR extraction.
type OCRResult struct {
	DocumentType string   `json:"document_type"`
	Confidence   float64  `json:"confidence"`
	RawText      string   `json:"raw_text,omitempty"`
	Data         *KTPData `json:"data,omitempty"`
}

// OCREngine defines the pluggable document extraction contract.
type OCREngine interface {
	Extract(ctx context.Context, image []byte) (*OCRResult, error)
}

// WithCircuitBreaker wraps an existing OCREngine with circuit breaking capabilities.
func WithCircuitBreaker(engine OCREngine, cfg ...CircuitBreakerConfig) OCREngine {
	return NewCircuitBreaker(engine, cfg...)
}

