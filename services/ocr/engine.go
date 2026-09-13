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

	// ErrUnsupportedSIMDocument indicates the image is not an Indonesian SIM.
	ErrUnsupportedSIMDocument = errors.New("uploaded document was not identified as an Indonesian SIM")

	// ErrUnsupportedPassportDocument indicates the image is not an Indonesian Passport.
	ErrUnsupportedPassportDocument = errors.New("uploaded document was not identified as an Indonesian Passport")

	// ErrUnsupportedNPWPDocument indicates the image is not an Indonesian NPWP.
	ErrUnsupportedNPWPDocument = errors.New("uploaded document was not identified as an Indonesian NPWP")

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

// SIMData represents structured fields extracted from an Indonesian SIM (Surat Izin Mengemudi).
type SIMData struct {
	NomorSIM      string `json:"nomor_sim"`
	Golongan      string `json:"golongan"`          // A | B I | B II | C | C I | C II | D | D I | A UMUM | B I UMUM | B II UMUM
	Nama          string `json:"nama"`
	TempatLahir   string `json:"tempat_lahir"`
	TanggalLahir  string `json:"tanggal_lahir"`     // YYYY-MM-DD
	GolonganDarah string `json:"golongan_darah"`   // A | B | AB | O | -
	JenisKelamin  string `json:"jenis_kelamin"`     // PRIA | WANITA
	Alamat        string `json:"alamat"`
	Pekerjaan     string `json:"pekerjaan"`
	Polda         string `json:"polda"`             // e.g. METRO JAYA
	MasaBerlaku   string `json:"masa_berlaku"`      // YYYY-MM-DD
}

// PassportData represents structured fields extracted from an Indonesian Passport (Paspor Republik Indonesia).
type PassportData struct {
	PassportNumber string `json:"passport_number"`
	FullName       string `json:"full_name"`
	Nationality    string `json:"nationality"`
	DateOfBirth    string `json:"date_of_birth"` // YYYY-MM-DD
	PlaceOfBirth   string `json:"place_of_birth"`
	Gender         string `json:"gender"`        // LAKI-LAKI | PEREMPUAN
	IssueDate      string `json:"issue_date"`    // YYYY-MM-DD
	ExpiryDate     string `json:"expiry_date"`   // YYYY-MM-DD
	IssuingOffice  string `json:"issuing_office"`
	MRZLine1       string `json:"mrz_line1,omitempty"`
	MRZLine2       string `json:"mrz_line2,omitempty"`
}

// NPWPData represents structured fields extracted from an Indonesian NPWP (Nomor Pokok Wajib Pajak).
type NPWPData struct {
	NPWP          string `json:"npwp"`                    // 15-digit or 16-digit normalized numeric string
	Nama          string `json:"nama"`                    // Taxpayer full name or corporate entity name
	NIK           string `json:"nik,omitempty"`           // 16-digit NIK (for individual/OP cards)
	Alamat        string `json:"alamat"`                  // Registered tax address
	Kelurahan     string `json:"kelurahan,omitempty"`
	Kecamatan     string `json:"kecamatan,omitempty"`
	KotaKabupaten string `json:"kota_kabupaten,omitempty"`
	Provinsi      string `json:"provinsi,omitempty"`
	KPP           string `json:"kpp"`                     // Registered Tax Office name or code
	TanggalDaftar string `json:"tanggal_daftar,omitempty"` // YYYY-MM-DD
}

// OCRResult represents the complete structured result of an OCR extraction.
//nolint:revive // spec mandates ocr.OCRResult naming
type OCRResult struct {
	DocumentType string        `json:"document_type"`
	Confidence   float64       `json:"confidence"`
	RawText      string        `json:"raw_text,omitempty"`
	Data         *KTPData      `json:"data,omitempty"`
	SIMData      *SIMData      `json:"sim_data,omitempty"`
	PassportData *PassportData `json:"passport_data,omitempty"`
	NPWPData     *NPWPData     `json:"npwp_data,omitempty"`
}

// OCREngine defines the pluggable document extraction contract.
//nolint:revive // spec mandates ocr.OCREngine naming
type OCREngine interface {
	Extract(ctx context.Context, image []byte) (*OCRResult, error)
}

// WithCircuitBreaker wraps an existing OCREngine with circuit breaking capabilities.
func WithCircuitBreaker(engine OCREngine, cfg ...CircuitBreakerConfig) OCREngine {
	return NewCircuitBreaker(engine, cfg...)
}

