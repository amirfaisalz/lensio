package ocr

import (
	"testing"
)

func TestValidateInvoiceNumber(t *testing.T) {
	tests := []struct {
		name    string
		num     string
		wantErr bool
	}{
		{"valid standard invoice number", "INV/2026/001", false},
		{"valid e-faktur number", "010.000-24.12345678", false},
		{"valid short number", "F01", false},
		{"empty number", "", true},
		{"whitespace only", "   ", true},
		{"too short", "12", true},
		{"too long", "INV-" + string(make([]byte, 50)), true},
		{"non-printable ascii", "INV\x00123", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateInvoiceNumber(tc.num)
			if (err != nil) != tc.wantErr {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateInvoiceDate(t *testing.T) {
	tests := []struct {
		name    string
		dateStr string
		wantErr bool
	}{
		{"valid standard date", "2026-03-15", false},
		{"valid boundary date", "1995-12-31", false},
		{"empty date", "", true},
		{"invalid format", "15-03-2026", true},
		{"invalid month", "2026-13-01", true},
		{"year out of range low", "1850-01-01", true},
		{"year out of range high", "2150-01-01", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateInvoiceDate(tc.dateStr)
			if (err != nil) != tc.wantErr {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateInvoice(t *testing.T) {
	validSellerNPWP := "09.254.294.3-407.000"
	validBuyerNPWP := "09.254.294.3-407.000"

	t.Run("nil invoice data", func(t *testing.T) {
		err := ValidateInvoice(nil)
		if err == nil {
			t.Fatal("expected error for nil invoice data")
		}
	})

	t.Run("valid complete invoice with line items", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-2026-9901",
			InvoiceDate:   "2026-04-10",
			DueDate:       "2026-05-10",
			SellerName:    "PT TECH UTAMA SYNTHETIC",
			SellerNPWP:    validSellerNPWP,
			SellerAddress: "JL. JEND. SUDIRMAN KAV. 21, JAKARTA",
			BuyerName:     "PT MAJU MUNDUR SYNTHETIC",
			BuyerNPWP:     validBuyerNPWP,
			BuyerAddress:  "JL. THAMRIN NO. 10, JAKARTA",
			Currency:      "idr",
			LineItems: []InvoiceLineItem{
				{
					Description: "Cloud Infrastructure Setup",
					Quantity:    2,
					UnitPrice:   5000000,
					TotalPrice:  10000000,
				},
				{
					Description: "Security Hardening",
					Quantity:    1,
					UnitPrice:   5000000,
					TotalPrice:  5000000,
				},
			},
			Subtotal:   15000000,
			Discount:   1000000,
			DPP:        14000000,
			PPN:        1540000, // 11% of 14,000,000
			GrandTotal: 15540000,
		}

		err := ValidateInvoice(inv)
		if err != nil {
			t.Fatalf("expected valid invoice, got: %v", err)
		}
		if inv.Currency != "IDR" {
			t.Fatalf("expected normalized currency IDR, got %s", inv.Currency)
		}
		if inv.SellerNPWP != "092542943407000" {
			t.Fatalf("expected cleaned seller npwp, got %s", inv.SellerNPWP)
		}
	})

	t.Run("valid minimal invoice without line items", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-MINIMAL-01",
			InvoiceDate:   "2026-06-01",
			SellerName:    "TOKO ELEKTRONIK JAYA",
			BuyerName:     "JOHN DOE SYNTHETIC",
			Subtotal:      200000,
			PPN:           22000,
		}

		err := ValidateInvoice(inv)
		if err != nil {
			t.Fatalf("expected valid minimal invoice, got: %v", err)
		}
		if inv.Currency != "IDR" {
			t.Fatalf("expected default IDR currency, got %s", inv.Currency)
		}
		if inv.DPP != 200000 {
			t.Fatalf("expected DPP to be 200000, got %.2f", inv.DPP)
		}
		if inv.GrandTotal != 222000 {
			t.Fatalf("expected GrandTotal to be 222000, got %.2f", inv.GrandTotal)
		}
	})

	t.Run("valid invoice with 000 placeholder npwp", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-00-NPWP",
			InvoiceDate:   "2026-06-01",
			SellerName:    "TOKO KELONTONG",
			BuyerName:     "NON-PKP BUYER",
			SellerNPWP:    "00.000.000.0-000.000",
			BuyerNPWP:     "00.000.000.0-000.000",
			GrandTotal:    100000,
		}

		err := ValidateInvoice(inv)
		if err != nil {
			t.Fatalf("expected valid invoice with placeholder npwp, got: %v", err)
		}
	})

	t.Run("invalid invoice number", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "",
			InvoiceDate:   "2026-01-01",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for empty invoice number")
		}
	})

	t.Run("invalid invoice date", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "invalid-date",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for invalid invoice date")
		}
	})

	t.Run("due date before invoice date", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			DueDate:       "2026-05-10",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error when due date is before invoice date")
		}
	})

	t.Run("due date invalid format", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			DueDate:       "10-05-2026",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for malformed due date")
		}
	})

	t.Run("missing seller name", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			SellerName:    "   ",
			BuyerName:     "BUYER",
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for missing seller name")
		}
	})

	t.Run("missing buyer name", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			SellerName:    "SELLER",
			BuyerName:     "",
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for missing buyer name")
		}
	})

	t.Run("invalid seller npwp", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			SellerName:    "SELLER",
			SellerNPWP:    "12345",
			BuyerName:     "BUYER",
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for invalid seller npwp")
		}
	})

	t.Run("invalid buyer npwp", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
			BuyerNPWP:     "999999999999999",
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for invalid buyer npwp")
		}
	})

	t.Run("invalid currency length", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
			Currency:      "RUPIAH",
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for non-3-letter currency")
		}
	})

	t.Run("invalid currency characters", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
			Currency:      "123",
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for non-alphabetic currency")
		}
	})

	t.Run("invalid line item quantity", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
			LineItems: []InvoiceLineItem{
				{
					Description: "Bad Item",
					Quantity:    0,
					UnitPrice:   100,
				},
			},
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for zero quantity")
		}
	})

	t.Run("invalid line item price", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
			LineItems: []InvoiceLineItem{
				{
					Description: "Negative Price Item",
					Quantity:    1,
					UnitPrice:   -100,
				},
			},
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for negative unit price")
		}
	})

	t.Run("line item total mismatch", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
			LineItems: []InvoiceLineItem{
				{
					Description: "Item Mismatch",
					Quantity:    2,
					UnitPrice:   100,
					TotalPrice:  500, // Should be 200
				},
			},
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for line item total mismatch")
		}
	})

	t.Run("subtotal mismatch with sum of line items", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
			LineItems: []InvoiceLineItem{
				{
					Description: "Item 1",
					Quantity:    2,
					UnitPrice:   100,
					TotalPrice:  200,
				},
			},
			Subtotal: 300, // Should be 200
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for subtotal mismatch")
		}
	})

	t.Run("negative PPN", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
			Subtotal:      100,
			PPN:           -10,
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for negative ppn")
		}
	})

	t.Run("grand total mismatch with dpp + ppn", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
			Subtotal:      100,
			DPP:           100,
			PPN:           11,
			GrandTotal:    200, // Should be 111
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for grand total mismatch")
		}
	})

	t.Run("grand total non-positive", func(t *testing.T) {
		inv := &InvoiceData{
			InvoiceNumber: "INV-123",
			InvoiceDate:   "2026-05-15",
			SellerName:    "SELLER",
			BuyerName:     "BUYER",
			GrandTotal:    0,
		}
		err := ValidateInvoice(inv)
		if err == nil {
			t.Fatal("expected error for zero grand total")
		}
	})
}

func BenchmarkValidateInvoiceNumber(b *testing.B) {
	num := "010.000-24.12345678"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateInvoiceNumber(num)
	}
}

func BenchmarkValidateInvoiceDate(b *testing.B) {
	d := "2026-09-13"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ValidateInvoiceDate(d)
	}
}

func BenchmarkValidateInvoice(b *testing.B) {
	inv := &InvoiceData{
		InvoiceNumber: "INV-2026-9901",
		InvoiceDate:   "2026-04-10",
		DueDate:       "2026-05-10",
		SellerName:    "PT TECH UTAMA SYNTHETIC",
		SellerNPWP:    "09.254.294.3-407.000",
		BuyerName:     "PT MAJU MUNDUR SYNTHETIC",
		BuyerNPWP:     "09.254.294.3-407.000",
		Currency:      "IDR",
		LineItems: []InvoiceLineItem{
			{
				Description: "Item 1",
				Quantity:    2,
				UnitPrice:   50000,
				TotalPrice:  100000,
			},
			{
				Description: "Item 2",
				Quantity:    1,
				UnitPrice:   50000,
				TotalPrice:  50000,
			},
		},
		Subtotal:   150000,
		DPP:        150000,
		PPN:        16500,
		GrandTotal: 166500,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Pass copy of struct to benchmark validation overhead
		copyInv := *inv
		_ = ValidateInvoice(&copyInv)
	}
}
