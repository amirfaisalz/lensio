package ocr_test

import (
	"testing"

	"github.com/amirfaisalz/nusaid/services/ocr"
)

func TestClassifyDocument(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantDocType string
		wantIsKTP   bool
	}{
		{
			name:        "official KTP header and NIK",
			text:        "REPUBLIK INDONESIA PROVINSI DKI JAKARTA NIK 3171010101900001",
			wantDocType: "ktp",
			wantIsKTP:   true,
		},
		{
			name:        "KTP markers without full header",
			text:        "KARTU TANDA PENDUDUK ALAMAT RT/RW 001/002 AGAMA ISLAM",
			wantDocType: "ktp",
			wantIsKTP:   true,
		},
		{
			name:        "one marker with 16 digit number",
			text:        "PROVINSI JAWA TIMUR 3578010101900001",
			wantDocType: "ktp",
			wantIsKTP:   true,
		},
		{
			name:        "invoice or receipt (non-KTP)",
			text:        "SUPERMARKET STRUK BELANJA TOTAL RP 150000 TERIMA KASIH",
			wantDocType: "unsupported",
			wantIsKTP:   false,
		},
		{
			name:        "empty text",
			text:        "",
			wantDocType: "unsupported",
			wantIsKTP:   false,
		},
		{
			name:        "single non-KTP word with 16-character word with letters",
			text:        "PROVINSI 317101010190000A",
			wantDocType: "unsupported",
			wantIsKTP:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			docType, isKTP := ocr.ClassifyDocument(tc.text)
			if docType != tc.wantDocType || isKTP != tc.wantIsKTP {
				t.Fatalf("expected docType=%q isKTP=%v, got docType=%q isKTP=%v", tc.wantDocType, tc.wantIsKTP, docType, isKTP)
			}
		})
	}
}
