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
