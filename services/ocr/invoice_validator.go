package ocr

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var (
	// ErrInvoiceNumberMissing indicates invoice number is empty.
	ErrInvoiceNumberMissing = errors.New("invoice number is required")

	// ErrInvoiceNumberInvalid indicates invoice number has invalid characters or length.
	ErrInvoiceNumberInvalid = errors.New("invoice number must be between 3 and 50 characters")

	// ErrInvoiceDateMissing indicates invoice date is empty.
	ErrInvoiceDateMissing = errors.New("invoice date is required")

	// ErrInvoiceDateInvalid indicates invoice date format is invalid.
	ErrInvoiceDateInvalid = errors.New("invoice date must be in YYYY-MM-DD format")

	// ErrDueDateInvalid indicates due date format is invalid or earlier than invoice date.
	ErrDueDateInvalid = errors.New("due date must be in YYYY-MM-DD format and on or after invoice date")

	// ErrSellerNameMissing indicates seller name is empty.
	ErrSellerNameMissing = errors.New("seller name is required")

	// ErrBuyerNameMissing indicates buyer name is empty.
	ErrBuyerNameMissing = errors.New("buyer name is required")

	// ErrInvalidCurrency indicates currency code is invalid.
	ErrInvalidCurrency = errors.New("currency must be a 3-letter alphabetic ISO code")

	// ErrInvalidLineItem indicates a line item has non-positive quantity or negative unit price.
	ErrInvalidLineItem = errors.New("line item quantity must be greater than 0 and unit price must be non-negative")

	// ErrSubtotalMismatch indicates subtotal differs from sum of line items.
	ErrSubtotalMismatch = errors.New("subtotal does not match sum of line items")

	// ErrGrandTotalMismatch indicates grand total differs from dpp + ppn.
	ErrGrandTotalMismatch = errors.New("grand total does not match dpp + ppn calculation")

	// ErrGrandTotalNonPositive indicates grand total is zero or negative.
	ErrGrandTotalNonPositive = errors.New("grand total must be greater than zero")
)

const floatRoundingTolerance = 2.0 // Tolerates minor tax rounding or decimal truncations in Indonesian Rupiah

// ValidateInvoiceNumber verifies invoice or e-faktur number structure.
// Complexity: Time O(n), Space O(1).
func ValidateInvoiceNumber(num string) error {
	trimmed := strings.TrimSpace(num)
	if trimmed == "" {
		return ErrInvoiceNumberMissing
	}
	if len(trimmed) < 3 || len(trimmed) > 50 {
		return ErrInvoiceNumberInvalid
	}
	for i := 0; i < len(trimmed); i++ {
		b := trimmed[i]
		// Printable ASCII between 32 and 126
		if b < 32 || b > 126 {
			return ErrInvoiceNumberInvalid
		}
	}
	return nil
}

// ValidateInvoiceDate verifies YYYY-MM-DD date within realistic range.
// Complexity: Time O(1), Space O(1).
func ValidateInvoiceDate(dateStr string) (time.Time, error) {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return time.Time{}, ErrInvoiceDateMissing
	}
	t, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrInvoiceDateInvalid, err)
	}
	if t.Year() < 1990 || t.Year() > 2100 {
		return time.Time{}, fmt.Errorf("%w: year %d out of valid range", ErrInvoiceDateInvalid, t.Year())
	}
	return t, nil
}

// ValidateInvoice validates all fields and arithmetic consistency in an Indonesian commercial invoice or e-faktur.
// Complexity: Time O(n) where n is the number of line items, Space O(1).
func ValidateInvoice(data *InvoiceData) error {
	if data == nil {
		return errors.New("invoice data cannot be nil")
	}

	// 1. Invoice Number
	data.InvoiceNumber = strings.TrimSpace(data.InvoiceNumber)
	if err := ValidateInvoiceNumber(data.InvoiceNumber); err != nil {
		return err
	}

	// 2. Invoice Date
	data.InvoiceDate = strings.TrimSpace(data.InvoiceDate)
	invDate, err := ValidateInvoiceDate(data.InvoiceDate)
	if err != nil {
		return err
	}

	// 3. Due Date (optional)
	data.DueDate = strings.TrimSpace(data.DueDate)
	if data.DueDate != "" {
		dueDate, err := time.Parse("2006-01-02", data.DueDate)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrDueDateInvalid, err)
		}
		if dueDate.Before(invDate) {
			return fmt.Errorf("%w: due date %s is before invoice date %s", ErrDueDateInvalid, data.DueDate, data.InvoiceDate)
		}
	}

	// 4. Seller & Buyer Names
	data.SellerName = strings.TrimSpace(data.SellerName)
	if data.SellerName == "" {
		return ErrSellerNameMissing
	}
	data.BuyerName = strings.TrimSpace(data.BuyerName)
	if data.BuyerName == "" {
		return ErrBuyerNameMissing
	}

	// 5. Seller & Buyer NPWP (optional, but if provided must be valid unless placeholder zero)
	data.SellerNPWP = strings.TrimSpace(data.SellerNPWP)
	if data.SellerNPWP != "" && CleanNPWP(data.SellerNPWP) != "000000000000000" && CleanNPWP(data.SellerNPWP) != "0000000000000000" {
		valid, cleaned, err := ValidateNPWP(data.SellerNPWP)
		if !valid || err != nil {
			return fmt.Errorf("invalid seller npwp: %w", err)
		}
		data.SellerNPWP = cleaned
	}

	data.BuyerNPWP = strings.TrimSpace(data.BuyerNPWP)
	if data.BuyerNPWP != "" && CleanNPWP(data.BuyerNPWP) != "000000000000000" && CleanNPWP(data.BuyerNPWP) != "0000000000000000" {
		valid, cleaned, err := ValidateNPWP(data.BuyerNPWP)
		if !valid || err != nil {
			return fmt.Errorf("invalid buyer npwp: %w", err)
		}
		data.BuyerNPWP = cleaned
	}

	// 6. Currency
	data.Currency = strings.ToUpper(strings.TrimSpace(data.Currency))
	if data.Currency == "" {
		data.Currency = "IDR"
	} else if len(data.Currency) != 3 {
		return ErrInvalidCurrency
	} else {
		for i := 0; i < 3; i++ {
			if data.Currency[i] < 'A' || data.Currency[i] > 'Z' {
				return ErrInvalidCurrency
			}
		}
	}

	// 7. Line Items & Subtotal
	var sumLineItems float64
	hasLineItems := len(data.LineItems) > 0
	for i := range data.LineItems {
		item := &data.LineItems[i]
		item.Description = strings.TrimSpace(item.Description)
		if item.Quantity <= 0 || item.UnitPrice < 0 {
			return fmt.Errorf("%w at index %d", ErrInvalidLineItem, i)
		}
		expectedTotal := item.Quantity * item.UnitPrice
		if item.TotalPrice == 0 {
			item.TotalPrice = expectedTotal
		} else if math.Abs(item.TotalPrice-expectedTotal) > floatRoundingTolerance {
			return fmt.Errorf("%w: line item %d total %.2f differs from qty*price %.2f", ErrInvalidLineItem, i, item.TotalPrice, expectedTotal)
		}
		sumLineItems += item.TotalPrice
	}

	if hasLineItems {
		if data.Subtotal == 0 {
			data.Subtotal = sumLineItems
		} else if math.Abs(data.Subtotal-sumLineItems) > floatRoundingTolerance {
			return fmt.Errorf("%w: subtotal %.2f != sum of line items %.2f", ErrSubtotalMismatch, data.Subtotal, sumLineItems)
		}
	}

	// 8. DPP (Dasar Pengenaan Pajak) & Tax Math
	if data.DPP == 0 {
		if data.Subtotal > 0 {
			data.DPP = data.Subtotal - data.Discount
		} else if data.GrandTotal > 0 {
			data.DPP = data.GrandTotal - data.PPN
			data.Subtotal = data.DPP
		}
	}
	if data.PPN < 0 {
		return errors.New("ppn cannot be negative")
	}

	expectedGrandTotal := data.DPP + data.PPN
	if data.GrandTotal == 0 {
		data.GrandTotal = expectedGrandTotal
	} else if math.Abs(data.GrandTotal-expectedGrandTotal) > floatRoundingTolerance {
		return fmt.Errorf("%w: grand total %.2f != dpp (%.2f) + ppn (%.2f)", ErrGrandTotalMismatch, data.GrandTotal, data.DPP, data.PPN)
	}

	if data.GrandTotal <= 0 {
		return ErrGrandTotalNonPositive
	}

	return nil
}
