package ocr

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Valid Indonesian Province Codes (Kemendagri / BPS Standard).
var validProvinces = map[string]string{
	"11": "ACEH",
	"12": "SUMATERA UTARA",
	"13": "SUMATERA BARAT",
	"14": "RIAU",
	"15": "JAMBI",
	"16": "SUMATERA SELATAN",
	"17": "BENGKULU",
	"18": "LAMPUNG",
	"19": "KEPULAUAN BANGKA BELITUNG",
	"21": "KEPULAUAN RIAU",
	"31": "DKI JAKARTA",
	"32": "JAWA BARAT",
	"33": "JAWA TENGAH",
	"34": "DAERAH ISTIMEWA YOGYAKARTA",
	"35": "JAWA TIMUR",
	"36": "BANTEN",
	"51": "BALI",
	"52": "NUSA TENGGARA BARAT",
	"53": "NUSA TENGGARA TIMUR",
	"61": "KALIMANTAN BARAT",
	"62": "KALIMANTAN TENGAH",
	"63": "KALIMANTAN SELATAN",
	"64": "KALIMANTAN TIMUR",
	"65": "KALIMANTAN UTARA",
	"71": "SULAWESI UTARA",
	"72": "SULAWESI TENGAH",
	"73": "SULAWESI SELATAN",
	"74": "SULAWESI TENGGARA",
	"75": "GORONTALO",
	"76": "SULAWESI BARAT",
	"81": "MALUKU",
	"82": "MALUKU UTARA",
	"91": "PAPUA BARAT",
	"92": "PAPUA BARAT DAYA",
	"93": "PAPUA SELATAN",
	"94": "PAPUA TENGAH",
	"95": "PAPUA PEGUNUNGAN",
	"96": "PAPUA",
}

// Valid standard KTP enums.
var (
	validGenders = map[string]struct{}{
		"LAKI-LAKI": {},
		"PEREMPUAN": {},
	}

	validReligions = map[string]struct{}{
		"ISLAM":                 {},
		"KRISTEN":               {},
		"KATOLIK":               {},
		"HINDU":                 {},
		"BUDDHA":                {},
		"KHONGHUCU":             {},
		"PENGHAYAT KEPERCAYAAN": {},
	}

	validMaritalStatuses = map[string]struct{}{
		"BELUM KAWIN": {},
		"KAWIN":       {},
		"CERAI HIDUP": {},
		"CERAI MATI":  {},
	}

	validCitizenships = map[string]struct{}{
		"WNI": {},
		"WNA": {},
	}
)

// NIKValidationResult provides detailed outcome of deterministic NIK verification.
type NIKValidationResult struct {
	IsValid      bool
	ProvinceCode string
	ProvinceName string
	RegencyCode  string
	DistrictCode string
	IsFemale     bool
	BirthDay     int
	BirthMonth   int
	BirthYear2D  int
	Sequence     int
	Error        string
}

// ValidateNIK performs strict, deterministic validation on a 16-digit Indonesian NIK:
// 1. Length must be exactly 16 digits.
// 2. All characters must be ASCII numeric ('0'-'9').
// 3. Province code (digits 1-2) must match official Kemendagri codes (O(1) map lookup).
// 4. Encoded birth date (digits 7-12, DDMMYY):
//   - Day: 01-31 (Male) or 41-71 (Female: Day + 40)
//   - Month: 01-12
//   - Calendar validity check (month length, leap year bounds)
//
// 5. Sequence number (digits 13-16) must be > 0000.
// Time Complexity: O(n) single pass with n=16. Zero heap allocations.
func ValidateNIK(nik string) NIKValidationResult {
	nik = strings.TrimSpace(nik)
	if len(nik) != 16 {
		return NIKValidationResult{
			IsValid: false,
			Error:   fmt.Sprintf("NIK length must be exactly 16 digits, got %d", len(nik)),
		}
	}

	for i := 0; i < 16; i++ {
		if nik[i] < '0' || nik[i] > '9' {
			return NIKValidationResult{
				IsValid: false,
				Error:   "NIK must contain digits only",
			}
		}
	}

	provCode := nik[0:2]
	provName, ok := validProvinces[provCode]
	if !ok {
		return NIKValidationResult{
			IsValid: false,
			Error:   fmt.Sprintf("unknown or invalid province code: %s", provCode),
		}
	}

	// Digits 3-4 are the kabupaten/kota code and 5-6 the kecamatan code. Neither
	// is ever "00" in a real NIK; both were previously accepted unchecked.
	if nik[2:4] == "00" {
		return NIKValidationResult{
			IsValid: false,
			Error:   "kabupaten/kota code cannot be 00",
		}
	}
	if nik[4:6] == "00" {
		return NIKValidationResult{
			IsValid: false,
			Error:   "kecamatan code cannot be 00",
		}
	}

	rawDay, _ := strconv.Atoi(nik[6:8])
	month, _ := strconv.Atoi(nik[8:10])
	year2D, _ := strconv.Atoi(nik[10:12])
	seq, _ := strconv.Atoi(nik[12:16])

	if seq == 0 {
		return NIKValidationResult{
			IsValid: false,
			Error:   "sequence number must be greater than 0000",
		}
	}

	// Women are encoded as day + 40, so raw values 32..40 and >71 are impossible.
	isFemale := rawDay > 40
	realDay := rawDay
	if isFemale {
		realDay = rawDay - 40
	}

	if rawDay > 31 && rawDay <= 40 {
		return NIKValidationResult{
			IsValid: false,
			Error:   fmt.Sprintf("invalid birth day encoding in NIK: %02d (32-40 is neither a day nor a day+40)", rawDay),
		}
	}

	if realDay < 1 || realDay > 31 {
		return NIKValidationResult{
			IsValid: false,
			Error:   fmt.Sprintf("invalid day of birth in NIK: %d", realDay),
		}
	}

	if month < 1 || month > 12 {
		return NIKValidationResult{
			IsValid: false,
			Error:   fmt.Sprintf("invalid month in NIK: %d", month),
		}
	}

	// Month days check
	maxDays := 31
	switch month {
	case 4, 6, 9, 11:
		maxDays = 30
	case 2:
		// Year is 2-digit; allow up to 29 for potential leap years
		maxDays = 29
	}

	if realDay > maxDays {
		return NIKValidationResult{
			IsValid: false,
			Error:   fmt.Sprintf("day %d exceeds max days (%d) for month %d", realDay, maxDays, month),
		}
	}

	return NIKValidationResult{
		IsValid:      true,
		ProvinceCode: provCode,
		ProvinceName: provName,
		RegencyCode:  nik[2:4],
		DistrictCode: nik[4:6],
		IsFemale:     isFemale,
		BirthDay:     realDay,
		BirthMonth:   month,
		BirthYear2D:  year2D,
		Sequence:     seq,
	}
}

// ValidateKTP performs comprehensive normalization and deterministic scoring on KTP data.
// Returns normalized KTPData, calculated confidence score (0.00 - 1.00), and list of validation issues.
func ValidateKTP(data *KTPData) (*KTPData, float64, []string) {
	if data == nil {
		return &KTPData{}, 0.0, []string{"ktp data is nil"}
	}

	var issues []string
	var score float64

	// 1. Normalize fields (uppercase, trimmed)
	normalized := &KTPData{
		NIK:              strings.TrimSpace(data.NIK),
		Nama:             strings.ToUpper(strings.TrimSpace(data.Nama)),
		TempatLahir:      strings.ToUpper(strings.TrimSpace(data.TempatLahir)),
		TanggalLahir:     strings.TrimSpace(data.TanggalLahir),
		JenisKelamin:     strings.ToUpper(strings.TrimSpace(data.JenisKelamin)),
		Alamat:           strings.ToUpper(strings.TrimSpace(data.Alamat)),
		RTRW:             strings.TrimSpace(data.RTRW),
		Kelurahan:        strings.ToUpper(strings.TrimSpace(data.Kelurahan)),
		Kecamatan:        strings.ToUpper(strings.TrimSpace(data.Kecamatan)),
		Agama:            strings.ToUpper(strings.TrimSpace(data.Agama)),
		StatusPerkawinan: strings.ToUpper(strings.TrimSpace(data.StatusPerkawinan)),
		Pekerjaan:        strings.ToUpper(strings.TrimSpace(data.Pekerjaan)),
		Kewarganegaraan:  strings.ToUpper(strings.TrimSpace(data.Kewarganegaraan)),
	}

	// 2. Deterministic NIK Validation (30% weight)
	nikRes := ValidateNIK(normalized.NIK)
	if nikRes.IsValid {
		score += 0.30
	} else {
		issues = append(issues, "nik: "+nikRes.Error)
	}

	// 3. Nama validation (15% weight)
	if len(normalized.Nama) >= 2 {
		score += 0.15
	} else {
		issues = append(issues, "nama: missing or too short")
	}

	// 4. Tanggal Lahir & Tempat Lahir (15% weight)
	dobValid := false
	if normalized.TanggalLahir != "" {
		if t, err := time.Parse("2006-01-02", normalized.TanggalLahir); err == nil {
			if t.Year() >= 1900 && t.Before(time.Now()) {
				dobValid = true
			}
		}
	}

	if dobValid && normalized.TempatLahir != "" {
		score += 0.15
	} else {
		if !dobValid {
			issues = append(issues, "tanggal_lahir: invalid date or format (must be YYYY-MM-DD)")
		}
		if normalized.TempatLahir == "" {
			issues = append(issues, "tempat_lahir: missing")
		}
	}

	// 5. Cross-check NIK with DOB & Gender (10% weight)
	crossCheckPassed := true
	if nikRes.IsValid {
		if dobValid {
			t, _ := time.Parse("2006-01-02", normalized.TanggalLahir)
			if t.Day() != nikRes.BirthDay || int(t.Month()) != nikRes.BirthMonth || (t.Year()%100) != nikRes.BirthYear2D {
				crossCheckPassed = false
				issues = append(issues, "nik: birth date mismatch with tanggal_lahir")
			}
		}
		if normalized.JenisKelamin != "" {
			if (normalized.JenisKelamin == "PEREMPUAN" && !nikRes.IsFemale) ||
				(normalized.JenisKelamin == "LAKI-LAKI" && nikRes.IsFemale) {
				crossCheckPassed = false
				issues = append(issues, "nik: gender encoding mismatch with jenis_kelamin")
			}
		}
	} else {
		crossCheckPassed = false
	}

	if crossCheckPassed && nikRes.IsValid && dobValid {
		score += 0.10
	} else if nikRes.IsValid && !crossCheckPassed {
		score -= 0.15
	}

	// 6. Jenis Kelamin Enum (5% weight)
	if _, ok := validGenders[normalized.JenisKelamin]; ok {
		score += 0.05
	} else {
		issues = append(issues, "jenis_kelamin: invalid enum (expected LAKI-LAKI or PEREMPUAN)")
	}

	// 7. Alamat & RT/RW (10% weight)
	if normalized.Alamat != "" {
		score += 0.05
	} else {
		issues = append(issues, "alamat: missing")
	}
	if normalized.RTRW != "" {
		score += 0.05
	} else {
		issues = append(issues, "rt_rw: missing")
	}

	// 8. Kelurahan & Kecamatan (5% weight)
	if normalized.Kelurahan != "" && normalized.Kecamatan != "" {
		score += 0.05
	} else {
		if normalized.Kelurahan == "" {
			issues = append(issues, "kelurahan: missing")
		}
		if normalized.Kecamatan == "" {
			issues = append(issues, "kecamatan: missing")
		}
	}

	// 9. Agama, Status Perkawinan, Kewarganegaraan Enums (10% weight)
	enumScore := 0.0
	if _, ok := validReligions[normalized.Agama]; ok {
		enumScore += 0.04
	}
	if _, ok := validMaritalStatuses[normalized.StatusPerkawinan]; ok {
		enumScore += 0.03
	}
	if _, ok := validCitizenships[normalized.Kewarganegaraan]; ok {
		enumScore += 0.03
	} else {
		// Default to WNI if omitted or recognizable
		if normalized.Kewarganegaraan == "" || normalized.Kewarganegaraan == "INDONESIA" {
			normalized.Kewarganegaraan = "WNI"
			enumScore += 0.03
		}
	}
	score += enumScore

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
