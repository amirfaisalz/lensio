package ocr

import (
	"strings"
	"testing"
)

func TestCalculateMRZCheckDigit(t *testing.T) {
	// Sample: Document number "L898902C3" with check digit "6" from ICAO 9303 spec
	calc := CalculateMRZCheckDigit("L898902C3")
	if calc != '6' {
		t.Errorf("expected '6', got '%c'", calc)
	}

	// Test '<' character handling
	calcFiller := CalculateMRZCheckDigit("<<<<<<<<<")
	if calcFiller != '0' {
		t.Errorf("expected '0' for all fillers, got '%c'", calcFiller)
	}

	// Test digits only
	calcDigits := CalculateMRZCheckDigit("123456")
	// 1*7 + 2*3 + 3*1 + 4*7 + 5*3 + 6*1 = 7 + 6 + 3 + 28 + 15 + 6 = 65 -> 5
	if calcDigits != '5' {
		t.Errorf("expected '5', got '%c'", calcDigits)
	}
}

func TestValidatePassportNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		isValid bool
	}{
		{"valid 9 chars starting with letter", "A12345678", true},
		{"valid 8 chars starting with letter", "X1234567", true},
		{"valid with spaces trimmed", " C87654321 ", true},
		{"too short", "A12345", false},
		{"too long", "A1234567890", false},
		{"starts with digit", "12345678A", false},
		{"contains invalid special char", "A123-4567", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ValidatePassportNumber(tt.input)
			if res.IsValid != tt.isValid {
				t.Errorf("ValidatePassportNumber(%q) isValid=%v, want %v (error=%s)", tt.input, res.IsValid, tt.isValid, res.Error)
			}
		})
	}
}

func TestParseMRZDate(t *testing.T) {
	// Valid past birth date
	dob, err := ParseMRZDate("900115", false)
	if err != nil || dob != "1990-01-15" {
		t.Errorf("expected 1990-01-15, got %s (err=%v)", dob, err)
	}

	// Valid recent child birth date
	dobChild, err := ParseMRZDate("200210", false)
	if err != nil || dobChild != "2020-02-10" {
		t.Errorf("expected 2020-02-10, got %s (err=%v)", dobChild, err)
	}

	// Valid expiry date
	exp, err := ParseMRZDate("300520", true)
	if err != nil || exp != "2030-05-20" {
		t.Errorf("expected 2030-05-20, got %s (err=%v)", exp, err)
	}

	// Invalid lengths & formats
	if _, err := ParseMRZDate("9001", false); err == nil {
		t.Error("expected error for short date")
	}
	if _, err := ParseMRZDate("90011A", false); err == nil {
		t.Error("expected error for non-numeric date")
	}
	if _, err := ParseMRZDate("901301", false); err == nil {
		t.Error("expected error for month 13")
	}
	if _, err := ParseMRZDate("900001", false); err == nil {
		t.Error("expected error for month 00")
	}
	if _, err := ParseMRZDate("900135", false); err == nil {
		t.Error("expected error for day 35")
	}
	if _, err := ParseMRZDate("900100", false); err == nil {
		t.Error("expected error for day 00")
	}
}

func createValidTestMRZ() (string, string) {
	line1 := "P<IDNSANTOSO<<BUDI<<<<<<<<<<<<<<<<<<<<<<<<<<"
	// Line 2 parts:
	docNum := "X1234567<"
	docCheck := CalculateMRZCheckDigit(docNum)
	nat := "IDN"
	dob := "900101"
	dobCheck := CalculateMRZCheckDigit(dob)
	sex := "M"
	exp := "300101"
	expCheck := CalculateMRZCheckDigit(exp)
	opt := "<<<<<<<<<<<<<<"
	optCheck := byte('<')

	compPayload := docNum + string(docCheck) + dob + string(dobCheck) + exp + string(expCheck) + opt + string(optCheck)
	compCheck := CalculateMRZCheckDigit(compPayload)

	line2 := docNum + string(docCheck) + nat + dob + string(dobCheck) + sex + exp + string(expCheck) + opt + string(optCheck) + string(compCheck)
	return line1, line2
}

func TestValidateMRZTD3(t *testing.T) {
	l1, l2 := createValidTestMRZ()
	res := ValidateMRZTD3(l1, l2)
	if !res.IsValid {
		t.Fatalf("expected valid MRZ, got error: %s", res.Error)
	}
	if res.PassportNumber != "X1234567" {
		t.Errorf("expected passport number X1234567, got %s", res.PassportNumber)
	}
	if res.DateOfBirth != "1990-01-01" {
		t.Errorf("expected DOB 1990-01-01, got %s", res.DateOfBirth)
	}
	if res.ExpiryDate != "2030-01-01" {
		t.Errorf("expected expiry 2030-01-01, got %s", res.ExpiryDate)
	}
	if res.Gender != "LAKI-LAKI" {
		t.Errorf("expected gender LAKI-LAKI, got %s", res.Gender)
	}
	if res.Surname != "SANTOSO" || res.GivenNames != "BUDI" {
		t.Errorf("expected surname SANTOSO given BUDI, got %s / %s", res.Surname, res.GivenNames)
	}

	// Female test
	l1F := "P<IDNLESTARI<<SITI<<<<<<<<<<<<<<<<<<<<<<<<<<"
	l2F := strings.Replace(l2, "M", "F", 1)
	// recompute composite check
	compPayloadF := l2F[0:10] + l2F[13:20] + l2F[21:43]
	compCheckF := CalculateMRZCheckDigit(compPayloadF)
	l2F = l2F[0:43] + string(compCheckF)
	resF := ValidateMRZTD3(l1F, l2F)
	if !resF.IsValid || resF.Gender != "PEREMPUAN" {
		t.Errorf("expected valid female MRZ with PEREMPUAN, got isValid=%v gender=%s error=%s", resF.IsValid, resF.Gender, resF.Error)
	}

	// Invalid line lengths
	resLen := ValidateMRZTD3("P<IDNSHORT", l2)
	if resLen.IsValid || !strings.Contains(resLen.Error, "lengths must be exactly 44") {
		t.Errorf("expected line length error, got %v", resLen.Error)
	}

	// Invalid line 1 prefix
	resPrefix := ValidateMRZTD3("V<IDNSANTOSO<<BUDI<<<<<<<<<<<<<<<<<<<<<<<<<<", l2)
	if resPrefix.IsValid || !strings.Contains(resPrefix.Error, "document code 'P'") {
		t.Errorf("expected prefix error, got %v", resPrefix.Error)
	}

	// Tampered line 2 doc number check digit
	corruptDoc := l2[0:9] + "0" + l2[10:]
	resTamperDoc := ValidateMRZTD3(l1, corruptDoc)
	if resTamperDoc.IsValid || !strings.Contains(resTamperDoc.Error, "passport number check digit mismatch") {
		t.Errorf("expected doc check mismatch, got %v", resTamperDoc.Error)
	}

	// Tampered DOB check digit
	corruptDOB := l2[0:19] + "0" + l2[20:]
	resTamperDOB := ValidateMRZTD3(l1, corruptDOB)
	if resTamperDOB.IsValid || !strings.Contains(resTamperDOB.Error, "dob check digit mismatch") {
		t.Errorf("expected dob check mismatch, got %v", resTamperDOB.Error)
	}

	// Tampered Expiry check digit
	corruptExp := l2[0:27] + "0" + l2[28:]
	resTamperExp := ValidateMRZTD3(l1, corruptExp)
	if resTamperExp.IsValid || !strings.Contains(resTamperExp.Error, "expiry date check digit mismatch") {
		t.Errorf("expected expiry check mismatch, got %v", resTamperExp.Error)
	}

	// Tampered composite check digit
	corruptComp := l2[0:43] + "0"
	resTamperComp := ValidateMRZTD3(l1, corruptComp)
	if resTamperComp.IsValid || !strings.Contains(resTamperComp.Error, "composite check digit mismatch") {
		t.Errorf("expected composite check mismatch, got %v", resTamperComp.Error)
	}
}

func TestValidatePassport(t *testing.T) {
	l1, l2 := createValidTestMRZ()

	validPassport := &PassportData{
		PassportNumber: "X1234567",
		FullName:       "BUDI SANTOSO",
		Nationality:    "IDN",
		DateOfBirth:    "1990-01-01",
		PlaceOfBirth:   "JAKARTA",
		Gender:         "LAKI-LAKI",
		IssueDate:      "2020-01-01",
		ExpiryDate:     "2030-01-01",
		IssuingOffice:  "JAKARTA SELATAN",
		MRZLine1:       l1,
		MRZLine2:       l2,
	}

	t.Run("nil passport data", func(t *testing.T) {
		p, score, issues := ValidatePassport(nil)
		if p == nil || score != 0.0 || len(issues) == 0 {
			t.Errorf("expected empty data and 0.0 score for nil input")
		}
	})

	t.Run("fully valid passport with MRZ", func(t *testing.T) {
		p, score, issues := ValidatePassport(validPassport)
		if len(issues) > 0 {
			t.Errorf("expected no issues, got: %v", issues)
		}
		if score < 0.95 {
			t.Errorf("expected high confidence score >= 0.95, got %f", score)
		}
		if p.PassportNumber != "X1234567" {
			t.Errorf("unexpected passport number %s", p.PassportNumber)
		}
	})

	t.Run("backfill name from MRZ when visual full name is empty", func(t *testing.T) {
		cp := *validPassport
		cp.FullName = ""
		p, score, _ := ValidatePassport(&cp)
		if p.FullName != "SANTOSO BUDI" {
			t.Errorf("expected name to backfill from MRZ 'SANTOSO BUDI', got %q", p.FullName)
		}
		if score < 0.90 {
			t.Errorf("expected high score with backfilled name, got %f", score)
		}
	})

	t.Run("valid passport without MRZ (partial crop)", func(t *testing.T) {
		cp := *validPassport
		cp.MRZLine1 = ""
		cp.MRZLine2 = ""
		_, score, issues := ValidatePassport(&cp)
		if len(issues) > 0 {
			t.Errorf("expected no fatal issues without MRZ, got: %v", issues)
		}
		if score < 0.80 {
			t.Errorf("expected reasonable score without MRZ, got %f", score)
		}
	})

	t.Run("mismatched visual and MRZ fields", func(t *testing.T) {
		cp := *validPassport
		cp.PassportNumber = "A9999999"
		cp.DateOfBirth = "1995-05-05"
		cp.ExpiryDate = "2035-05-05"
		cp.Gender = "PEREMPUAN"
		_, score, issues := ValidatePassport(&cp)
		if len(issues) < 4 {
			t.Errorf("expected at least 4 mismatch issues, got %v", issues)
		}
		if score >= 0.90 {
			t.Errorf("expected score to be reduced for mismatches, got %f", score)
		}
	})

	t.Run("invalid dates: expiry before issue date", func(t *testing.T) {
		cp := *validPassport
		cp.IssueDate = "2025-01-01"
		cp.ExpiryDate = "2020-01-01"
		cp.MRZLine1 = ""
		cp.MRZLine2 = ""
		_, _, issues := ValidatePassport(&cp)
		found := false
		for _, iss := range issues {
			if strings.Contains(iss, "expiry date must be after issue date") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected expiry date after issue date issue, got: %v", issues)
		}
	})

	t.Run("normalizes various aliases for gender and nationality", func(t *testing.T) {
		cp := *validPassport
		cp.Nationality = "INDONESIA"
		cp.Gender = "PRIA"
		p, _, _ := ValidatePassport(&cp)
		if p.Nationality != "IDN" {
			t.Errorf("expected nationality IDN, got %s", p.Nationality)
		}
		if p.Gender != "LAKI-LAKI" {
			t.Errorf("expected gender LAKI-LAKI, got %s", p.Gender)
		}
	})

	t.Run("missing required fields", func(t *testing.T) {
		empty := &PassportData{}
		_, score, issues := ValidatePassport(empty)
		if score > 0.15 {
			t.Errorf("expected low score for empty data, got %f", score)
		}
		if len(issues) < 4 {
			t.Errorf("expected multiple missing field issues, got %v", issues)
		}
	})

	t.Run("unrecognized gender and foreign nationality", func(t *testing.T) {
		p := &PassportData{
			PassportNumber: "A12345678",
			FullName:       "JOHN DOE",
			Nationality:    "USA",
			DateOfBirth:    "1985-05-15",
			PlaceOfBirth:   "NEW YORK",
			Gender:         "UNKNOWN",
			ExpiryDate:     "2030-05-15",
			IssuingOffice:  "EMBASSY",
		}
		_, score, issues := ValidatePassport(p)
		if score <= 0.50 {
			t.Errorf("expected reasonable score for valid doc, got %f", score)
		}
		hasGenderIssue := false
		for _, iss := range issues {
			if strings.Contains(iss, "gender: unrecognized value") {
				hasGenderIssue = true
			}
		}
		if !hasGenderIssue {
			t.Errorf("expected unrecognized gender issue, got %v", issues)
		}
	})

	t.Run("female gender and empty nationality", func(t *testing.T) {
		p := &PassportData{
			PassportNumber: "B87654321",
			FullName:       "JANE DOE",
			Nationality:    "",
			DateOfBirth:    "1992-08-20",
			PlaceOfBirth:   "BANDUNG",
			Gender:         "WANITA",
			ExpiryDate:     "2032-08-20",
		}
		norm, score, _ := ValidatePassport(p)
		if norm.Nationality != "IDN" {
			t.Errorf("expected default IDN nationality, got %s", norm.Nationality)
		}
		if norm.Gender != "PEREMPUAN" {
			t.Errorf("expected PEREMPUAN gender, got %s", norm.Gender)
		}
		if score < 0.60 {
			t.Errorf("expected positive score, got %f", score)
		}
	})

	t.Run("only issue date available or invalid expiry date", func(t *testing.T) {
		p := &PassportData{
			PassportNumber: "C12345678",
			FullName:       "TEST PERSON",
			Nationality:    "IDN",
			DateOfBirth:    "invalid-date",
			PlaceOfBirth:   "",
			Gender:         "PEREMPUAN",
			IssueDate:      "2020-01-01",
			ExpiryDate:     "invalid-expiry",
		}
		_, _, issues := ValidatePassport(p)
		hasExpiryIssue := false
		for _, iss := range issues {
			if strings.Contains(iss, "expiry_date:") {
				hasExpiryIssue = true
			}
		}
		if !hasExpiryIssue {
			t.Errorf("expected expiry issue, got %v", issues)
		}
	})
}

func BenchmarkValidatePassportNumber(b *testing.B) {
	num := "A12345678"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidatePassportNumber(num)
	}
}

func BenchmarkCalculateMRZCheckDigit(b *testing.B) {
	data := "X1234567<"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateMRZCheckDigit(data)
	}
}

func BenchmarkValidateMRZTD3(b *testing.B) {
	l1, l2 := createValidTestMRZ()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateMRZTD3(l1, l2)
	}
}

func BenchmarkValidatePassport(b *testing.B) {
	l1, l2 := createValidTestMRZ()
	data := &PassportData{
		PassportNumber: "X1234567",
		FullName:       "BUDI SANTOSO",
		Nationality:    "IDN",
		DateOfBirth:    "1990-01-01",
		PlaceOfBirth:   "JAKARTA",
		Gender:         "LAKI-LAKI",
		IssueDate:      "2020-01-01",
		ExpiryDate:     "2030-01-01",
		IssuingOffice:  "JAKARTA SELATAN",
		MRZLine1:       l1,
		MRZLine2:       l2,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ValidatePassport(data)
	}
}
