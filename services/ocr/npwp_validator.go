package ocr

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrNPWPInvalidLength indicates NPWP does not have 15 or 16 numeric digits.
	ErrNPWPInvalidLength = errors.New("npwp must contain exactly 15 or 16 numeric digits")

	// ErrNPWPNonNumeric indicates NPWP contains non-numeric characters.
	ErrNPWPNonNumeric = errors.New("npwp must contain only numeric characters")

	// ErrNPWPInvalidTaxpayerType indicates invalid taxpayer category prefix.
	ErrNPWPInvalidTaxpayerType = errors.New("npwp taxpayer category code cannot be 00")

	// ErrNPWPInvalidChecksum indicates the 9th digit Luhn checksum mismatch.
	ErrNPWPInvalidChecksum = errors.New("npwp check digit verification failed")

	// ErrNPWPInvalidKPP indicates invalid KPP (tax office) code.
	ErrNPWPInvalidKPP = errors.New("npwp kpp code cannot be 000")
)

// CleanNPWP strips dots, hyphens, spaces, and formatting characters from raw NPWP string.
// Complexity: Time O(n), Space O(n) where n <= 32.
func CleanNPWP(raw string) string {
	var sb strings.Builder
	sb.Grow(len(raw))
	for i := 0; i < len(raw); i++ {
		b := raw[i]
		if b >= '0' && b <= '9' {
			sb.WriteByte(b)
		}
	}
	return sb.String()
}

// CalculateNPWPCheckDigit computes the 9th digit of a 15-digit NPWP as a Luhn
// checksum over the first eight digits.
//
// Caveat worth stating plainly: the Directorate General of Taxes has never
// published this algorithm. Luhn-over-eight is the convention the ecosystem
// converged on and it holds for every NPWP we can check, but it is a community
// heuristic, not a specification. A number failing this check is very likely
// mistyped or misread; a number passing it is structurally plausible, not
// proven to be issued. Do not present it to end users as proof of registration.
func CalculateNPWPCheckDigit(first8Digits string) (int, error) {
	if len(first8Digits) != 8 {
		return -1, fmt.Errorf("expected 8 digits, got %d", len(first8Digits))
	}

	weights := [8]int{1, 2, 1, 2, 1, 2, 1, 2}
	sum := 0

	for i := 0; i < 8; i++ {
		b := first8Digits[i]
		if b < '0' || b > '9' {
			return -1, ErrNPWPNonNumeric
		}
		digit := int(b - '0')
		prod := digit * weights[i]
		if prod > 9 {
			prod -= 9
		}
		sum += prod
	}

	checkDigit := (10 - (sum % 10)) % 10
	return checkDigit, nil
}

// ValidateNPWP15 deterministically validates a 15-digit Indonesian NPWP.
// Complexity: Time O(1), Space O(1).
func ValidateNPWP15(cleaned string) (bool, error) {
	if len(cleaned) != 15 {
		return false, ErrNPWPInvalidLength
	}

	for i := 0; i < 15; i++ {
		if cleaned[i] < '0' || cleaned[i] > '9' {
			return false, ErrNPWPNonNumeric
		}
	}

	// 1. Taxpayer category code (digits 1-2): cannot be "00"
	if cleaned[0] == '0' && cleaned[1] == '0' {
		return false, ErrNPWPInvalidTaxpayerType
	}

	// 2. Check digit (digit 9)
	expectedCheck, err := CalculateNPWPCheckDigit(cleaned[:8])
	if err != nil {
		return false, err
	}
	actualCheck := int(cleaned[8] - '0')
	if actualCheck != expectedCheck {
		return false, fmt.Errorf("%w: expected %d, got %d", ErrNPWPInvalidChecksum, expectedCheck, actualCheck)
	}

	// 3. KPP code (digits 10-12): cannot be "000"
	if cleaned[9:12] == "000" {
		return false, ErrNPWPInvalidKPP
	}

	return true, nil
}

// ValidateNPWP16 validates a 16-digit Indonesian NPWP (PMK 112/PMK.03/2022).
// - For Corporate/Foreign (Badan/WNA): starts with '0', followed by 15-digit NPWP.
// - For Individuals (Orang Pribadi WNI): 16-digit NIK format.
// Complexity: Time O(1), Space O(1).
func ValidateNPWP16(cleaned string) (bool, error) {
	if len(cleaned) != 16 {
		return false, ErrNPWPInvalidLength
	}

	for i := 0; i < 16; i++ {
		if cleaned[i] < '0' || cleaned[i] > '9' {
			return false, ErrNPWPNonNumeric
		}
	}

	if cleaned[0] == '0' {
		// Corporate / foreign entity NPWP16: '0' prefix + 15-digit legacy NPWP
		return ValidateNPWP15(cleaned[1:])
	}

	// Individual NPWP16: 16-digit NIK
	nikRes := ValidateNIK(cleaned)
	if !nikRes.IsValid {
		return false, fmt.Errorf("npwp 16-digit nik validation failed: %s", nikRes.Error)
	}

	return true, nil
}

// ValidateNPWP validates either a 15-digit or 16-digit raw or formatted NPWP string.
// Returns valid boolean, cleaned numeric string, and error if invalid.
func ValidateNPWP(raw string) (bool, string, error) {
	cleaned := CleanNPWP(raw)

	switch len(cleaned) {
	case 15:
		valid, err := ValidateNPWP15(cleaned)
		if !valid {
			return false, cleaned, err
		}
		return true, cleaned, nil
	case 16:
		valid, err := ValidateNPWP16(cleaned)
		if !valid {
			return false, cleaned, err
		}
		return true, cleaned, nil
	default:
		return false, cleaned, fmt.Errorf("%w: got %d digits", ErrNPWPInvalidLength, len(cleaned))
	}
}

// FormatNPWP15 formats a 15-digit cleaned string into standard XX.XXX.XXX.X-XXX.XXX format.
func FormatNPWP15(cleaned string) string {
	if len(cleaned) != 15 {
		return cleaned
	}
	return fmt.Sprintf("%s.%s.%s.%s-%s.%s",
		cleaned[0:2],
		cleaned[2:5],
		cleaned[5:8],
		cleaned[8:9],
		cleaned[9:12],
		cleaned[12:15],
	)
}

// ValidateNPWPData verifies all structured fields extracted from an Indonesian NPWP document.
func ValidateNPWPData(data *NPWPData) (bool, map[string]string) {
	errs := make(map[string]string)
	if data == nil {
		errs["document"] = "npwp data is empty"
		return false, errs
	}

	// 1. NPWP Number
	if data.NPWP == "" {
		errs["npwp"] = "npwp number is required"
	} else if valid, _, err := ValidateNPWP(data.NPWP); !valid {
		errs["npwp"] = err.Error()
	}

	// 2. Nama Wajib Pajak
	cleanNama := strings.TrimSpace(data.Nama)
	if cleanNama == "" {
		errs["nama"] = "taxpayer name is required"
	} else if len(cleanNama) < 2 {
		errs["nama"] = "taxpayer name is too short"
	}

	// 3. Alamat
	if strings.TrimSpace(data.Alamat) == "" {
		errs["alamat"] = "taxpayer address is required"
	}

	// 4. KPP
	if strings.TrimSpace(data.KPP) == "" {
		errs["kpp"] = "tax office (kpp) is required"
	}

	// 5. NIK (if present)
	if data.NIK != "" {
		cleanNIK := CleanNPWP(data.NIK)
		nikRes := ValidateNIK(cleanNIK)
		if !nikRes.IsValid {
			errs["nik"] = fmt.Sprintf("invalid nik: %s", nikRes.Error)
		}
	}

	// 6. Tanggal Daftar (if present)
	if data.TanggalDaftar != "" {
		parsedDate, err := time.Parse("2006-01-02", data.TanggalDaftar)
		if err != nil {
			errs["tanggal_daftar"] = "invalid date format, must be YYYY-MM-DD"
		} else if parsedDate.After(time.Now().Add(24 * time.Hour)) {
			errs["tanggal_daftar"] = "registration date cannot be in the future"
		}
	}

	return len(errs) == 0, errs
}
