package ocr_test

import (
	"testing"

	"github.com/amirfaisalz/lensio/services/ocr"
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
		{
			name:        "official SIM header",
			text:        "KEPOLISIAN NEGARA REPUBLIK INDONESIA SURAT IZIN MENGEMUDI DRIVING LICENSE SIM A",
			wantDocType: "sim",
			wantIsKTP:   true,
		},
		{
			name:        "SIM markers without full header",
			text:        "POLRI GOL. SIM C MASA BERLAKU 2028-05-15 POLDA JAWA TIMUR",
			wantDocType: "sim",
			wantIsKTP:   true,
		},
		{
			name:        "official Passport header",
			text:        "REPUBLIK INDONESIA PASPOR PASSPORT KANTOR IMIGRASI NOMOR PASPOR X1234567",
			wantDocType: "passport",
			wantIsKTP:   true,
		},
		{
			name:        "Passport MRZ marker and date of expiry",
			text:        "P<IDNSANTOSO<<BUDI DATE OF EXPIRY DATE OF ISSUE",
			wantDocType: "passport",
			wantIsKTP:   true,
		},
		{
			name:        "official NPWP header and markers",
			text:        "KEMENTERIAN KEUANGAN DIREKTORAT JENDERAL PAJAK NOMOR POKOK WAJIB PAJAK NPWP 09.254.294.3-407.000 KPP PRATAMA",
			wantDocType: "npwp",
			wantIsKTP:   true,
		},
		{
			name:        "NPWP card markers without full header",
			text:        "NPWP WAJIB PAJAK TERDAFTAR KPP MADYA",
			wantDocType: "npwp",
			wantIsKTP:   true,
		},
		{
			name:        "official KK header",
			text:        "REPUBLIK INDONESIA KARTU KELUARGA NO. KK 3171010101200001 KEPALA KELUARGA BUDI SANTOSO",
			wantDocType: "kk",
			wantIsKTP:   true,
		},
		{
			name:        "KK markers without title",
			text:        "NO. KK 3171010101200001 HUBUNGAN DALAM KELUARGA STATUS HUBUNGAN NAMA AYAH NAMA IBU",
			wantDocType: "kk",
			wantIsKTP:   true,
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
