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

// ClassifyDocument analyzes raw text tokens to classify if the document is an Indonesian KTP.
// Returns document type string ("ktp" or "unsupported") and a boolean indicator.
// Minimum 2 strong markers required for positive classification.
func ClassifyDocument(rawText string) (string, bool) {
	upper := strings.ToUpper(rawText)
	matches := 0

	for _, kw := range ktpKeywords {
		if strings.Contains(upper, kw) {
			matches++
		}
	}

	// Also check for 16-digit numeric pattern which strongly indicates an Indonesian identity card
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

	if matches >= 2 || (matches >= 1 && has16Digits) {
		return "ktp", true
	}

	return "unsupported", false
}
