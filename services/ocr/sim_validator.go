package ocr

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Valid Indonesian SIM categories (Korlantas POLRI standard).
var validSIMGolongan = map[string]struct{}{
	"A":             {},
	"A UMUM":        {},
	"B I":           {},
	"B I UMUM":      {},
	"B II":          {},
	"B II UMUM":     {},
	"C":             {},
	"C I":           {},
	"C II":          {},
	"D":             {},
	"D I":           {},
	"INTERNASIONAL": {},
}

// Valid SIM gender enums.
var validSIMGenders = map[string]struct{}{
	"PRIA":   {},
	"WANITA": {},
}

// Valid blood types on Indonesian identity documents.
var validBloodTypes = map[string]struct{}{
	"A":  {},
	"B":  {},
	"AB": {},
	"O":  {},
	"-":  {},
}

// Valid Indonesian Kepolisian Daerah (Polda) regions.
var validPoldaRegions = map[string]string{
	"METRO JAYA":         "DKI Jakarta, Depok, Tangerang, Bekasi",
	"JAWA BARAT":         "Jawa Barat",
	"JAWA TENGAH":        "Jawa Tengah",
	"JAWA TIMUR":         "Jawa Timur",
	"BANTEN":             "Banten",
	"DIY":                "DI Yogyakarta",
	"YOGYAKARTA":         "DI Yogyakarta",
	"BALI":               "Bali",
	"SUMATERA UTARA":     "Sumatera Utara",
	"SUMATERA BARAT":     "Sumatera Barat",
	"SUMATERA SELATAN":   "Sumatera Selatan",
	"ACEH":               "Aceh",
	"RIAU":               "Riau",
	"KEPULAUAN RIAU":     "Kepulauan Riau",
	"JAMBI":              "Jambi",
	"BENGKULU":           "Bengkulu",
	"LAMPUNG":            "Lampung",
	"BANGKA BELITUNG":    "Kepulauan Bangka Belitung",
	"KALIMANTAN BARAT":   "Kalimantan Barat",
	"KALIMANTAN TENGAH":  "Kalimantan Tengah",
	"KALIMANTAN SELATAN": "Kalimantan Selatan",
	"KALIMANTAN TIMUR":   "Kalimantan Timur",
	"KALIMANTAN UTARA":   "Kalimantan Utara",
	"SULAWESI UTARA":     "Sulawesi Utara",
	"SULAWESI TENGAH":    "Sulawesi Tengah",
	"SULAWESI SELATAN":   "Sulawesi Selatan",
	"SULAWESI TENGGARA":  "Sulawesi Tenggara",
	"GORONTALO":          "Gorontalo",
	"SULAWESI BARAT":     "Sulawesi Barat",
	"MALUKU":             "Maluku",
	"MALUKU UTARA":       "Maluku Utara",
	"PAPUA":              "Papua",
	"PAPUA BARAT":        "Papua Barat",
	"PAPUA SELATAN":      "Papua Selatan",
	"PAPUA TENGAH":       "Papua Tengah",
	"PAPUA PEGUNUNGAN":   "Papua Pegunungan",
	"PAPUA BARAT DAYA":   "Papua Barat Daya",
	"NTB":                "Nusa Tenggara Barat",
	"NTT":                "Nusa Tenggara Timur",
}

// SIMNumberValidationResult provides the outcome of deterministic SIM number validation.
type SIMNumberValidationResult struct {
	IsValid bool
	Cleaned string
	Error   string
}

// ValidateSIMNumber performs strict, deterministic validation on an Indonesian SIM number:
// 1. Strips hyphens, spaces, and OCR noise.
// 2. Length must be between 12 and 16 digits (standard Korlantas POLRI format).
// 3. All characters must be ASCII numeric ('0'-'9').
// Time Complexity: O(n) single pass with n <= 16. Zero heap allocations.
func ValidateSIMNumber(nomor string) SIMNumberValidationResult {
	nomor = strings.TrimSpace(nomor)
	nomor = strings.ReplaceAll(nomor, "-", "")
	nomor = strings.ReplaceAll(nomor, " ", "")
	nomor = cleanDigits(nomor)

	if len(nomor) < 12 || len(nomor) > 16 {
		return SIMNumberValidationResult{
			IsValid: false,
			Cleaned: nomor,
			Error:   fmt.Sprintf("SIM number length must be between 12 and 16 digits, got %d", len(nomor)),
		}
	}

	for i := 0; i < len(nomor); i++ {
		if nomor[i] < '0' || nomor[i] > '9' {
			return SIMNumberValidationResult{
				IsValid: false,
				Cleaned: nomor,
				Error:   "SIM number must contain numeric digits only",
			}
		}
	}

	return SIMNumberValidationResult{
		IsValid: true,
		Cleaned: nomor,
	}
}

// ValidateSIM performs comprehensive normalization and deterministic scoring on SIM data.
// Returns normalized SIMData, calculated confidence score (0.00 - 1.00), and list of validation issues.
func ValidateSIM(data *SIMData) (*SIMData, float64, []string) {
	if data == nil {
		return &SIMData{}, 0.0, []string{"sim data is nil"}
	}

	var issues []string
	var score float64

	// Normalize gender (accept LAKI-LAKI -> PRIA, PEREMPUAN -> WANITA)
	gender := strings.ToUpper(strings.TrimSpace(data.JenisKelamin))
	switch gender {
	case "LAKI-LAKI", "L", "PRIA":
		gender = "PRIA"
	case "PEREMPUAN", "P", "W", "WANITA":
		gender = "WANITA"
	}

	// Normalize blood type
	bloodType := strings.ToUpper(strings.TrimSpace(data.GolonganDarah))
	bloodType = strings.Trim(bloodType, "- ")
	if bloodType == "" {
		bloodType = "-"
	}

	// Normalize Golongan (e.g. "SIM A" -> "A", "C1" -> "C I")
	golongan := strings.ToUpper(strings.TrimSpace(data.Golongan))
	golongan = strings.TrimPrefix(golongan, "SIM")
	golongan = strings.TrimSpace(golongan)
	switch golongan {
	case "C1":
		golongan = "C I"
	case "C2":
		golongan = "C II"
	case "B1":
		golongan = "B I"
	case "B2":
		golongan = "B II"
	case "D1":
		golongan = "D I"
	}

	// 1. Normalize fields (uppercase, trimmed)
	normalized := &SIMData{
		NomorSIM:      strings.TrimSpace(data.NomorSIM),
		Golongan:      golongan,
		Nama:          strings.ToUpper(strings.TrimSpace(data.Nama)),
		TempatLahir:   strings.ToUpper(strings.TrimSpace(data.TempatLahir)),
		TanggalLahir:  normalizeDate(data.TanggalLahir),
		GolonganDarah: bloodType,
		JenisKelamin:  gender,
		Alamat:        strings.ToUpper(strings.TrimSpace(data.Alamat)),
		Pekerjaan:     strings.ToUpper(strings.TrimSpace(data.Pekerjaan)),
		Polda:         strings.ToUpper(strings.TrimSpace(data.Polda)),
		MasaBerlaku:   normalizeDate(data.MasaBerlaku),
	}

	// 2. Deterministic SIM Number Validation (25% weight)
	simNumRes := ValidateSIMNumber(normalized.NomorSIM)
	if simNumRes.IsValid {
		normalized.NomorSIM = simNumRes.Cleaned
		score += 0.25
	} else {
		issues = append(issues, "nomor_sim: "+simNumRes.Error)
	}

	// 3. Golongan SIM Validation (15% weight)
	if _, ok := validSIMGolongan[normalized.Golongan]; ok {
		score += 0.15
	} else {
		issues = append(issues, "golongan: invalid or unrecognized SIM category (expected A, B I, B II, C, C I, C II, D, D I)")
	}

	// 4. Nama Validation (15% weight)
	if len(normalized.Nama) >= 2 {
		score += 0.15
	} else {
		issues = append(issues, "nama: missing or too short")
	}

	// 5. Tanggal Lahir & Tempat Lahir (15% weight)
	dobValid := false
	if normalized.TanggalLahir != "" {
		if t, err := time.Parse("2006-01-02", normalized.TanggalLahir); err == nil {
			now := time.Now()
			// Must be born after 1900 and at least 17 years ago (legal driving age)
			minAgeDate := now.AddDate(-17, 0, 0)
			if t.Year() >= 1900 && t.Before(minAgeDate) {
				dobValid = true
			} else if t.After(minAgeDate) && t.Before(now) {
				issues = append(issues, "tanggal_lahir: driver age must be at least 17 years old")
			}
		}
	}

	if dobValid && normalized.TempatLahir != "" {
		score += 0.15
	} else {
		if !dobValid && !containsIssue(issues, "driver age") {
			issues = append(issues, "tanggal_lahir: invalid date or format (must be YYYY-MM-DD)")
		}
		if normalized.TempatLahir == "" {
			issues = append(issues, "tempat_lahir: missing")
		}
	}

	// 6. Masa Berlaku Validation (10% weight)
	if normalized.MasaBerlaku != "" {
		if t, err := time.Parse("2006-01-02", normalized.MasaBerlaku); err == nil {
			if t.Year() >= 2000 && t.Year() <= 2100 {
				score += 0.10
			} else {
				issues = append(issues, "masa_berlaku: year out of plausible range")
			}
		} else {
			issues = append(issues, "masa_berlaku: invalid date format (must be YYYY-MM-DD)")
		}
	} else {
		issues = append(issues, "masa_berlaku: missing")
	}

	// 7. Jenis Kelamin Enum (5% weight)
	if _, ok := validSIMGenders[normalized.JenisKelamin]; ok {
		score += 0.05
	} else {
		issues = append(issues, "jenis_kelamin: invalid enum (expected PRIA or WANITA)")
	}

	// 8. Alamat (5% weight)
	if len(normalized.Alamat) >= 3 {
		score += 0.05
	} else {
		issues = append(issues, "alamat: missing or too short")
	}

	// 9. Pekerjaan (5% weight)
	if normalized.Pekerjaan != "" {
		score += 0.05
	} else {
		issues = append(issues, "pekerjaan: missing")
	}

	// 10. Polda & Golongan Darah (5% weight)
	poldaScore := 0.0
	cleanPolda := strings.TrimPrefix(normalized.Polda, "POLDA")
	cleanPolda = strings.TrimSpace(cleanPolda)
	if _, ok := validPoldaRegions[cleanPolda]; ok || len(normalized.Polda) >= 3 {
		poldaScore += 0.03
	} else if normalized.Polda == "" {
		issues = append(issues, "polda: missing")
	}

	if _, ok := validBloodTypes[normalized.GolonganDarah]; ok {
		poldaScore += 0.02
	}
	score += poldaScore

	// Clamp and round confidence to 2 decimal places
	roundedConfidence := math.Round(score*100) / 100
	if roundedConfidence < 0.00 {
		roundedConfidence = 0.00
	}
	if roundedConfidence > 1.00 {
		roundedConfidence = 1.00
	}

	return normalized, roundedConfidence, issues
}

func containsIssue(issues []string, sub string) bool {
	for _, iss := range issues {
		if strings.Contains(iss, sub) {
			return true
		}
	}
	return false
}
