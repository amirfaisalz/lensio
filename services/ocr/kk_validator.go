package ocr

import (
	"fmt"
	"strconv"
	"strings"
)

// Valid Indonesian Kartu Keluarga family relationship status (Status Hubungan Dalam Keluarga - SHDK).
var validKKRelations = map[string]struct{}{
	"KEPALA KELUARGA": {},
	"SUAMI":           {},
	"ISTRI":           {},
	"ANAK":            {},
	"MENANTU":         {},
	"CUCU":            {},
	"ORANG TUA":       {},
	"MERTUA":          {},
	"FAMILI LAIN":     {},
	"PEMBANTU":        {},
	"LAINNYA":         {},
}

// KKValidationResult provides detailed outcome of Kartu Keluarga verification.
type KKValidationResult struct {
	IsValid           bool     `json:"is_valid"`
	ProvinceCode      string   `json:"province_code,omitempty"`
	ProvinceName      string   `json:"province_name,omitempty"`
	TotalMembers      int      `json:"total_members"`
	HeadOfFamilyFound bool     `json:"head_of_family_found"`
	Errors            []string `json:"errors,omitempty"`
}

// CleanNomorKK strips whitespace, hyphens, and dots from a raw KK number.
// Runs in O(n) single pass with zero heap allocations beyond the string builder.
func CleanNomorKK(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c >= '0' && c <= '9' {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// ValidateNomorKK validates the 16-digit Indonesian Nomor Kartu Keluarga format:
// 1. Length must be exactly 16 digits.
// 2. Digits 1-2 must match an official Indonesian Province code (Kemendagri).
// 3. Digits 7-12 encode the date of issuance (DDMMYY), which must be a valid calendar date.
// 4. Digits 13-16 sequence number must not be 0000.
// Time Complexity: O(n) where n=16, zero heap allocations.
func ValidateNomorKK(nomorKK string) (bool, string, string, string) {
	cleaned := CleanNomorKK(nomorKK)
	if len(cleaned) != 16 {
		return false, "", "", fmt.Sprintf("nomor KK length must be exactly 16 digits, got %d", len(cleaned))
	}

	provCode := cleaned[0:2]
	provName, ok := validProvinces[provCode]
	if !ok {
		return false, provCode, "", fmt.Sprintf("invalid province code in nomor KK: %s", provCode)
	}

	day, _ := strconv.Atoi(cleaned[6:8])
	month, _ := strconv.Atoi(cleaned[8:10])
	year2D, _ := strconv.Atoi(cleaned[10:12])
	seq, _ := strconv.Atoi(cleaned[12:16])

	if seq == 0 {
		return false, provCode, provName, "sequence number in nomor KK cannot be 0000"
	}

	if month < 1 || month > 12 {
		return false, provCode, provName, fmt.Sprintf("invalid month %d in nomor KK", month)
	}

	// Days in month validation
	daysInMonth := [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if month == 2 {
		if year2D%4 == 0 {
			daysInMonth[1] = 29
		}
	}

	if day < 1 || day > daysInMonth[month-1] {
		return false, provCode, provName, fmt.Sprintf("invalid day %d for month %d in nomor KK", day, month)
	}

	return true, provCode, provName, ""
}

// ValidateKK validates a complete KKData record including family members list.
// Evaluates structure integrity, NIK per member, single head of household constraint,
// and cross-checks the kepala keluarga header against the member list.
func ValidateKK(data *KKData) KKValidationResult {
	if data == nil {
		return KKValidationResult{
			IsValid: false,
			Errors:  []string{"kartu keluarga data is nil"},
		}
	}

	var errors []string

	// 1. Validate Nomor KK
	validKK, provCode, provName, kkErr := ValidateNomorKK(data.NomorKK)
	if !validKK {
		errors = append(errors, kkErr)
	}

	// 2. Validate Kepala Keluarga header field
	headName := strings.TrimSpace(data.KepalaKeluarga)
	if headName == "" {
		errors = append(errors, "nama kepala keluarga is required")
	}

	// 3. Validate Address fields
	if strings.TrimSpace(data.Alamat) == "" {
		errors = append(errors, "alamat is required")
	}

	// 4. Validate Members list
	if len(data.AnggotaKeluarga) == 0 {
		errors = append(errors, "kartu keluarga must contain at least one family member")
		return KKValidationResult{
			IsValid:           false,
			ProvinceCode:      provCode,
			ProvinceName:      provName,
			TotalMembers:      0,
			HeadOfFamilyFound: false,
			Errors:            errors,
		}
	}

	headFound := false
	headMatchesHeader := false

	for i, m := range data.AnggotaKeluarga {
		idxStr := fmt.Sprintf("anggota[%d]", i)

		// Name validation
		if strings.TrimSpace(m.Nama) == "" {
			errors = append(errors, fmt.Sprintf("%s: nama is required", idxStr))
		}

		// NIK validation
		nikRes := ValidateNIK(m.NIK)
		if !nikRes.IsValid {
			errors = append(errors, fmt.Sprintf("%s: invalid NIK (%s): %s", idxStr, m.NIK, nikRes.Error))
		}

		// Gender validation
		cleanGender := strings.ToUpper(strings.TrimSpace(m.JenisKelamin))
		if _, ok := validGenders[cleanGender]; !ok {
			errors = append(errors, fmt.Sprintf("%s: invalid jenis kelamin '%s'", idxStr, m.JenisKelamin))
		}

		// Status Hubungan validation
		cleanRel := strings.ToUpper(strings.TrimSpace(m.StatusHubungan))
		if _, ok := validKKRelations[cleanRel]; !ok {
			errors = append(errors, fmt.Sprintf("%s: invalid status hubungan '%s'", idxStr, m.StatusHubungan))
		}

		if cleanRel == "KEPALA KELUARGA" {
			if headFound {
				errors = append(errors, fmt.Sprintf("%s: multiple kepala keluarga found in document", idxStr))
			}
			headFound = true
			if headName != "" && strings.EqualFold(strings.TrimSpace(m.Nama), headName) {
				headMatchesHeader = true
			}
		}
	}

	if !headFound {
		errors = append(errors, "no family member with status 'KEPALA KELUARGA' found")
	} else if headName != "" && !headMatchesHeader {
		matched := false
		for _, m := range data.AnggotaKeluarga {
			if strings.ToUpper(strings.TrimSpace(m.StatusHubungan)) == "KEPALA KELUARGA" {
				if strings.Contains(strings.ToUpper(m.Nama), strings.ToUpper(headName)) ||
					strings.Contains(strings.ToUpper(headName), strings.ToUpper(m.Nama)) {
					matched = true
					break
				}
			}
		}
		if !matched {
			errors = append(errors, "kepala keluarga header does not match member with status 'KEPALA KELUARGA'")
		}
	}

	return KKValidationResult{
		IsValid:           len(errors) == 0,
		ProvinceCode:      provCode,
		ProvinceName:      provName,
		TotalMembers:      len(data.AnggotaKeluarga),
		HeadOfFamilyFound: headFound,
		Errors:            errors,
	}
}
