package ocr

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	nikRegex       = regexp.MustCompile(`(?i)(?:NIK|N1K|N!K)?[\s:;.-]*([0-9OlI]{16})`)
	nikFallback    = regexp.MustCompile(`\b([0-9]{16})\b`)
	namaRegex      = regexp.MustCompile(`(?i)(?:Nama|Narna)[\s:;.-]+([^\r\n]+)`)
	ttlRegex       = regexp.MustCompile(`(?i)(?:Tempat(?:/|\s*)Tgl\s*Lahir|Tempat\s*Lahir)[\s:;.-]+([^,;\r\n]+)[,;.\s]+(\d{1,2}[-\s/.]\d{1,2}[-\s/.]\d{4})`)
	genderRegex    = regexp.MustCompile(`(?i)(?:Jenis\s*Kelamin)[\s:;.-]*(LAKI-LAKI|PEREMPUAN)`)
	alamatRegex    = regexp.MustCompile(`(?i)(?:Alamat)[\s:;.-]+([^\n\r]+)`)
	rtrwRegex      = regexp.MustCompile(`(?i)(?:RT/RW|RT\s*/\s*RW)[\s:;.-]*(\d{1,3})\s*/\s*(\d{1,3})`)
	kelDesaRegex   = regexp.MustCompile(`(?i)(?:Kel/Desa|Kelurahan|Desa)[\s:;.-]+([^\n\r]+)`)
	kecamatanRegex = regexp.MustCompile(`(?i)(?:Kecamatan)[\s:;.-]+([^\n\r]+)`)
	agamaRegex     = regexp.MustCompile(`(?i)(?:Agama)[\s:;.-]*(ISLAM|KRISTEN|KATOLIK|HINDU|BUDDHA|KHONGHUCU)`)
	statusRegex    = regexp.MustCompile(`(?i)(?:Status\s*Perkawinan)[\s:;.-]*(BELUM\s+KAWIN|KAWIN|CERAI\s+HIDUP|CERAI\s+MATI)`)
	pekerjaanRegex = regexp.MustCompile(`(?i)(?:Pekerjaan)[\s:;.-]+([^\n\r]+)`)
	kwnRegex       = regexp.MustCompile(`(?i)(?:Kewarganegaraan)[\s:;.-]*(WNI|WNA)`)

	// SIM specific regex patterns
	simNomorRegex      = regexp.MustCompile(`(?i)(?:No\.?\s*SIM|Nomor\s*SIM|SIM\s*No\.?)[\t :;.-]*([0-9OlI\t -]{12,24})`)
	simNomorFallback  = regexp.MustCompile(`\b([0-9]{12,16})\b`)
	simGolonganRegex   = regexp.MustCompile(`(?i)(?:Gol(?:ongan)?\.?\s*(?:SIM)?|SIM)[\s:;.-]*([A-D](?:\s*(?:I|II|1|2))?(?:\s*UMUM)?|INTERNASIONAL)`)
	simNamaRegex       = regexp.MustCompile(`(?i)(?:1\.?\s*Nama|Nama)[\s:;.-]+([^\r\n]+)`)
	simTTLRegex        = regexp.MustCompile(`(?i)(?:2\.?\s*Tempat(?:/|\s*)Tgl\s*Lahir|Tempat(?:/|\s*)Tgl\s*Lahir)[\s:;.-]+([^,;\r\n]+)[,;.\s]+(\d{1,2}[-\s/.]\d{1,2}[-\s/.]\d{4})`)
	simDarahRegex      = regexp.MustCompile(`(?i)(?:Gol(?:ongan)?\.?\s*Darah|Darah)[\s:;.-]*([ABO-]{1,2})`)
	simGenderRegex     = regexp.MustCompile(`(?i)(?:3\.?\s*Jenis\s*Kelamin|Jenis\s*Kelamin)[\s:;.-]*(PRIA|WANITA|LAKI-LAKI|PEREMPUAN)`)
	simAlamatRegex     = regexp.MustCompile(`(?i)(?:4\.?\s*Alamat|Alamat)[\s:;.-]+([^\n\r]+)`)
	simPekerjaanRegex  = regexp.MustCompile(`(?i)(?:5\.?\s*Pekerjaan|Pekerjaan)[\s:;.-]+([^\n\r]+)`)
	simPoldaRegex      = regexp.MustCompile(`(?i)(?:Polda|Kepolisian\s*Daerah)[\s:;.-]+([^\n\r]+)`)
	simMasaBerlakuRegex = regexp.MustCompile(`(?i)(?:Berlaku\s*(?:s/d|hingga|sampai)|Masa\s*Berlaku)[\s:;.-]*(\d{1,2}[-\s/.]\d{1,2}[-\s/.]\d{4})`)

	// Passport specific regex patterns
	passportNomorRegex    = regexp.MustCompile(`(?i)(?:Passport\s*No\.?|Paspor\s*No\.?|No\.?\s*Paspor|Nomor\s*Paspor)[\s:;.-]*([A-Z][0-9A-Z]{7,8})`)
	passportNomorFallback = regexp.MustCompile(`\b([A-Z][0-9]{7,8})\b`)
	passportNamaRegex     = regexp.MustCompile(`(?i)(?:Nama\s*Lengkap(?:\s*/\s*Full\s*Name)?|Full\s*Name|Nama|Name)[\s:;.-]+([^\r\n]+)`)
	passportNatRegex      = regexp.MustCompile(`(?i)(?:Kewarganegaraan(?:\s*/\s*Nationality)?|Nationality)[\s:;.-]*(INDONESIA|WNI|IDN|[A-Z]{3})`)
	passportDOBRegex      = regexp.MustCompile(`(?i)(?:Tgl\s*Lahir|Date\s*of\s*Birth|Tanggal\s*Lahir)[\s:;.-]+(\d{1,2}[-\s/.]\d{1,2}[-\s/.]\d{4})`)
	passportPOBRegex      = regexp.MustCompile(`(?i)(?:Tempat\s*Lahir(?:\s*/\s*Place\s*of\s*Birth)?|Place\s*of\s*Birth)[\s:;.-]+([^\r\n,;]+)`)
	passportGenderRegex   = regexp.MustCompile(`(?i)(?:Jenis\s*Kelamin(?:\s*/\s*Sex)?|Sex)[\s:;.-]*([MF]|LAKI-LAKI|PEREMPUAN|PRIA|WANITA)`)
	passportIssueRegex    = regexp.MustCompile(`(?i)(?:Tgl\s*Pengeluaran(?:\s*/\s*Date\s*of\s*Issue)?|Date\s*of\s*Issue)[\s:;.-]+(\d{1,2}[-\s/.]\d{1,2}[-\s/.]\d{4})`)
	passportExpiryRegex   = regexp.MustCompile(`(?i)(?:Tgl\s*Habis\s*Berlaku(?:\s*/\s*Date\s*of\s*Expiry)?|Date\s*of\s*Expiry)[\s:;.-]+(\d{1,2}[-\s/.]\d{1,2}[-\s/.]\d{4})`)
	passportOfficeRegex   = regexp.MustCompile(`(?i)(?:Kantor\s*yang\s*Mengeluarkan(?:\s*/\s*Issuing\s*Office)?|Issuing\s*Office)[\s:;.-]+([^\r\n]+)`)
	passportMRZ1Regex     = regexp.MustCompile(`(P[<A-Z0-9]{43})`)
	passportMRZ2Regex     = regexp.MustCompile(`([A-Z0-9][<A-Z0-9]{43})`)

	// NPWP specific regex patterns
	npwpNomorRegex      = regexp.MustCompile(`(?i)(?:NPWP|Nomor\s*Pokok\s*Wajib\s*Pajak)[\s:;.-]*([0-9OlI\s.-]{15,24})`)
	npwpNomorFallback   = regexp.MustCompile(`\b([0-9]{2}[.-]?[0-9]{3}[.-]?[0-9]{3}[.-]?[0-9][.-]?[0-9]{3}[.-]?[0-9]{3})\b`)
	npwpNomor16Fallback = regexp.MustCompile(`\b([0-9]{16})\b`)
	npwpNamaRegex       = regexp.MustCompile(`(?i)(?:Nama(?:\s*WP)?|Nama\s*Wajib\s*Pajak)[\s:;.-]+([^\r\n]+)`)
	npwpNIKRegex        = regexp.MustCompile(`(?i)(?:NIK|N1K)[\s:;.-]*([0-9OlI]{16})`)
	npwpAlamatRegex     = regexp.MustCompile(`(?i)(?:Alamat)[\s:;.-]+([^\r\n]+)`)
	npwpKPPRegex        = regexp.MustCompile(`(?i)(?:KPP|Kantor\s*Pelayanan\s*Pajak)[\s:;.-]+([^\r\n]+)`)
	npwpDaftarRegex     = regexp.MustCompile(`(?i)(?:Terdaftar|Tgl\s*Daftar|Tanggal\s*Daftar)[\s:;.-]*(\d{1,2}[-\s/.]\d{1,2}[-\s/.]\d{4})`)

	// KK specific regex patterns
	kkNomorRegex       = regexp.MustCompile(`(?i)(?:No\.?\s*KK|Nomor\s*KK|Kartu\s*Keluarga\s*No\.?)[\s:;.-]*([0-9OlI\s.-]{16,24})`)
	kkNomorFallback    = regexp.MustCompile(`\b([0-9]{16})\b`)
	kkKepalaRegex      = regexp.MustCompile(`(?i)(?:Nama\s*Kepala\s*Keluarga|Kepala\s*Keluarga)[\s:;.-]+([^\r\n]+)`)
	kkAlamatRegex      = regexp.MustCompile(`(?i)(?:Alamat)[\s:;.-]+([^\r\n]+)`)
	kkRTRWRegex        = regexp.MustCompile(`(?i)(?:RT/RW|RT\s*/\s*RW)[\s:;.-]*(\d{1,3})\s*/\s*(\d{1,3})`)
	kkKodePosRegex     = regexp.MustCompile(`(?i)(?:Kode\s*Pos)[\s:;.-]*(\d{5})`)
	kkKelurahanRegex   = regexp.MustCompile(`(?i)(?:Desa/Kelurahan|Kelurahan/Desa|Kelurahan|Desa)[\s:;.-]+([^\r\n]+)`)
	kkKecamatanRegex   = regexp.MustCompile(`(?i)(?:Kecamatan)[\s:;.-]+([^\r\n]+)`)
	kkKabupatenRegex   = regexp.MustCompile(`(?i)(?:Kabupaten/Kota|Kabupaten|Kota)[\s:;.-]+([^\r\n]+)`)
	kkProvinsiRegex    = regexp.MustCompile(`(?i)(?:Provinsi)[\s:;.-]+([^\r\n]+)`)
	kkDikeluarkanRegex = regexp.MustCompile(`(?i)(?:Dikeluarkan\s*Tanggal|Tgl\s*Dikeluarkan)[\s:;.-]*(\d{1,2}[-\s/.]\d{1,2}[-\s/.]\d{4})`)

	// Invoice specific regex patterns
	invoiceNomorRegex    = regexp.MustCompile(`(?i)(?:Invoice\s*(?:No\.?|Nomor)|Nomor\s*(?:Faktur|Invoice)|No\.?\s*(?:Faktur|Invoice))[\t :;.-]*([A-Z0-9/.-]{3,30})`)
	invoiceDateRegex     = regexp.MustCompile(`(?i)(?:Tanggal(?:\s*Faktur|\s*Invoice)?|Date|Tgl)[\s:;.-]*(\d{1,2}[-\s/.]\d{1,2}[-\s/.]\d{4})`)
	invoiceDueDateRegex  = regexp.MustCompile(`(?i)(?:Jatuh\s*Tempo|Due\s*Date)[\s:;.-]*(\d{1,2}[-\s/.]\d{1,2}[-\s/.]\d{4})`)
	invoiceSellerRegex   = regexp.MustCompile(`(?i)(?:Pengusaha\s*Kena\s*Pajak|Nama\s*Penjual|Seller|Dari)[\s:;.-]+([^\r\n]+)`)
	invoiceBuyerRegex    = regexp.MustCompile(`(?i)(?:Pembeli\s*BKP\s*/\s*Penerima\s*JKP|Nama\s*Pembeli|Buyer|Kepada)[\s:;.-]+([^\r\n]+)`)
	invoiceSubtotalRegex = regexp.MustCompile(`(?i)(?:Jumlah\s*Harga\s*Jual|Subtotal|Sub\s*Total)[\s:;.Rp]*([\d.,]+)`)
	invoiceDPPRegex      = regexp.MustCompile(`(?i)(?:Dasar\s*Pengenaan\s*Pajak|DPP)[\s:;.Rp]*([\d.,]+)`)
	invoicePPNRegex      = regexp.MustCompile(`(?i)(?:Pajak\s*Pertambahan\s*Nilai|PPN|P\.P\.N\.)[\s:;.Rp]*([\d.,]+)`)
	invoiceTotalRegex    = regexp.MustCompile(`(?i)(?:Total\s*Tagihan|Grand\s*Total|Jumlah\s*Yang\s*Harus\s*Dibayar|Total)[\s:;.Rp]*([\d.,]+)`)
)

// cleanDigits fixes common OCR confusion in numeric fields (O->0, I/l->1).
func cleanDigits(s string) string {
	r := strings.NewReplacer(
		"O", "0", "o", "0", "D", "0",
		"I", "1", "l", "1", "|", "1",
	)
	return r.Replace(s)
}

// normalizeDate attempts to convert various date string layouts to standard YYYY-MM-DD.
func normalizeDate(dateStr string) string {
	dateStr = strings.TrimSpace(dateStr)
	dateStr = strings.ReplaceAll(dateStr, "/", "-")
	dateStr = strings.ReplaceAll(dateStr, ".", "-")
	dateStr = strings.ReplaceAll(dateStr, " ", "-")

	layouts := []string{
		"02-01-2006",
		"2-1-2006",
		"2006-01-02",
		"02-1-2006",
		"2-01-2006",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t.Format("2006-01-02")
		}
	}

	return dateStr
}

// cleanField strips noisy OCR artifacts and trims text.
func cleanField(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ":;,-. ")
	return strings.ToUpper(s)
}

// ParseKTPFromRawText extracts structured KTP fields from raw OCR text using regex and heuristics.
func ParseKTPFromRawText(rawText string) *KTPData {
	data := &KTPData{}

	// 1. NIK
	if m := nikRegex.FindStringSubmatch(rawText); len(m) > 1 {
		cleaned := cleanDigits(m[1])
		if len(cleaned) == 16 {
			data.NIK = cleaned
		}
	}
	if data.NIK == "" {
		if m := nikFallback.FindStringSubmatch(rawText); len(m) > 1 {
			data.NIK = m[1]
		}
	}

	// 2. Nama
	if m := namaRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Nama = cleanField(m[1])
	}

	// 3. Tempat / Tanggal Lahir
	if m := ttlRegex.FindStringSubmatch(rawText); len(m) > 2 {
		data.TempatLahir = cleanField(m[1])
		data.TanggalLahir = normalizeDate(m[2])
	}

	// 4. Jenis Kelamin
	if m := genderRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.JenisKelamin = cleanField(m[1])
	} else {
		// Fallback check
		upper := strings.ToUpper(rawText)
		if strings.Contains(upper, "LAKI-LAKI") {
			data.JenisKelamin = "LAKI-LAKI"
		} else if strings.Contains(upper, "PEREMPUAN") {
			data.JenisKelamin = "PEREMPUAN"
		}
	}

	// 5. Alamat
	if m := alamatRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Alamat = cleanField(m[1])
	}

	// 6. RT/RW
	if m := rtrwRegex.FindStringSubmatch(rawText); len(m) > 2 {
		rt := fmt.Sprintf("%03s", strings.TrimSpace(m[1]))
		rw := fmt.Sprintf("%03s", strings.TrimSpace(m[2]))
		data.RTRW = fmt.Sprintf("%s/%s", rt, rw)
	}

	// 7. Kelurahan / Desa
	if m := kelDesaRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Kelurahan = cleanField(m[1])
	}

	// 8. Kecamatan
	if m := kecamatanRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Kecamatan = cleanField(m[1])
	}

	// 9. Agama
	if m := agamaRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Agama = cleanField(m[1])
	}

	// 10. Status Perkawinan
	if m := statusRegex.FindStringSubmatch(rawText); len(m) > 1 {
		status := cleanField(m[1])
		// Normalize spaces (e.g. BELUM   KAWIN -> BELUM KAWIN)
		status = strings.Join(strings.Fields(status), " ")
		data.StatusPerkawinan = status
	}

	// 11. Pekerjaan
	if m := pekerjaanRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Pekerjaan = cleanField(m[1])
	}

	// 12. Kewarganegaraan
	if m := kwnRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Kewarganegaraan = cleanField(m[1])
	} else if strings.Contains(strings.ToUpper(rawText), "WNI") {
		data.Kewarganegaraan = "WNI"
	}

	return data
}

// ParseSIMFromRawText extracts structured SIM fields from raw OCR text using regex and heuristics.
func ParseSIMFromRawText(rawText string) *SIMData {
	data := &SIMData{}

	// 1. Nomor SIM
	if m := simNomorRegex.FindStringSubmatch(rawText); len(m) > 1 {
		cleaned := cleanDigits(m[1])
		cleaned = strings.ReplaceAll(cleaned, "-", "")
		cleaned = strings.ReplaceAll(cleaned, " ", "")
		cleaned = strings.ReplaceAll(cleaned, "\t", "")
		cleaned = strings.TrimSpace(cleaned)
		if len(cleaned) >= 12 && len(cleaned) <= 16 {
			data.NomorSIM = cleaned
		}
	}
	if data.NomorSIM == "" {
		if m := simNomorFallback.FindStringSubmatch(rawText); len(m) > 1 {
			data.NomorSIM = m[1]
		}
	}

	// 2. Golongan
	if m := simGolonganRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Golongan = cleanField(m[1])
	}

	// 3. Nama
	if m := simNamaRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Nama = cleanField(m[1])
	}

	// 4. Tempat & Tanggal Lahir
	if m := simTTLRegex.FindStringSubmatch(rawText); len(m) > 2 {
		data.TempatLahir = cleanField(m[1])
		data.TanggalLahir = normalizeDate(m[2])
	}

	// 5. Golongan Darah
	if m := simDarahRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.GolonganDarah = cleanField(m[1])
	}

	// 6. Jenis Kelamin
	if m := simGenderRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.JenisKelamin = cleanField(m[1])
	} else {
		upper := strings.ToUpper(rawText)
		if strings.Contains(upper, "PRIA") || strings.Contains(upper, "LAKI-LAKI") {
			data.JenisKelamin = "PRIA"
		} else if strings.Contains(upper, "WANITA") || strings.Contains(upper, "PEREMPUAN") {
			data.JenisKelamin = "WANITA"
		}
	}

	// 7. Alamat
	if m := simAlamatRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Alamat = cleanField(m[1])
	}

	// 8. Pekerjaan
	if m := simPekerjaanRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Pekerjaan = cleanField(m[1])
	}

	// 9. Polda
	if m := simPoldaRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Polda = cleanField(m[1])
	}

	// 10. Masa Berlaku
	if m := simMasaBerlakuRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.MasaBerlaku = normalizeDate(m[1])
	}

	return data
}

// ParsePassportFromRawText extracts structured Passport fields from raw OCR text using regex and heuristics.
func ParsePassportFromRawText(rawText string) *PassportData {
	data := &PassportData{}

	// 1. Passport Number
	if m := passportNomorRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.PassportNumber = strings.ToUpper(strings.TrimSpace(m[1]))
	}
	if data.PassportNumber == "" {
		if m := passportNomorFallback.FindStringSubmatch(rawText); len(m) > 1 {
			data.PassportNumber = strings.ToUpper(strings.TrimSpace(m[1]))
		}
	}

	// 2. Full Name
	if m := passportNamaRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.FullName = cleanField(m[1])
	}

	// 3. Nationality
	if m := passportNatRegex.FindStringSubmatch(rawText); len(m) > 1 {
		nat := strings.ToUpper(strings.TrimSpace(m[1]))
		if nat == "INDONESIA" || nat == "WNI" {
			nat = "IDN"
		}
		data.Nationality = nat
	} else if strings.Contains(strings.ToUpper(rawText), "INDONESIA") || strings.Contains(strings.ToUpper(rawText), "IDN") {
		data.Nationality = "IDN"
	}

	// 4. Date of Birth
	if m := passportDOBRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.DateOfBirth = normalizeDate(m[1])
	}

	// 5. Place of Birth
	if m := passportPOBRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.PlaceOfBirth = cleanField(m[1])
	}

	// 6. Gender
	if m := passportGenderRegex.FindStringSubmatch(rawText); len(m) > 1 {
		g := strings.ToUpper(strings.TrimSpace(m[1]))
		switch g {
		case "M", "PRIA", "LAKI-LAKI":
			data.Gender = "LAKI-LAKI"
		case "F", "WANITA", "PEREMPUAN":
			data.Gender = "PEREMPUAN"
		}
	} else {
		upper := strings.ToUpper(rawText)
		if strings.Contains(upper, "LAKI-LAKI") || strings.Contains(upper, "PRIA") {
			data.Gender = "LAKI-LAKI"
		} else if strings.Contains(upper, "PEREMPUAN") || strings.Contains(upper, "WANITA") {
			data.Gender = "PEREMPUAN"
		}
	}

	// 7. Issue Date
	if m := passportIssueRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.IssueDate = normalizeDate(m[1])
	}

	// 8. Expiry Date
	if m := passportExpiryRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.ExpiryDate = normalizeDate(m[1])
	}

	// 9. Issuing Office
	if m := passportOfficeRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.IssuingOffice = cleanField(m[1])
	}

	// 10. MRZ Lines
	if m := passportMRZ1Regex.FindStringSubmatch(rawText); len(m) > 1 {
		data.MRZLine1 = strings.ToUpper(strings.TrimSpace(m[1]))
	}
	if m := passportMRZ2Regex.FindStringSubmatch(rawText); len(m) > 1 {
		data.MRZLine2 = strings.ToUpper(strings.TrimSpace(m[1]))
	}

	return data
}

// ParseNPWPFromRawText extracts structured NPWP fields from raw OCR text using regex and heuristics.
func ParseNPWPFromRawText(rawText string) *NPWPData {
	data := &NPWPData{}

	// 1. NPWP Number
	if m := npwpNomorRegex.FindStringSubmatch(rawText); len(m) > 1 {
		cleaned := CleanNPWP(cleanDigits(m[1]))
		if len(cleaned) == 15 || len(cleaned) == 16 {
			data.NPWP = cleaned
		}
	}
	if data.NPWP == "" {
		if m := npwpNomorFallback.FindStringSubmatch(rawText); len(m) > 1 {
			data.NPWP = CleanNPWP(m[1])
		} else if m := npwpNomor16Fallback.FindStringSubmatch(rawText); len(m) > 1 {
			data.NPWP = m[1]
		}
	}

	// 2. Taxpayer Name
	if m := npwpNamaRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Nama = cleanField(m[1])
	}

	// 3. NIK (if on card)
	if m := npwpNIKRegex.FindStringSubmatch(rawText); len(m) > 1 {
		cleaned := cleanDigits(m[1])
		if len(cleaned) == 16 {
			data.NIK = cleaned
		}
	}

	// 4. Address
	if m := npwpAlamatRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Alamat = cleanField(m[1])
	}

	// 5. KPP
	if m := npwpKPPRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.KPP = cleanField(m[1])
	}

	// 6. Tanggal Daftar
	if m := npwpDaftarRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.TanggalDaftar = normalizeDate(m[1])
	}

	return data
}

// ParseKKFromRawText extracts structured Kartu Keluarga fields from raw OCR text using regex and heuristics.
func ParseKKFromRawText(rawText string) *KKData {
	data := &KKData{
		AnggotaKeluarga: make([]KKFamilyMember, 0),
	}

	// 1. Nomor KK
	if m := kkNomorRegex.FindStringSubmatch(rawText); len(m) > 1 {
		cleaned := CleanNomorKK(cleanDigits(m[1]))
		if len(cleaned) == 16 {
			data.NomorKK = cleaned
		}
	}
	if data.NomorKK == "" {
		if m := kkNomorFallback.FindStringSubmatch(rawText); len(m) > 1 {
			data.NomorKK = m[1]
		}
	}

	// 2. Kepala Keluarga
	if m := kkKepalaRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.KepalaKeluarga = cleanField(m[1])
	}

	// 3. Alamat
	if m := kkAlamatRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Alamat = cleanField(m[1])
	}

	// 4. RT / RW
	if m := kkRTRWRegex.FindStringSubmatch(rawText); len(m) > 2 {
		data.RTRW = fmt.Sprintf("%03s/%03s", m[1], m[2])
	}

	// 5. Kode Pos
	if m := kkKodePosRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.KodePos = m[1]
	}

	// 6. Kelurahan / Desa
	if m := kkKelurahanRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.KelurahanDesa = cleanField(m[1])
	}

	// 7. Kecamatan
	if m := kkKecamatanRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Kecamatan = cleanField(m[1])
	}

	// 8. Kabupaten / Kota
	if m := kkKabupatenRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.KabupatenKota = cleanField(m[1])
	}

	// 9. Provinsi
	if m := kkProvinsiRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Provinsi = cleanField(m[1])
	}

	// 10. Tanggal Dikeluarkan
	if m := kkDikeluarkanRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.TanggalDikeluarkan = normalizeDate(m[1])
	}

	return data
}

// parseAmount converts string currency representation (e.g. "1.500.000,00" or "1500000") to float64.
func parseAmount(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "Rp")
	s = strings.TrimPrefix(s, "RP")
	s = strings.TrimPrefix(s, "rp")
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ":;,-. ")

	if strings.Contains(s, ",") && strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	} else if strings.Contains(s, ".") && !strings.Contains(s, ",") {
		parts := strings.Split(s, ".")
		if len(parts) > 1 && len(parts[len(parts)-1]) == 3 {
			s = strings.ReplaceAll(s, ".", "")
		}
	} else if strings.Contains(s, ",") && !strings.Contains(s, ".") {
		parts := strings.Split(s, ",")
		if len(parts) > 1 && len(parts[len(parts)-1]) != 3 {
			s = strings.ReplaceAll(s, ",", ".")
		} else {
			s = strings.ReplaceAll(s, ",", "")
		}
	}

	var val float64
	_, _ = fmt.Sscanf(s, "%f", &val)
	return val
}

// ParseInvoiceFromRawText extracts structured Invoice / E-Faktur fields from raw OCR text using regex and heuristics.
func ParseInvoiceFromRawText(rawText string) *InvoiceData {
	data := &InvoiceData{
		Currency:  "IDR",
		LineItems: make([]InvoiceLineItem, 0),
	}

	// 1. Invoice Number
	if m := invoiceNomorRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.InvoiceNumber = strings.TrimSpace(m[1])
	}

	// 2. Invoice Date
	if m := invoiceDateRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.InvoiceDate = normalizeDate(m[1])
	}

	// 3. Due Date
	if m := invoiceDueDateRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.DueDate = normalizeDate(m[1])
	}

	// 4. Seller & Buyer
	if m := invoiceSellerRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.SellerName = cleanField(m[1])
	}
	if m := invoiceBuyerRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.BuyerName = cleanField(m[1])
	}

	// 5. Subtotal, DPP, PPN, Grand Total
	if m := invoiceSubtotalRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.Subtotal = parseAmount(m[1])
	}
	if m := invoiceDPPRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.DPP = parseAmount(m[1])
	}
	if m := invoicePPNRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.PPN = parseAmount(m[1])
	}
	if m := invoiceTotalRegex.FindStringSubmatch(rawText); len(m) > 1 {
		data.GrandTotal = parseAmount(m[1])
	}

	// Auto-fill DPP / GrandTotal if needed
	_ = ValidateInvoice(data)

	return data
}



