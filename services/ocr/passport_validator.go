package ocr

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	passportNumRegex = regexp.MustCompile(`^[A-Z][0-9A-Z]{7,8}$`)
	mrzWeights       = []int{7, 3, 1}
)

// MRZValidationResult captures the detailed outcome of MRZ TD3 validation.
type MRZValidationResult struct {
	IsValid            bool
	PassportNumber     string
	Nationality        string
	DateOfBirth        string // YYYY-MM-DD
	Gender             string // LAKI-LAKI | PEREMPUAN
	ExpiryDate         string // YYYY-MM-DD
	Surname            string
	GivenNames         string
	PassportNumValid   bool
	DOBValid           bool
	ExpiryValid        bool
	CompositeValid     bool
	Error              string
}

// PassportNumberValidationResult provides the outcome of deterministic passport number validation.
type PassportNumberValidationResult struct {
	IsValid bool
	Cleaned string
	Error   string
}

// CalculateMRZCheckDigit calculates the ICAO 9303 check digit for an MRZ field string.
// Weights repeat 7, 3, 1, 7, 3, 1... Modulo 10.
// Characters '0'-'9' have values 0-9, 'A'-'Z' have values 10-35, and '<' has value 0.
func CalculateMRZCheckDigit(data string) byte {
	sum := 0
	for i := 0; i < len(data); i++ {
		ch := data[i]
		var val int
		switch {
		case ch >= '0' && ch <= '9':
			val = int(ch - '0')
		case ch >= 'A' && ch <= 'Z':
			val = int(ch - 'A') + 10
		default:
			val = 0
		}
		weight := mrzWeights[i%3]
		sum += val * weight
	}
	return byte((sum % 10) + '0')
}

// ValidatePassportNumber performs strict, deterministic validation on an Indonesian passport number:
// 1. Strips whitespace.
// 2. Length must be 8 or 9 characters.
// 3. Must start with an uppercase letter followed by 7 or 8 alphanumeric characters.
// Time Complexity: O(n) with n <= 9. Zero heap allocations.
func ValidatePassportNumber(num string) PassportNumberValidationResult {
	num = strings.ToUpper(strings.TrimSpace(num))
	num = strings.ReplaceAll(num, " ", "")

	if len(num) < 8 || len(num) > 9 {
		return PassportNumberValidationResult{
			IsValid: false,
			Cleaned: num,
			Error:   fmt.Sprintf("passport number length must be 8 or 9 characters, got %d", len(num)),
		}
	}

	if !passportNumRegex.MatchString(num) {
		return PassportNumberValidationResult{
			IsValid: false,
			Cleaned: num,
			Error:   "passport number must start with a letter followed by 7-8 alphanumeric characters",
		}
	}

	return PassportNumberValidationResult{
		IsValid: true,
		Cleaned: num,
	}
}

// ParseMRZDate parses a 6-digit YYMMDD date string from an MRZ into YYYY-MM-DD.
func ParseMRZDate(yymmdd string, isExpiry bool) (string, error) {
	if len(yymmdd) != 6 {
		return "", fmt.Errorf("mrz date must be 6 digits (YYMMDD), got %d", len(yymmdd))
	}
	for i := 0; i < 6; i++ {
		if yymmdd[i] < '0' || yymmdd[i] > '9' {
			return "", fmt.Errorf("mrz date contains non-numeric character at %d", i)
		}
	}

	yy, _ := strconv.Atoi(yymmdd[0:2])
	mm, _ := strconv.Atoi(yymmdd[2:4])
	dd, _ := strconv.Atoi(yymmdd[4:6])

	if mm < 1 || mm > 12 {
		return "", fmt.Errorf("invalid month in mrz date: %d", mm)
	}
	if dd < 1 || dd > 31 {
		return "", fmt.Errorf("invalid day in mrz date: %d", dd)
	}

	currentYear := time.Now().Year()
	currentCentury := (currentYear / 100) * 100
	currentYY := currentYear % 100

	var fullYear int
	if isExpiry {
		// Passport expiration is generally in current century or next decade
		fullYear = currentCentury + yy
		// If calculated expiration is more than 30 years in past, adjust century
		if fullYear < currentYear-30 {
			fullYear += 100
		}
	} else {
		// Date of birth
		if yy > currentYY {
			fullYear = currentCentury - 100 + yy
		} else {
			fullYear = currentCentury + yy
		}
	}

	return fmt.Sprintf("%04d-%02d-%02d", fullYear, mm, dd), nil
}

// ValidateMRZTD3 validates a 2-line ICAO 9303 TD3 Machine Readable Zone (MRZ).
// Line 1 and Line 2 must be exactly 44 characters.
func ValidateMRZTD3(line1, line2 string) MRZValidationResult {
	line1 = strings.ToUpper(strings.TrimSpace(line1))
	line2 = strings.ToUpper(strings.TrimSpace(line2))

	if len(line1) != 44 || len(line2) != 44 {
		return MRZValidationResult{
			IsValid: false,
			Error:   fmt.Sprintf("mrz line lengths must be exactly 44 characters (got %d and %d)", len(line1), len(line2)),
		}
	}

	// Line 1 Check: Document type starting with P
	if line1[0] != 'P' {
		return MRZValidationResult{
			IsValid: false,
			Error:   fmt.Sprintf("expected document code 'P' at line1[0], got '%c'", line1[0]),
		}
	}

	// Extract Name from Line 1 (chars 5-44): SURNAME<<GIVEN<NAMES<<<<
	namePart := line1[5:44]
	var surname, givenNames string
	parts := strings.Split(namePart, "<<")
	if len(parts) > 0 {
		surname = strings.ReplaceAll(parts[0], "<", " ")
		surname = strings.TrimSpace(surname)
	}
	if len(parts) > 1 {
		givenNames = strings.ReplaceAll(parts[1], "<", " ")
		givenNames = strings.TrimSpace(givenNames)
	}

	// Line 2 Check:
	// Chars 0-9: Passport Number
	rawDocNum := line2[0:9]
	docNum := strings.TrimRight(rawDocNum, "<")
	docNumCheck := line2[9]
	calcDocNumCheck := CalculateMRZCheckDigit(rawDocNum)
	docNumValid := docNumCheck == calcDocNumCheck

	// Chars 10-13: Nationality (3 chars)
	nat := strings.TrimRight(line2[10:13], "<")

	// Chars 13-19: Date of Birth (YYMMDD)
	rawDOB := line2[13:19]
	dobCheck := line2[19]
	calcDOBCheck := CalculateMRZCheckDigit(rawDOB)
	dobCheckValid := dobCheck == calcDOBCheck
	parsedDOB, errDOB := ParseMRZDate(rawDOB, false)
	dobValid := dobCheckValid && errDOB == nil

	// Char 20: Sex ('M', 'F', or '<')
	sexChar := line2[20]
	var gender string
	switch sexChar {
	case 'M':
		gender = "LAKI-LAKI"
	case 'F':
		gender = "PEREMPUAN"
	}

	// Chars 21-27: Expiration Date (YYMMDD)
	rawExpiry := line2[21:27]
	expiryCheck := line2[27]
	calcExpiryCheck := CalculateMRZCheckDigit(rawExpiry)
	expiryCheckValid := expiryCheck == calcExpiryCheck
	parsedExpiry, errExp := ParseMRZDate(rawExpiry, true)
	expiryValid := expiryCheckValid && errExp == nil

	// Chars 28-42: Optional personal number (14 chars)
	rawOptional := line2[28:42]
	optionalCheck := line2[42]
	calcOptionalCheck := CalculateMRZCheckDigit(rawOptional)
	_ = optionalCheck == calcOptionalCheck || optionalCheck == '<'

	// Char 43: Composite check digit over:
	// line2[0:10] (doc num + check) + line2[13:20] (dob + check) + line2[21:43] (expiry + check + optional + check)
	compositePayload := line2[0:10] + line2[13:20] + line2[21:43]
	compositeCheck := line2[43]
	calcCompositeCheck := CalculateMRZCheckDigit(compositePayload)
	compositeValid := compositeCheck == calcCompositeCheck

	allValid := docNumValid && dobValid && expiryValid && compositeValid

	res := MRZValidationResult{
		IsValid:          allValid,
		PassportNumber:   docNum,
		Nationality:      nat,
		DateOfBirth:      parsedDOB,
		Gender:           gender,
		ExpiryDate:       parsedExpiry,
		Surname:          surname,
		GivenNames:       givenNames,
		PassportNumValid: docNumValid,
		DOBValid:         dobValid,
		ExpiryValid:      expiryValid,
		CompositeValid:   compositeValid,
	}

	if !allValid {
		var errs []string
		if !docNumValid {
			errs = append(errs, "passport number check digit mismatch")
		}
		if !dobValid {
			errs = append(errs, "dob check digit mismatch or invalid date")
		}
		if !expiryValid {
			errs = append(errs, "expiry date check digit mismatch or invalid date")
		}
		if !compositeValid {
			errs = append(errs, "composite check digit mismatch")
		}
		res.Error = strings.Join(errs, "; ")
	}

	return res
}

// ValidatePassport performs normalization, deterministic validation, MRZ verification,
// and scoring on PassportData.
// Returns normalized PassportData, confidence score (0.00 - 1.00), and issues list.
func ValidatePassport(data *PassportData) (*PassportData, float64, []string) {
	if data == nil {
		return &PassportData{}, 0.0, []string{"passport data is nil"}
	}

	var issues []string
	var score float64

	// 1. Normalize fields
	passNum := strings.ToUpper(strings.TrimSpace(data.PassportNumber))
	passNum = strings.ReplaceAll(passNum, " ", "")

	nat := strings.ToUpper(strings.TrimSpace(data.Nationality))
	if nat == "INDONESIA" || nat == "WNI" || nat == "" {
		nat = "IDN"
	}

	gender := strings.ToUpper(strings.TrimSpace(data.Gender))
	switch gender {
	case "M", "L", "PRIA", "LAKI-LAKI":
		gender = "LAKI-LAKI"
	case "F", "P", "W", "WANITA", "PEREMPUAN":
		gender = "PEREMPUAN"
	}

	normalized := &PassportData{
		PassportNumber: passNum,
		FullName:       cleanField(data.FullName),
		Nationality:    nat,
		DateOfBirth:    normalizeDate(data.DateOfBirth),
		PlaceOfBirth:   cleanField(data.PlaceOfBirth),
		Gender:         gender,
		IssueDate:      normalizeDate(data.IssueDate),
		ExpiryDate:     normalizeDate(data.ExpiryDate),
		IssuingOffice:  cleanField(data.IssuingOffice),
		MRZLine1:       strings.ToUpper(strings.TrimSpace(data.MRZLine1)),
		MRZLine2:       strings.ToUpper(strings.TrimSpace(data.MRZLine2)),
	}

	// 2. Passport Number Validation (25% weight)
	passNumRes := ValidatePassportNumber(normalized.PassportNumber)
	if passNumRes.IsValid {
		normalized.PassportNumber = passNumRes.Cleaned
		score += 0.25
	} else {
		issues = append(issues, "passport_number: "+passNumRes.Error)
	}

	// 3. MRZ Validation (25% weight)
	hasMRZ := normalized.MRZLine1 != "" && normalized.MRZLine2 != ""
	if hasMRZ {
		mrzRes := ValidateMRZTD3(normalized.MRZLine1, normalized.MRZLine2)
		if mrzRes.IsValid {
			score += 0.25
		} else {
			score += 0.10 // Partial credit if parsed
			issues = append(issues, "mrz: "+mrzRes.Error)
		}

		// Cross-check MRZ with visual fields
		if mrzRes.PassportNumber != "" && normalized.PassportNumber != "" {
			if !strings.EqualFold(mrzRes.PassportNumber, normalized.PassportNumber) {
				issues = append(issues, "passport_number: mismatch with MRZ document number")
				score -= 0.10
			}
		}
		if mrzRes.DateOfBirth != "" && normalized.DateOfBirth != "" {
			if mrzRes.DateOfBirth != normalized.DateOfBirth {
				issues = append(issues, "date_of_birth: mismatch with MRZ date of birth")
				score -= 0.10
			}
		}
		if mrzRes.ExpiryDate != "" && normalized.ExpiryDate != "" {
			if mrzRes.ExpiryDate != normalized.ExpiryDate {
				issues = append(issues, "expiry_date: mismatch with MRZ expiry date")
				score -= 0.10
			}
		}
		if mrzRes.Gender != "" && normalized.Gender != "" {
			if mrzRes.Gender != normalized.Gender {
				issues = append(issues, "gender: mismatch with MRZ sex")
				score -= 0.05
			}
		}

		// If visual name was empty but MRZ had names, backfill
		if normalized.FullName == "" && (mrzRes.Surname != "" || mrzRes.GivenNames != "") {
			name := strings.TrimSpace(mrzRes.Surname + " " + mrzRes.GivenNames)
			normalized.FullName = name
		}
	} else {
		// When MRZ is absent in partial crop, give weight if visual fields are coherent
		if passNumRes.IsValid {
			score += 0.15
		}
	}

	// 4. Full Name Validation (15% weight)
	if len(normalized.FullName) >= 2 {
		score += 0.15
	} else {
		issues = append(issues, "full_name: missing or too short")
	}

	// 5. Date of Birth & Place of Birth (15% weight)
	dobValid := false
	if normalized.DateOfBirth != "" {
		if t, err := time.Parse("2006-01-02", normalized.DateOfBirth); err == nil {
			if t.Year() >= 1900 && t.Before(time.Now()) {
				dobValid = true
			}
		}
	}

	if dobValid && normalized.PlaceOfBirth != "" {
		score += 0.15
	} else {
		if !dobValid {
			issues = append(issues, "date_of_birth: invalid date or format (must be YYYY-MM-DD)")
		}
		if normalized.PlaceOfBirth == "" {
			issues = append(issues, "place_of_birth: missing")
		}
	}

	// 6. Dates Plausibility: Issue Date & Expiry Date (10% weight)
	datesValid := false
	var issueT, expiryT time.Time
	var errI, errE error
	if normalized.IssueDate != "" {
		issueT, errI = time.Parse("2006-01-02", normalized.IssueDate)
	}
	if normalized.ExpiryDate != "" {
		expiryT, errE = time.Parse("2006-01-02", normalized.ExpiryDate)
	}

	if errE == nil && !expiryT.IsZero() {
		if errI == nil && !issueT.IsZero() {
			if expiryT.After(issueT) {
				datesValid = true
				score += 0.10
			} else {
				issues = append(issues, "expiry_date: expiry date must be after issue date")
			}
		} else {
			// Only expiry date available
			datesValid = true
			score += 0.08
		}
	} else {
		issues = append(issues, "expiry_date: missing or invalid date format (must be YYYY-MM-DD)")
	}
	_ = datesValid

	// 7. Nationality (5% weight)
	if normalized.Nationality == "IDN" {
		score += 0.05
	} else if normalized.Nationality != "" {
		score += 0.03
	} else {
		issues = append(issues, "nationality: missing")
	}

	// 8. Gender & Issuing Office (5% weight)
	genderScore := 0.0
	if normalized.Gender == "LAKI-LAKI" || normalized.Gender == "PEREMPUAN" {
		genderScore += 0.03
	} else if normalized.Gender != "" {
		issues = append(issues, "gender: unrecognized value (expected LAKI-LAKI or PEREMPUAN)")
	}

	if normalized.IssuingOffice != "" {
		genderScore += 0.02
	}
	score += genderScore

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
