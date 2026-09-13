package ocr

import (
	"strings"
)

// List of distinctive textual markers present on official Indonesian KTPs.
var ktpKeywords = []string{
	"REPUBLIK INDONESIA",
	"KARTU TANDA PENDUDUK",
	"PROVINSI",
	"NIK",
	"TEMPAT/TGL LAHIR",
	"JENIS KELAMIN",
	"GOL. DARAH",
	"ALAMAT",
	"RT/RW",
	"KEL/DESA",
	"KELURAHAN",
	"KECAMATAN",
	"AGAMA",
	"STATUS PERKAWINAN",
	"PEKERJAAN",
	"KEWARGANEGARAAN",
	"BERLAKU HINGGA",
}

// List of distinctive textual markers present on official Indonesian SIMs.
var simKeywords = []string{
	"SURAT IZIN MENGEMUDI",
	"DRIVING LICENSE",
	"KEPOLISIAN NEGARA REPUBLIK INDONESIA",
	"POLRI",
	"KORLANTAS",
	"GOL. SIM",
	"GOLONGAN SIM",
	"MASA BERLAKU",
	"BERLAKU S/D",
	"BERLAKU HINGGA",
	"SIM A",
	"SIM B",
	"SIM C",
	"SIM D",
	"POLDA",
}

// List of distinctive textual markers present on official Indonesian Passports.
var passportKeywords = []string{
	"PASPOR",
	"PASSPORT",
	"KANTOR IMIGRASI",
	"KANIM",
	"KODE NEGARA",
	"COUNTRY CODE",
	"NOMOR PASPOR",
	"PASSPORT NO",
	"DATE OF BIRTH",
	"DATE OF EXPIRY",
	"DATE OF ISSUE",
	"ISSUING OFFICE",
	"P<IDN",
}

// List of distinctive textual markers present on official Indonesian NPWP cards.
var npwpKeywords = []string{
	"NPWP",
	"NOMOR POKOK WAJIB PAJAK",
	"KEMENTERIAN KEUANGAN",
	"DIREKTORAT JENDERAL PAJAK",
	"KPP PRATAMA",
	"KPP MADYA",
	"WAJIB PAJAK",
	"TERDAFTAR",
}

// List of distinctive textual markers present on official Indonesian Kartu Keluarga (KK).
var kkKeywords = []string{
	"KARTU KELUARGA",
	"NO. KK",
	"NOMOR KK",
	"KEPALA KELUARGA",
	"HUBUNGAN DALAM KELUARGA",
	"STATUS HUBUNGAN",
	"NAMA AYAH",
	"NAMA IBU",
	"DOKUMEN IMIGRASI",
}

// List of distinctive textual markers present on Indonesian Commercial Invoices and E-Faktur.
var invoiceKeywords = []string{
	"FAKTUR PAJAK",
	"INVOICE",
	"TAGIHAN",
	"FAKTUR PENJUALAN",
	"PENGUSAHA KENA PAJAK",
	"DASAR PENGENAAN PAJAK",
	"JUMLAH HARGA JUAL",
	"PPN",
	"TOTAL HARGA",
	"GRAND TOTAL",
	"JATUH TEMPO",
	"DUE DATE",
}

// ClassifyDocument analyzes raw text tokens to classify if the document is an Indonesian KTP, SIM, Passport, NPWP, KK, or Invoice.
// Returns document type string ("ktp", "sim", "passport", "npwp", "kk", "invoice", or "unsupported") and a boolean indicator of recognition.
// Minimum 2 strong markers required for positive classification (or 1 explicit title marker).
func ClassifyDocument(rawText string) (string, bool) {
	upper := strings.ToUpper(rawText)

	// Check KK first if explicit title exists
	kkMatches := 0
	hasExplicitKKTitle := strings.Contains(upper, "KARTU KELUARGA")
	for _, kw := range kkKeywords {
		if strings.Contains(upper, kw) {
			kkMatches++
		}
	}

	// Check Invoice if explicit title exists
	invoiceMatches := 0
	hasExplicitInvoiceTitle := strings.Contains(upper, "FAKTUR PAJAK") || strings.Contains(upper, "INVOICE") || strings.Contains(upper, "FAKTUR PENJUALAN") || (strings.Contains(upper, "TAGIHAN") && (strings.Contains(upper, "TOTAL") || strings.Contains(upper, "PPN") || strings.Contains(upper, "SUBTOTAL")))
	for _, kw := range invoiceKeywords {
		if strings.Contains(upper, kw) {
			invoiceMatches++
		}
	}

	// Check Passport first if explicit title or MRZ prefix exists
	passportMatches := 0
	hasExplicitPassportTitle := (strings.Contains(upper, "PASPOR") || strings.Contains(upper, "PASSPORT")) && (strings.Contains(upper, "INDONESIA") || strings.Contains(upper, "IDN"))
	for _, kw := range passportKeywords {
		if strings.Contains(upper, kw) {
			passportMatches++
		}
	}

	// Check NPWP
	npwpMatches := 0
	hasExplicitNPWPTitle := strings.Contains(upper, "NOMOR POKOK WAJIB PAJAK") || (strings.Contains(upper, "NPWP") && (strings.Contains(upper, "PAJAK") || strings.Contains(upper, "KPP") || strings.Contains(upper, "WAJIB PAJAK") || strings.Contains(upper, "DIREKTORAT JENDERAL PAJAK")))
	for _, kw := range npwpKeywords {
		if strings.Contains(upper, kw) {
			npwpMatches++
		}
	}

	// Check SIM
	simMatches := 0
	hasExplicitSIMTitle := strings.Contains(upper, "SURAT IZIN MENGEMUDI") || strings.Contains(upper, "DRIVING LICENSE")
	for _, kw := range simKeywords {
		if strings.Contains(upper, kw) {
			simMatches++
		}
	}

	ktpMatches := 0
	hasExplicitKTPTitle := strings.Contains(upper, "KARTU TANDA PENDUDUK") || strings.Contains(upper, "KTP")
	for _, kw := range ktpKeywords {
		if strings.Contains(upper, kw) {
			ktpMatches++
		}
	}

	// Check for 16-digit numeric pattern
	has16Digits := false
	for _, word := range strings.Fields(upper) {
		clean := strings.Trim(word, ":;,-.")
		if len(clean) == 16 {
			numeric := true
			for i := 0; i < 16; i++ {
				if clean[i] < '0' || clean[i] > '9' {
					numeric = false
					break
				}
			}
			if numeric {
				has16Digits = true
				break
			}
		}
	}

	if hasExplicitKKTitle || (kkMatches >= 2 && kkMatches > ktpMatches) {
		return "kk", true
	}

	if hasExplicitPassportTitle || (passportMatches >= 2 && passportMatches > ktpMatches && passportMatches > simMatches && passportMatches > npwpMatches) {
		return "passport", true
	}

	if hasExplicitInvoiceTitle || (invoiceMatches >= 2 && invoiceMatches > ktpMatches && invoiceMatches > simMatches && invoiceMatches > npwpMatches) {
		return "invoice", true
	}

	if hasExplicitNPWPTitle || (npwpMatches >= 2 && npwpMatches > ktpMatches && npwpMatches > simMatches && npwpMatches > passportMatches) {
		return "npwp", true
	}

	if hasExplicitSIMTitle || (simMatches >= 2 && simMatches > ktpMatches && simMatches > npwpMatches) {
		return "sim", true
	}

	if hasExplicitKTPTitle || ktpMatches >= 2 || (ktpMatches >= 1 && has16Digits) {
		return "ktp", true
	}

	if invoiceMatches >= 2 {
		return "invoice", true
	}

	if kkMatches >= 2 {
		return "kk", true
	}

	if npwpMatches >= 2 {
		return "npwp", true
	}

	if simMatches >= 2 {
		return "sim", true
	}

	if passportMatches >= 2 {
		return "passport", true
	}

	return "unsupported", false
}
