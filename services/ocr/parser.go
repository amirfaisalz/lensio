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

