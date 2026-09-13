package providers

import (
	"bytes"
	"context"
	"sync"

	"github.com/amirfaisalz/lensio/services/ocr"
)

// Magic markers in image bytes to trigger specific mock behaviors during testing.
var (
	MarkerUnsupportedDoc        = []byte("MOCK_UNSUPPORTED_DOC")
	MarkerOCRFailure            = []byte("MOCK_OCR_FAILURE")
	MarkerLowConfidence         = []byte("MOCK_LOW_CONFIDENCE")
	MarkerSIMDoc                = []byte("MOCK_SIM_DOC")
	MarkerSIMLowConfidence      = []byte("MOCK_SIM_LOW_CONFIDENCE")
	MarkerPassportDoc           = []byte("MOCK_PASSPORT_DOC")
	MarkerPassportLowConfidence = []byte("MOCK_PASSPORT_LOW_CONFIDENCE")
	MarkerNPWPDoc               = []byte("MOCK_NPWP_DOC")
	MarkerNPWPLowConfidence     = []byte("MOCK_NPWP_LOW_CONFIDENCE")
	MarkerKKDoc                 = []byte("MOCK_KK_DOC")
	MarkerKKLowConfidence       = []byte("MOCK_KK_LOW_CONFIDENCE")
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

	if bytes.Contains(image, MarkerPassportDoc) || bytes.Contains(image, MarkerPassportLowConfidence) {
		confidence := 0.98
		var passportData *ocr.PassportData
		if bytes.Contains(image, MarkerPassportLowConfidence) {
			confidence = 0.45
			passportData = &ocr.PassportData{
				PassportNumber: "X1234567",
			}
		} else {
			passportData = &ocr.PassportData{
				PassportNumber: "X1234567",
				FullName:       "BUDI SANTOSO",
				Nationality:    "IDN",
				DateOfBirth:    "1990-01-01",
				PlaceOfBirth:   "JAKARTA",
				Gender:         "LAKI-LAKI",
				IssueDate:      "2020-01-01",
				ExpiryDate:     "2030-01-01",
				IssuingOffice:  "KANIM JAKARTA SELATAN",
				MRZLine1:       "P<IDNSANTOSO<<BUDI<<<<<<<<<<<<<<<<<<<<<<<<<<",
				MRZLine2:       "X1234567<7IDN9001011M3001019<<<<<<<<<<<<<<<2",
			}
		}
		return &ocr.OCRResult{
			DocumentType: "passport",
			Confidence:   confidence,
			RawText:      "REPUBLIK INDONESIA PASPOR PASSPORT Jenis/Type: P Kode Negara/Country Code: IDN Nomor Paspor/Passport No: X1234567 Nama Lengkap/Full Name: BUDI SANTOSO Kewarganegaraan/Nationality: INDONESIA Tanggal Lahir/Date of Birth: 01-01-1990 Tempat Lahir/Place of Birth: JAKARTA Jenis Kelamin/Sex: LAKI-LAKI Tanggal Pengeluaran/Date of Issue: 01-01-2020 Tanggal Habis Berlaku/Date of Expiry: 01-01-2030 Kantor yang Mengeluarkan/Issuing Office: KANIM JAKARTA SELATAN P<IDNSANTOSO<<BUDI<<<<<<<<<<<<<<<<<<<<<<<<<< X1234567<7IDN9001011M3001019<<<<<<<<<<<<<<<2",
			PassportData: passportData,
		}, nil
	}

	if bytes.Contains(image, MarkerNPWPDoc) || bytes.Contains(image, MarkerNPWPLowConfidence) {
		confidence := 0.99
		var npwpData *ocr.NPWPData
		if bytes.Contains(image, MarkerNPWPLowConfidence) {
			confidence = 0.40
			npwpData = &ocr.NPWPData{
				NPWP: "092542943407000",
			}
		} else {
			npwpData = &ocr.NPWPData{
				NPWP:          "092542943407000",
				Nama:          "BUDI SANTOSO",
				NIK:           "3171010101900001",
				Alamat:        "JL. JENDERAL SUDIRMAN KAV. 21",
				Kelurahan:     "KARET KUNINGAN",
				Kecamatan:     "SETIABUDI",
				KotaKabupaten: "JAKARTA SELATAN",
				Provinsi:      "DKI JAKARTA",
				KPP:           "KPP PRATAMA JAKARTA SETIABUDI SATU",
				TanggalDaftar: "2015-08-17",
			}
		}
		return &ocr.OCRResult{
			DocumentType: "npwp",
			Confidence:   confidence,
			RawText:      "KEMENTERIAN KEUANGAN DIREKTORAT JENDERAL PAJAK NOMOR POKOK WAJIB PAJAK NPWP 09.254.294.3-407.000 NAMA BUDI SANTOSO NIK 3171010101900001 ALAMAT JL. JENDERAL SUDIRMAN KAV. 21 KPP PRATAMA JAKARTA SETIABUDI SATU TERDAFTAR 17-08-2015",
			NPWPData:     npwpData,
		}, nil
	}

	if bytes.Contains(image, MarkerKKDoc) || bytes.Contains(image, MarkerKKLowConfidence) {
		confidence := 0.99
		var kkData *ocr.KKData
		if bytes.Contains(image, MarkerKKLowConfidence) {
			confidence = 0.40
			kkData = &ocr.KKData{
				NomorKK: "3171010101200001",
			}
		} else {
			kkData = &ocr.KKData{
				NomorKK:            "3171010101200001",
				KepalaKeluarga:     "BUDI SANTOSO",
				Alamat:             "JL. SUDIRMAN NO. 12",
				RTRW:               "001/002",
				KodePos:            "12190",
				KelurahanDesa:      "SENAYAN",
				Kecamatan:          "KEBAYORAN BARU",
				KabupatenKota:      "JAKARTA SELATAN",
				Provinsi:           "DKI JAKARTA",
				TanggalDikeluarkan: "2020-01-01",
				AnggotaKeluarga: []ocr.KKFamilyMember{
					{
						Nama:             "BUDI SANTOSO",
						NIK:              "3171010101900001",
						JenisKelamin:     "LAKI-LAKI",
						TempatLahir:      "JAKARTA",
						TanggalLahir:     "1990-01-01",
						Agama:            "ISLAM",
						Pendidikan:       "STRATA I",
						JenisPekerjaan:   "KARYAWAN SWASTA",
						GolonganDarah:    "O",
						StatusPerkawinan: "KAWIN",
						StatusHubungan:   "KEPALA KELUARGA",
						Kewarganegaraan:  "WNI",
						NamaAyah:         "SANTOSO",
						NamaIbu:          "MARYAM",
					},
					{
						Nama:             "SITI AMINAH",
						NIK:              "3171014101920002",
						JenisKelamin:     "PEREMPUAN",
						TempatLahir:      "BANDUNG",
						TanggalLahir:     "1992-01-01",
						Agama:            "ISLAM",
						Pendidikan:       "STRATA I",
						JenisPekerjaan:   "IBU RUMAH TANGGA",
						GolonganDarah:    "A",
						StatusPerkawinan: "KAWIN",
						StatusHubungan:   "ISTRI",
						Kewarganegaraan:  "WNI",
						NamaAyah:         "AHMAD",
						NamaIbu:          "FATIMAH",
					},
					{
						Nama:             "RUDI SANTOSO",
						NIK:              "3171011505150003",
						JenisKelamin:     "LAKI-LAKI",
						TempatLahir:      "JAKARTA",
						TanggalLahir:     "2015-05-15",
						Agama:            "ISLAM",
						Pendidikan:       "BELUM/TIDAK BEKERJA",
						JenisPekerjaan:   "PELAJAR/MAHASISWA",
						GolonganDarah:    "O",
						StatusPerkawinan: "BELUM KAWIN",
						StatusHubungan:   "ANAK",
						Kewarganegaraan:  "WNI",
						NamaAyah:         "BUDI SANTOSO",
						NamaIbu:          "SITI AMINAH",
					},
				},
			}
		}
		return &ocr.OCRResult{
			DocumentType: "kk",
			Confidence:   confidence,
			RawText:      "REPUBLIK INDONESIA KARTU KELUARGA NO. KK 3171010101200001 KEPALA KELUARGA BUDI SANTOSO ALAMAT JL. SUDIRMAN NO. 12 RT/RW 001/002 DESA/KELURAHAN SENAYAN KECAMATAN KEBAYORAN BARU KABUPATEN/KOTA JAKARTA SELATAN PROVINSI DKI JAKARTA TANGGAL DIKELUARKAN 01-01-2020 BUDI SANTOSO 3171010101900001 LAKI-LAKI JAKARTA 01-01-1990 ISLAM STRATA I KARYAWAN SWASTA KAWIN KEPALA KELUARGA WNI SANTOSO MARYAM SITI AMINAH 3171014101920002 PEREMPUAN BANDUNG 01-01-1992 ISLAM STRATA I IBU RUMAH TANGGA KAWIN ISTRI WNI AHMAD FATIMAH RUDI SANTOSO 3171011505150003 LAKI-LAKI JAKARTA 15-05-2015 ISLAM BELUM/TIDAK BEKERJA PELAJAR/MAHASISWA BELUM KAWIN ANAK WNI BUDI SANTOSO SITI AMINAH",
			KKData:       kkData,
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
