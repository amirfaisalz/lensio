package ocr

import (
	"strings"
)

// List of distinctive textual markers present on official Indonesian KTPs.
var ktpKeywords = []string{
	"REPUBLIK INDONESIA",
	"KARTU TANDA PENDUDUK",
	"PROVINSI",
	"NIK",
	"TEMPAT/TGL LAHIR",
	"JENIS KELAMIN",
	"GOL. DARAH",
	"ALAMAT",
	"RT/RW",
	"KEL/DESA",
	"KELURAHAN",
	"KECAMATAN",
	"AGAMA",
	"STATUS PERKAWINAN",
	"PEKERJAAN",
	"KEWARGANEGARAAN",
	"BERLAKU HINGGA",
}

// List of distinctive textual markers present on official Indonesian SIMs.
var simKeywords = []string{
	"SURAT IZIN MENGEMUDI",
	"DRIVING LICENSE",
	"KEPOLISIAN NEGARA REPUBLIK INDONESIA",
	"POLRI",
	"KORLANTAS",
	"GOL. SIM",
	"GOLONGAN SIM",
	"MASA BERLAKU",
	"BERLAKU S/D",
	"BERLAKU HINGGA",
	"SIM A",
	"SIM B",
	"SIM C",
	"SIM D",
	"POLDA",
}

// List of distinctive textual markers present on official Indonesian Passports.
var passportKeywords = []string{
	"PASPOR",
	"PASSPORT",
	"KANTOR IMIGRASI",
	"KANIM",
	"KODE NEGARA",
	"COUNTRY CODE",
	"NOMOR PASPOR",
	"PASSPORT NO",
	"DATE OF BIRTH",
	"DATE OF EXPIRY",
	"DATE OF ISSUE",
	"ISSUING OFFICE",
	"P<IDN",
}

// ClassifyDocument analyzes raw text tokens to classify if the document is an Indonesian KTP, SIM, or Passport.
// Returns document type string ("ktp", "sim", "passport", or "unsupported") and a boolean indicator of recognition.
// Minimum 2 strong markers required for positive classification (or 1 explicit title marker).
func ClassifyDocument(rawText string) (string, bool) {
	upper := strings.ToUpper(rawText)

	// Check Passport first if explicit title or MRZ prefix exists
	passportMatches := 0
	hasExplicitPassportTitle := (strings.Contains(upper, "PASPOR") || strings.Contains(upper, "PASSPORT")) && (strings.Contains(upper, "INDONESIA") || strings.Contains(upper, "IDN"))
	for _, kw := range passportKeywords {
		if strings.Contains(upper, kw) {
			passportMatches++
		}
	}

	// Check SIM
	simMatches := 0
	hasExplicitSIMTitle := strings.Contains(upper, "SURAT IZIN MENGEMUDI") || strings.Contains(upper, "DRIVING LICENSE")
	for _, kw := range simKeywords {
		if strings.Contains(upper, kw) {
			simMatches++
		}
	}

	ktpMatches := 0
	hasExplicitKTPTitle := strings.Contains(upper, "KARTU TANDA PENDUDUK") || strings.Contains(upper, "KTP")
	for _, kw := range ktpKeywords {
		if strings.Contains(upper, kw) {
			ktpMatches++
		}
	}

	// Check for 16-digit numeric pattern
	has16Digits := false
	for _, word := range strings.Fields(upper) {
		clean := strings.Trim(word, ":;,-.")
		if len(clean) == 16 {
			numeric := true
			for i := 0; i < 16; i++ {
				if clean[i] < '0' || clean[i] > '9' {
					numeric = false
					break
				}
			}
			if numeric {
				has16Digits = true
				break
			}
		}
	}

	if hasExplicitPassportTitle || (passportMatches >= 2 && passportMatches > ktpMatches && passportMatches > simMatches) {
		return "passport", true
	}

	if hasExplicitSIMTitle || (simMatches >= 2 && simMatches > ktpMatches) {
		return "sim", true
	}

	if hasExplicitKTPTitle || ktpMatches >= 2 || (ktpMatches >= 1 && has16Digits) {
		return "ktp", true
	}

	if simMatches >= 2 {
		return "sim", true
	}

	if passportMatches >= 2 {
		return "passport", true
	}

	return "unsupported", false
}
