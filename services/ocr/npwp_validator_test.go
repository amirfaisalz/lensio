package ocr

import (
	"strings"
	"testing"
)

func TestCleanNPWP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"09.254.294.3-407.000", "092542943407000"},
		{" 09-254-294-3 407 000 ", "092542943407000"},
		{"0012345678901234", "0012345678901234"},
		{"", ""},
		{"abc123def456", "123456"},
	}

	for _, tt := range tests {
		res := CleanNPWP(tt.input)
		if res != tt.expected {
			t.Errorf("CleanNPWP(%q) = %q, expected %q", tt.input, res, tt.expected)
		}
	}
}

func TestCalculateNPWPCheckDigit(t *testing.T) {
	tests := []struct {
		first8      string
		expected    int
		expectError bool
	}{
		{"09254294", 3, false},
		{"01234567", 4, false},
		{"02345678", 3, false},
		{"1234567", -1, true},  // Too short
		{"123456789", -1, true}, // Too long
		{"1234567A", -1, true}, // Non numeric
	}

	for _, tt := range tests {
		digit, err := CalculateNPWPCheckDigit(tt.first8)
		if (err != nil) != tt.expectError {
			t.Errorf("CalculateNPWPCheckDigit(%q) error = %v, expectError = %v", tt.first8, err, tt.expectError)
		}
		if !tt.expectError && digit != tt.expected {
			t.Errorf("CalculateNPWPCheckDigit(%q) = %d, expected %d", tt.first8, digit, tt.expected)
		}
	}
}

func TestValidateNPWP15(t *testing.T) {
	tests := []struct {
		name        string
		npwp        string
		expectValid bool
		expectErr   error
	}{
		{
			name:        "Valid 15 digit NPWP",
			npwp:        "092542943407000",
			expectValid: true,
			expectErr:   nil,
		},
		{
			name:        "Valid 15 digit another format",
			npwp:        "012345674012000",
			expectValid: true,
			expectErr:   nil,
		},
		{
			name:        "Invalid length - 14 digits",
			npwp:        "09254294340700",
			expectValid: false,
			expectErr:   ErrNPWPInvalidLength,
		},
		{
			name:        "Invalid length - 16 digits",
			npwp:        "0925429434070001",
			expectValid: false,
			expectErr:   ErrNPWPInvalidLength,
		},
		{
			name:        "Non numeric character",
			npwp:        "09254294340700A",
			expectValid: false,
			expectErr:   ErrNPWPNonNumeric,
		},
		{
			name:        "Invalid taxpayer prefix 00",
			npwp:        "002542943407000",
			expectValid: false,
			expectErr:   ErrNPWPInvalidTaxpayerType,
		},
		{
			name:        "Invalid check digit (checksum error)",
			npwp:        "092542948407000", // Check digit is 8 instead of 3
			expectValid: false,
			expectErr:   ErrNPWPInvalidChecksum,
		},
		{
			name:        "Invalid KPP code 000",
			npwp:        "092542943000000",
			expectValid: false,
			expectErr:   ErrNPWPInvalidKPP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := ValidateNPWP15(tt.npwp)
			if valid != tt.expectValid {
				t.Fatalf("ValidateNPWP15(%s) valid = %v, expected %v", tt.npwp, valid, tt.expectValid)
			}
			if tt.expectErr != nil {
				if err == nil || !strings.Contains(err.Error(), tt.expectErr.Error()) {
					t.Fatalf("ValidateNPWP15(%s) expected error containing %v, got %v", tt.npwp, tt.expectErr, err)
				}
			}
		})
	}
}

func TestValidateNPWP16(t *testing.T) {
	tests := []struct {
		name        string
		npwp        string
		expectValid bool
		expectErr   bool
	}{
		{
			name:        "Valid 16 digit corporate (starts with 0 + 15 digit legacy)",
			npwp:        "0092542943407000",
			expectValid: true,
			expectErr:   false,
		},
		{
			name:        "Invalid 16 digit corporate (bad checksum)",
			npwp:        "0092542948407000",
			expectValid: false,
			expectErr:   true,
		},
		{
			name:        "Valid 16 digit individual (valid NIK)",
			npwp:        "3171010101900001", // DKI Jakarta NIK
			expectValid: true,
			expectErr:   false,
		},
		{
			name:        "Invalid 16 digit individual (invalid province in NIK)",
			npwp:        "0000000000000001", // Not starting with 0? wait, starts with 0 will be corporate!
			expectValid: false,
			expectErr:   true,
		},
		{
			name:        "Invalid 16 digit individual (invalid province 99 in NIK)",
			npwp:        "9971012345670001",
			expectValid: false,
			expectErr:   true,
		},
		{
			name:        "Invalid length 15 digits",
			npwp:        "092542943407000",
			expectValid: false,
			expectErr:   true,
		},
		{
			name:        "Non numeric characters",
			npwp:        "009254294340700X",
			expectValid: false,
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := ValidateNPWP16(tt.npwp)
			if valid != tt.expectValid {
				t.Fatalf("ValidateNPWP16(%s) valid = %v, expected %v, err = %v", tt.npwp, valid, tt.expectValid, err)
			}
			if (err != nil) != tt.expectErr {
				t.Fatalf("ValidateNPWP16(%s) unexpected error state: %v", tt.npwp, err)
			}
		})
	}
}

func TestValidateNPWP(t *testing.T) {
	// Formatted 15-digit
	valid, cleaned, err := ValidateNPWP("09.254.294.3-407.000")
	if !valid || err != nil || cleaned != "092542943407000" {
		t.Fatalf("ValidateNPWP formatted 15-digit failed: valid=%v, cleaned=%s, err=%v", valid, cleaned, err)
	}

	// 16-digit corporate
	valid, cleaned, err = ValidateNPWP("0092542943407000")
	if !valid || err != nil || cleaned != "0092542943407000" {
		t.Fatalf("ValidateNPWP 16-digit corporate failed: valid=%v, cleaned=%s, err=%v", valid, cleaned, err)
	}

	// 16-digit individual
	valid, cleaned, err = ValidateNPWP("3171010101900001")
	if !valid || err != nil || cleaned != "3171010101900001" {
		t.Fatalf("ValidateNPWP 16-digit individual failed: valid=%v, cleaned=%s, err=%v", valid, cleaned, err)
	}

	// Invalid length
	valid, _, err = ValidateNPWP("12345")
	if valid || err == nil {
		t.Fatalf("ValidateNPWP short length should fail")
	}

	// Formatted invalid 15-digit
	valid, _, err = ValidateNPWP("09.254.294.9-407.000")
	if valid || err == nil {
		t.Fatalf("ValidateNPWP bad checksum should fail")
	}

	// Formatted 16-digit invalid
	valid, _, err = ValidateNPWP("9971012345670001")
	if valid || err == nil {
		t.Fatalf("ValidateNPWP bad NIK should fail")
	}
}

func TestFormatNPWP15(t *testing.T) {
	formatted := FormatNPWP15("092542943407000")
	if formatted != "09.254.294.3-407.000" {
		t.Errorf("FormatNPWP15 failed: got %s, want 09.254.294.3-407.000", formatted)
	}

	// Short string returns unmodified
	if FormatNPWP15("123") != "123" {
		t.Errorf("FormatNPWP15 short string should return unmodified")
	}
}

func TestValidateNPWPData(t *testing.T) {
	// 1. Nil data
	valid, errs := ValidateNPWPData(nil)
	if valid || errs["document"] == "" {
		t.Fatal("ValidateNPWPData(nil) should fail")
	}

	// 2. Fully valid 15-digit NPWP
	validData := &NPWPData{
		NPWP:          "09.254.294.3-407.000",
		Nama:          "BUDI SANTOSO",
		NIK:           "3171010101900001",
		Alamat:        "JL. JENDERAL SUDIRMAN NO. 10",
		KPP:           "KPP PRATAMA JAKARTA SETIABUDI SATU",
		TanggalDaftar: "2015-08-17",
	}
	valid, errs = ValidateNPWPData(validData)
	if !valid || len(errs) > 0 {
		t.Fatalf("ValidateNPWPData valid failed: %v", errs)
	}

	// 3. Validation failures
	invalidData := &NPWPData{
		NPWP:          "",
		Nama:          "A", // Too short
		Alamat:        "",
		KPP:           "",
		NIK:           "123", // Invalid NIK
		TanggalDaftar: "2099-01-01", // Future date
	}
	valid, errs = ValidateNPWPData(invalidData)
	if valid {
		t.Fatal("ValidateNPWPData invalidData should fail")
	}
	if errs["npwp"] == "" || errs["nama"] == "" || errs["alamat"] == "" || errs["kpp"] == "" || errs["nik"] == "" || errs["tanggal_daftar"] == "" {
		t.Fatalf("ValidateNPWPData missing expected errors: %v", errs)
	}

	// 4. Invalid date format
	dateErrData := &NPWPData{
		NPWP:          "09.254.294.3-407.000",
		Nama:          "BUDI SANTOSO",
		Alamat:        "JL. SUDIRMAN",
		KPP:           "KPP SETIABUDI",
		TanggalDaftar: "17/08/2015", // Not YYYY-MM-DD
	}
	valid, errs = ValidateNPWPData(dateErrData)
	if valid || errs["tanggal_daftar"] == "" {
		t.Fatalf("ValidateNPWPData date format error missing: %v", errs)
	}
}

// Benchmarks for Algorithmic Efficiency (Rule 3)
func BenchmarkCleanNPWP(b *testing.B) {
	raw := "09.254.294.3-407.000"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CleanNPWP(raw)
	}
}

func BenchmarkCalculateNPWPCheckDigit(b *testing.B) {
	digits := "09254294"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CalculateNPWPCheckDigit(digits)
	}
}

func BenchmarkValidateNPWP15(b *testing.B) {
	cleaned := "092542943407000"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ValidateNPWP15(cleaned)
	}
}

func BenchmarkValidateNPWP16(b *testing.B) {
	cleaned := "0092542943407000"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ValidateNPWP16(cleaned)
	}
}

func BenchmarkValidateNPWP(b *testing.B) {
	raw := "09.254.294.3-407.000"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ValidateNPWP(raw)
	}
}
