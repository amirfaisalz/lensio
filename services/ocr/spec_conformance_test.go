package ocr_test

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/amirfaisalz/lensio/services/ocr"
)

// These tests check the validators against the published specifications rather
// than against our own fixtures. A fixture only proves we are self-consistent;
// a specimen from ICAO or a rule from Permendagri proves we are correct.
//
// All identity numbers here are synthetic.

// ---------------------------------------------------------------------------
// MRZ — ICAO Doc 9303 Part 3 (check digits) and Part 4 (TD3 layout)
// ---------------------------------------------------------------------------

// The specimen printed in ICAO Doc 9303 Part 4 (Anna Maria Eriksson, Utopia).
const (
	icaoSpecimenLine1 = "P<UTOERIKSSON<<ANNA<MARIA<<<<<<<<<<<<<<<<<<<"
	icaoSpecimenLine2 = "L898902C36UTO7408122F1204159ZE184226B<<<<<10"
)

func TestMRZCheckDigit_ICAOPublishedVectors(t *testing.T) {
	// Worked examples from ICAO 9303 Part 3, Section 4.9.
	cases := []struct {
		field string
		want  byte
	}{
		{"L898902C3", '6'},      // document number
		{"740812", '2'},         // date of birth
		{"120415", '9'},         // date of expiry
		{"ZE184226B<<<<<", '1'}, // optional personal number
		{"<<<<<<<<<<<<<<", '0'}, // all-filler field sums to zero
		{"AB2134<<<", '5'},      // letters weighted A=10..Z=35
	}

	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			if got := ocr.CalculateMRZCheckDigit(tc.field); got != tc.want {
				t.Errorf("CalculateMRZCheckDigit(%q) = %c, want %c", tc.field, got, tc.want)
			}
		})
	}
}

func TestMRZTD3_ICAOSpecimenValidatesFully(t *testing.T) {
	res := ocr.ValidateMRZTD3(icaoSpecimenLine1, icaoSpecimenLine2)

	if !res.IsValid {
		t.Fatalf("the ICAO specimen must validate; got error: %s", res.Error)
	}
	for name, ok := range map[string]bool{
		"passport number": res.PassportNumValid,
		"date of birth":   res.DOBValid,
		"expiry":          res.ExpiryValid,
		"composite":       res.CompositeValid,
		"optional":        res.OptionalValid,
		"sex":             res.SexValid,
	} {
		if !ok {
			t.Errorf("%s check failed on the ICAO specimen", name)
		}
	}

	if res.PassportNumber != "L898902C3" {
		t.Errorf("passport number = %q, want L898902C3", res.PassportNumber)
	}
	if res.Nationality != "UTO" {
		t.Errorf("nationality = %q, want UTO", res.Nationality)
	}
	if res.DateOfBirth != "1974-08-12" {
		t.Errorf("date of birth = %q, want 1974-08-12", res.DateOfBirth)
	}
	if res.ExpiryDate != "2012-04-15" {
		t.Errorf("expiry = %q, want 2012-04-15", res.ExpiryDate)
	}
	if res.Gender != "PEREMPUAN" {
		t.Errorf("gender = %q, want PEREMPUAN (sex char F)", res.Gender)
	}
	if res.Surname != "ERIKSSON" || res.GivenNames != "ANNA MARIA" {
		t.Errorf("name = %q / %q, want ERIKSSON / ANNA MARIA", res.Surname, res.GivenNames)
	}
}

// Every check digit in the specimen must be load-bearing: corrupting any single
// one has to fail the line. This is what proves the checks are actually wired in
// rather than computed and discarded.
func TestMRZTD3_EachCheckDigitIsLoadBearing(t *testing.T) {
	cases := []struct {
		name  string
		index int
		field func(ocr.MRZValidationResult) bool
	}{
		{"document number", 9, func(r ocr.MRZValidationResult) bool { return r.PassportNumValid }},
		{"date of birth", 19, func(r ocr.MRZValidationResult) bool { return r.DOBValid }},
		{"expiry", 27, func(r ocr.MRZValidationResult) bool { return r.ExpiryValid }},
		{"optional personal number", 42, func(r ocr.MRZValidationResult) bool { return r.OptionalValid }},
		{"composite", 43, func(r ocr.MRZValidationResult) bool { return r.CompositeValid }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			corrupted := []byte(icaoSpecimenLine2)
			// Shift the digit by one so it can no longer be correct.
			if corrupted[tc.index] == '9' {
				corrupted[tc.index] = '0'
			} else if corrupted[tc.index] >= '0' && corrupted[tc.index] <= '8' {
				corrupted[tc.index]++
			} else {
				corrupted[tc.index] = '0'
			}

			res := ocr.ValidateMRZTD3(icaoSpecimenLine1, string(corrupted))
			if res.IsValid {
				t.Fatalf("corrupting the %s check digit must invalidate the MRZ", tc.name)
			}
			if tc.field(res) {
				t.Errorf("the %s sub-check still reported valid", tc.name)
			}
		})
	}
}

func TestMRZTD3_SexCharacterMustBeMFOrFiller(t *testing.T) {
	for _, sex := range []byte{'X', 'Q', '1', ' '} {
		corrupted := []byte(icaoSpecimenLine2)
		corrupted[20] = sex
		res := ocr.ValidateMRZTD3(icaoSpecimenLine1, string(corrupted))
		if res.SexValid {
			t.Errorf("sex character %q must be rejected (ICAO allows only M, F, <)", string(sex))
		}
	}

	// '<' means unspecified and is legal, but it changes the composite digit,
	// so check the sub-result rather than overall validity.
	corrupted := []byte(icaoSpecimenLine2)
	corrupted[20] = '<'
	if res := ocr.ValidateMRZTD3(icaoSpecimenLine1, string(corrupted)); !res.SexValid {
		t.Error("'<' (unspecified) must be accepted as a sex character")
	}
}

func TestMRZTD3_RejectsMalformedLines(t *testing.T) {
	cases := []struct{ name, l1, l2 string }{
		{"short line 1", "P<UTO", icaoSpecimenLine2},
		{"short line 2", icaoSpecimenLine1, "L898902C36UTO"},
		{"document code not P", "X" + icaoSpecimenLine1[1:], icaoSpecimenLine2},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if res := ocr.ValidateMRZTD3(tc.l1, tc.l2); res.IsValid {
				t.Error("expected malformed MRZ to be rejected")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NIK — Permendagri 109/2019 layout: PP KK CC DDMMYY SSSS
// ---------------------------------------------------------------------------

func TestValidateNIK_StructuralRules(t *testing.T) {
	// 32 = Jawa Barat, regency 01, district 02, born 17 Aug 1990, sequence 0001.
	const maleJabar = "3201021708900001"
	// Same person encoded as female: birth day + 40 => 17 becomes 57.
	const femaleJabar = "3201025708900001"

	t.Run("valid male NIK", func(t *testing.T) {
		res := ocr.ValidateNIK(maleJabar)
		if !res.IsValid {
			t.Fatalf("expected valid, got %q", res.Error)
		}
		if res.IsFemale {
			t.Error("day 17 must not be read as female")
		}
		if res.BirthDay != 17 || res.BirthMonth != 8 || res.BirthYear2D != 90 {
			t.Errorf("birth parsed as %02d-%02d-%02d, want 17-08-90", res.BirthDay, res.BirthMonth, res.BirthYear2D)
		}
		if res.ProvinceCode != "32" || res.RegencyCode != "01" || res.DistrictCode != "02" {
			t.Errorf("admin codes = %s/%s/%s, want 32/01/02", res.ProvinceCode, res.RegencyCode, res.DistrictCode)
		}
		if res.Sequence != 1 {
			t.Errorf("sequence = %d, want 1", res.Sequence)
		}
	})

	t.Run("female encoding subtracts 40 from the day", func(t *testing.T) {
		res := ocr.ValidateNIK(femaleJabar)
		if !res.IsValid {
			t.Fatalf("expected valid, got %q", res.Error)
		}
		if !res.IsFemale {
			t.Error("day 57 must be decoded as female")
		}
		if res.BirthDay != 17 {
			t.Errorf("decoded day = %d, want 17", res.BirthDay)
		}
	})

	invalid := []struct{ name, nik string }{
		{"15 digits", "320102170890001"},
		{"17 digits", "32010217089000012"},
		{"contains a letter", "32010217089O0001"},
		{"unknown province 00", "0001021708900001"},
		{"unknown province 99", "9901021708900001"},
		{"regency code 00", "3200021708900001"},
		{"district code 00", "3201001708900001"},
		{"sequence 0000", "3201021708900000"},
		{"day 00", "3201020008900001"},
		{"day 32 is neither a day nor day+40", "3201023208900001"},
		{"day 40 is neither a day nor day+40", "3201024008900001"},
		{"day 72 decodes to 32", "3201027208900001"},
		{"month 00", "3201021700900001"},
		{"month 13", "3201021713900001"},
		{"31 April", "3201023104900001"},
		{"30 February", "3201023002900001"},
	}
	for _, tc := range invalid {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			if res := ocr.ValidateNIK(tc.nik); res.IsValid {
				t.Errorf("expected %s to be rejected", tc.nik)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NPWP — Luhn over the first 8 digits; 16-digit forms per PMK 112/2022
// ---------------------------------------------------------------------------

func TestNPWPCheckDigit_IsLuhnOverFirstEightDigits(t *testing.T) {
	// Independently recompute Luhn to prove the implementation, rather than
	// asserting against a number the implementation itself produced.
	luhn := func(payload string) int {
		sum := 0
		for i := 0; i < len(payload); i++ {
			d := int(payload[i] - '0')
			if i%2 == 1 { // positions 2,4,6,8 from the left are doubled
				d *= 2
				if d > 9 {
					d -= 9
				}
			}
			sum += d
		}
		return (10 - sum%10) % 10
	}

	for _, payload := range []string{"01234567", "98765432", "11111111", "70123456", "00000001"} {
		got, err := ocr.CalculateNPWPCheckDigit(payload)
		if err != nil {
			t.Fatalf("CalculateNPWPCheckDigit(%q): %v", payload, err)
		}
		if want := luhn(payload); got != want {
			t.Errorf("check digit for %q = %d, want %d", payload, got, want)
		}
	}
}

// buildNPWP15 assembles a structurally valid 15-digit NPWP with a correct check digit.
func buildNPWP15(t *testing.T, first8, kpp, branch string) string {
	t.Helper()
	cd, err := ocr.CalculateNPWPCheckDigit(first8)
	if err != nil {
		t.Fatalf("check digit: %v", err)
	}
	return fmt.Sprintf("%s%d%s%s", first8, cd, kpp, branch)
}

func TestValidateNPWP_LengthsAndStructure(t *testing.T) {
	valid15 := buildNPWP15(t, "01234567", "012", "000")

	t.Run("accepts a well formed 15 digit NPWP", func(t *testing.T) {
		ok, cleaned, err := ocr.ValidateNPWP(valid15)
		if !ok {
			t.Fatalf("expected valid, got %v", err)
		}
		if cleaned != valid15 {
			t.Errorf("cleaned = %q, want %q", cleaned, valid15)
		}
	})

	t.Run("accepts the formatted presentation form", func(t *testing.T) {
		formatted := fmt.Sprintf("%s.%s.%s.%s-%s.%s",
			valid15[0:2], valid15[2:5], valid15[5:8], valid15[8:9], valid15[9:12], valid15[12:15])
		if ok, _, err := ocr.ValidateNPWP(formatted); !ok {
			t.Fatalf("punctuation must be stripped before validation, got %v", err)
		}
	})

	t.Run("rejects a corrupted check digit", func(t *testing.T) {
		corrupted := []byte(valid15)
		corrupted[8] = '0' + byte((int(corrupted[8]-'0')+1)%10)
		if ok, _, _ := ocr.ValidateNPWP(string(corrupted)); ok {
			t.Error("a wrong check digit must be rejected")
		}
	})

	t.Run("rejects taxpayer type 00", func(t *testing.T) {
		if ok, _, _ := ocr.ValidateNPWP(buildNPWP15(t, "00234567", "012", "000")); ok {
			t.Error("taxpayer category 00 must be rejected")
		}
	})

	t.Run("rejects KPP 000", func(t *testing.T) {
		if ok, _, _ := ocr.ValidateNPWP(buildNPWP15(t, "01234567", "000", "000")); ok {
			t.Error("KPP code 000 must be rejected")
		}
	})

	for _, bad := range []string{"", "123", "12345678901234", "1234567890123456789"} {
		t.Run("rejects length "+fmt.Sprint(len(bad)), func(t *testing.T) {
			ok, _, err := ocr.ValidateNPWP(bad)
			if ok {
				t.Error("expected rejection")
			}
			if err == nil {
				t.Error("expected an error explaining the rejection")
			}
		})
	}
}

// PMK 112/2022: individuals use their 16-digit NIK as NPWP; entities use a
// leading 0 followed by the legacy 15-digit number.
func TestValidateNPWP16_PMK112Forms(t *testing.T) {
	t.Run("entity form is 0 + valid 15 digit npwp", func(t *testing.T) {
		entity := "0" + buildNPWP15(t, "01234567", "012", "000")
		if ok, _, err := ocr.ValidateNPWP(entity); !ok {
			t.Fatalf("expected valid entity NPWP16, got %v", err)
		}
	})

	t.Run("entity form rejects an invalid embedded npwp", func(t *testing.T) {
		bad := "0" + buildNPWP15(t, "01234567", "000", "000") // KPP 000
		if ok, _, _ := ocr.ValidateNPWP(bad); ok {
			t.Error("an invalid embedded 15-digit NPWP must not pass")
		}
	})

	t.Run("individual form is a valid NIK", func(t *testing.T) {
		if ok, _, err := ocr.ValidateNPWP("3201021708900001"); !ok {
			t.Fatalf("a valid NIK must be accepted as an individual NPWP16, got %v", err)
		}
	})

	t.Run("individual form rejects a structurally impossible NIK", func(t *testing.T) {
		if ok, _, _ := ocr.ValidateNPWP("9901021708900001"); ok {
			t.Error("an invalid province code must not pass as an individual NPWP16")
		}
	})
}

// ---------------------------------------------------------------------------
// Invoice — statutory PPN rates and tax arithmetic
// ---------------------------------------------------------------------------

func baseInvoice() *ocr.InvoiceData {
	return &ocr.InvoiceData{
		InvoiceNumber: "INV/2026/0001",
		InvoiceDate:   "2026-01-15",
		SellerName:    "PT Contoh Sintetis",
		BuyerName:     "PT Pembeli Sintetis",
		Currency:      "IDR",
		LineItems: []ocr.InvoiceLineItem{
			{Description: "Jasa", Quantity: 1, UnitPrice: 1000000, TotalPrice: 1000000},
		},
		Subtotal: 1000000,
	}
}

func TestValidateInvoice_AcceptsStatutoryPPNRates(t *testing.T) {
	for _, rate := range []float64{0.12, 0.11, 0.10} {
		t.Run(fmt.Sprintf("%.0f%%", rate*100), func(t *testing.T) {
			inv := baseInvoice()
			inv.DPP = 1000000
			inv.PPN = 1000000 * rate
			inv.GrandTotal = inv.DPP + inv.PPN

			if err := ocr.ValidateInvoice(inv); err != nil {
				t.Fatalf("statutory rate %.0f%% must be accepted: %v", rate*100, err)
			}
			if math.Abs(inv.PPNRate-rate) > 1e-9 {
				t.Errorf("derived PPNRate = %v, want %v", inv.PPNRate, rate)
			}
		})
	}
}

func TestValidateInvoice_RejectsNonStatutoryPPNRate(t *testing.T) {
	// Arithmetically consistent (GrandTotal = DPP + PPN) but 0.5% is not a rate
	// that exists in Indonesian law. This is the case that used to pass.
	inv := baseInvoice()
	inv.DPP = 1000000
	inv.PPN = 5000
	inv.GrandTotal = 1005000

	err := ocr.ValidateInvoice(inv)
	if err == nil {
		t.Fatal("a 0.5% PPN must be rejected even though the totals add up")
	}
	if !errors.Is(err, ocr.ErrPPNRateInvalid) {
		t.Errorf("expected ErrPPNRateInvalid, got %v", err)
	}
}

func TestValidateInvoice_AllowsExemptZeroPPN(t *testing.T) {
	inv := baseInvoice()
	inv.DPP = 1000000
	inv.PPN = 0
	inv.GrandTotal = 1000000

	if err := ocr.ValidateInvoice(inv); err != nil {
		t.Fatalf("a zero PPN (non-PKP, export, exempt goods) must be accepted: %v", err)
	}
	if inv.PPNRate != 0 {
		t.Errorf("PPNRate = %v, want 0 for an exempt invoice", inv.PPNRate)
	}
}

func TestValidateInvoice_DiscountBounds(t *testing.T) {
	t.Run("rejects a negative discount", func(t *testing.T) {
		inv := baseInvoice()
		inv.Discount = -50000
		if err := ocr.ValidateInvoice(inv); !errors.Is(err, ocr.ErrInvalidDiscount) {
			t.Errorf("expected ErrInvalidDiscount, got %v", err)
		}
	})

	t.Run("rejects a discount larger than the subtotal", func(t *testing.T) {
		inv := baseInvoice()
		inv.Discount = 2000000
		if err := ocr.ValidateInvoice(inv); !errors.Is(err, ocr.ErrInvalidDiscount) {
			t.Errorf("expected ErrInvalidDiscount, got %v", err)
		}
	})

	t.Run("applies a valid discount to the taxable base", func(t *testing.T) {
		inv := baseInvoice()
		inv.Discount = 200000
		inv.PPN = 88000 // 11% of 800000
		if err := ocr.ValidateInvoice(inv); err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
		if inv.DPP != 800000 {
			t.Errorf("DPP = %v, want 800000 (subtotal - discount)", inv.DPP)
		}
		if inv.GrandTotal != 888000 {
			t.Errorf("GrandTotal = %v, want 888000", inv.GrandTotal)
		}
	})
}

func TestValidateInvoice_TaxArithmeticIsEnforced(t *testing.T) {
	t.Run("line item total must equal qty * unit price", func(t *testing.T) {
		inv := baseInvoice()
		inv.LineItems[0].TotalPrice = 900000 // well outside the rounding tolerance
		if err := ocr.ValidateInvoice(inv); !errors.Is(err, ocr.ErrInvalidLineItem) {
			t.Errorf("expected ErrInvalidLineItem, got %v", err)
		}
	})

	t.Run("sub-rupiah line item rounding stays within tolerance", func(t *testing.T) {
		inv := baseInvoice()
		inv.LineItems[0].TotalPrice = 999999 // 1 rupiah of truncation
		inv.Subtotal = 999999
		inv.PPN = 109999.89
		if err := ocr.ValidateInvoice(inv); err != nil {
			t.Fatalf("a one-rupiah truncation must not fail an invoice: %v", err)
		}
	})

	t.Run("subtotal must equal the sum of line items", func(t *testing.T) {
		inv := baseInvoice()
		inv.Subtotal = 1500000
		if err := ocr.ValidateInvoice(inv); !errors.Is(err, ocr.ErrSubtotalMismatch) {
			t.Errorf("expected ErrSubtotalMismatch, got %v", err)
		}
	})

	t.Run("grand total must equal dpp + ppn", func(t *testing.T) {
		inv := baseInvoice()
		inv.DPP = 1000000
		inv.PPN = 110000
		inv.GrandTotal = 1500000
		if err := ocr.ValidateInvoice(inv); !errors.Is(err, ocr.ErrGrandTotalMismatch) {
			t.Errorf("expected ErrGrandTotalMismatch, got %v", err)
		}
	})

	t.Run("multi line invoice sums and taxes correctly", func(t *testing.T) {
		inv := baseInvoice()
		inv.LineItems = []ocr.InvoiceLineItem{
			{Description: "A", Quantity: 3, UnitPrice: 250000, TotalPrice: 750000},
			{Description: "B", Quantity: 2, UnitPrice: 125000, TotalPrice: 250000},
		}
		inv.Subtotal = 0 // derived
		inv.PPN = 120000 // 12% of 1000000
		if err := ocr.ValidateInvoice(inv); err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
		if inv.Subtotal != 1000000 || inv.DPP != 1000000 || inv.GrandTotal != 1120000 {
			t.Errorf("derived subtotal=%v dpp=%v grand=%v, want 1000000/1000000/1120000",
				inv.Subtotal, inv.DPP, inv.GrandTotal)
		}
	})

	t.Run("rounding within tolerance is accepted", func(t *testing.T) {
		inv := baseInvoice()
		inv.DPP = 1000000
		inv.PPN = 110000.4 // real e-faktur rounding
		inv.GrandTotal = 1110000
		if err := ocr.ValidateInvoice(inv); err != nil {
			t.Fatalf("sub-rupiah rounding must be tolerated: %v", err)
		}
	})
}
