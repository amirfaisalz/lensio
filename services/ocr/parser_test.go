package ocr_test

import (
	"strings"
	"testing"

	"github.com/amirfaisalz/lensio/services/ocr"
)

func TestParseKTPFromRawText(t *testing.T) {
	sampleRawText := `
PROVINSI DKI JAKARTA
JAKARTA PUSAT
NIK : 3171010101900001
Nama : BUDI SANTOSO
Tempat/Tgl Lahir : JAKARTA, 01-01-1990
Jenis Kelamin : LAKI-LAKI
Alamat : JL. MERDEKA NO. 10
RT/RW : 1/2
Kel/Desa : GAMBIR
Kecamatan : GAMBIR
Agama : ISLAM
Status Perkawinan: KAWIN
Pekerjaan : KARYAWAN SWASTA
Kewarganegaraan : WNI
`

	data := ocr.ParseKTPFromRawText(sampleRawText)
	if data.NIK != "3171010101900001" {
		t.Errorf("expected NIK 3171010101900001, got %s", data.NIK)
	}
	if data.Nama != "BUDI SANTOSO" {
		t.Errorf("expected Nama BUDI SANTOSO, got %s", data.Nama)
	}
	if data.TempatLahir != "JAKARTA" {
		t.Errorf("expected TempatLahir JAKARTA, got %s", data.TempatLahir)
	}
	if data.TanggalLahir != "1990-01-01" {
		t.Errorf("expected TanggalLahir 1990-01-01, got %s", data.TanggalLahir)
	}
	if data.JenisKelamin != "LAKI-LAKI" {
		t.Errorf("expected JenisKelamin LAKI-LAKI, got %s", data.JenisKelamin)
	}
	if data.Alamat != "JL. MERDEKA NO. 10" {
		t.Errorf("expected Alamat JL. MERDEKA NO. 10, got %s", data.Alamat)
	}
	if data.RTRW != "001/002" {
		t.Errorf("expected RTRW 001/002, got %s", data.RTRW)
	}
	if data.Kelurahan != "GAMBIR" {
		t.Errorf("expected Kelurahan GAMBIR, got %s", data.Kelurahan)
	}
	if data.Kecamatan != "GAMBIR" {
		t.Errorf("expected Kecamatan GAMBIR, got %s", data.Kecamatan)
	}
	if data.Agama != "ISLAM" {
		t.Errorf("expected Agama ISLAM, got %s", data.Agama)
	}
	if data.StatusPerkawinan != "KAWIN" {
		t.Errorf("expected StatusPerkawinan KAWIN, got %s", data.StatusPerkawinan)
	}
	if data.Pekerjaan != "KARYAWAN SWASTA" {
		t.Errorf("expected Pekerjaan KARYAWAN SWASTA, got %s", data.Pekerjaan)
	}
	if data.Kewarganegaraan != "WNI" {
		t.Errorf("expected Kewarganegaraan WNI, got %s", data.Kewarganegaraan)
	}

	t.Run("ocr typos and date formats", func(t *testing.T) {
		noisyText := `
NIK : 3171OlOlOl9OOOO1
Narna : SITI AMINAH
Tempat Lahir : BANDUNG, 15/05/1995
PEREMPUAN
Kewarganegaraan : INDONESIA WNI
`
		res := ocr.ParseKTPFromRawText(noisyText)
		if res.NIK != "3171010101900001" {
			t.Errorf("expected cleaned NIK, got %s", res.NIK)
		}
		if res.Nama != "SITI AMINAH" {
			t.Errorf("expected cleaned Nama, got %s", res.Nama)
		}
		if res.TempatLahir != "BANDUNG" {
			t.Errorf("expected BANDUNG, got %s", res.TempatLahir)
		}
		if res.TanggalLahir != "1995-05-15" {
			t.Errorf("expected normalized date 1995-05-15, got %s", res.TanggalLahir)
		}
		if res.JenisKelamin != "PEREMPUAN" {
			t.Errorf("expected PEREMPUAN, got %s", res.JenisKelamin)
		}
	})

	t.Run("fallback NIK detection without prefix", func(t *testing.T) {
		fallbackText := "REPUBLIK INDONESIA 3201015505950002 SITI LAKI-LAKI"
		res := ocr.ParseKTPFromRawText(fallbackText)
		if res.NIK != "3201015505950002" {
			t.Errorf("expected fallback NIK 3201015505950002, got %s", res.NIK)
		}
		if res.JenisKelamin != "LAKI-LAKI" {
			t.Errorf("expected fallback gender LAKI-LAKI, got %s", res.JenisKelamin)
		}
	})

	t.Run("unparseable date returns raw string", func(t *testing.T) {
		raw := "Tempat/Tgl Lahir : JAKARTA, 99-99-9999"
		res := ocr.ParseKTPFromRawText(raw)
		if res.TanggalLahir != "99-99-9999" {
			t.Errorf("expected unchanged raw date string, got %s", res.TanggalLahir)
		}
	})
}

func BenchmarkParseKTPFromRawText(b *testing.B) {
	raw := `
PROVINSI DKI JAKARTA
NIK : 3171010101900001
Nama : BUDI SANTOSO
Tempat/Tgl Lahir : JAKARTA, 01-01-1990
Jenis Kelamin : LAKI-LAKI
Alamat : JL. MERDEKA NO. 10
RT/RW : 001/002
Kel/Desa : GAMBIR
Kecamatan : GAMBIR
Agama : ISLAM
Status Perkawinan: KAWIN
Pekerjaan : KARYAWAN SWASTA
Kewarganegaraan : WNI
`
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = ocr.ParseKTPFromRawText(raw)
	}
}

func TestParseSIMFromRawText(t *testing.T) {
	sampleRawText := `
KEPOLISIAN NEGARA REPUBLIK INDONESIA
SURAT IZIN MENGEMUDI
DRIVING LICENSE
SIM A
No. SIM: 1234-5678-9012
1. Nama : BUDI SANTOSO
2. Tempat/Tgl Lahir : JAKARTA, 01-01-1990
3. Gol. Darah : O - Jenis Kelamin : PRIA
4. Alamat : JL. MERDEKA NO. 10
5. Pekerjaan : KARYAWAN SWASTA
Polda : POLDA METRO JAYA
Berlaku s/d : 01-01-2029
`

	data := ocr.ParseSIMFromRawText(sampleRawText)
	if data.NomorSIM != "123456789012" {
		t.Errorf("expected NomorSIM 123456789012, got %s", data.NomorSIM)
	}
	if data.Golongan != "A" {
		t.Errorf("expected Golongan A, got %s", data.Golongan)
	}
	if data.Nama != "BUDI SANTOSO" {
		t.Errorf("expected Nama BUDI SANTOSO, got %s", data.Nama)
	}
	if data.TempatLahir != "JAKARTA" {
		t.Errorf("expected TempatLahir JAKARTA, got %s", data.TempatLahir)
	}
	if data.TanggalLahir != "1990-01-01" {
		t.Errorf("expected TanggalLahir 1990-01-01, got %s", data.TanggalLahir)
	}
	if data.GolonganDarah != "O" {
		t.Errorf("expected GolonganDarah O, got %s", data.GolonganDarah)
	}
	if data.JenisKelamin != "PRIA" {
		t.Errorf("expected JenisKelamin PRIA, got %s", data.JenisKelamin)
	}
	if data.Alamat != "JL. MERDEKA NO. 10" {
		t.Errorf("expected Alamat JL. MERDEKA NO. 10, got %s", data.Alamat)
	}
	if data.Pekerjaan != "KARYAWAN SWASTA" {
		t.Errorf("expected Pekerjaan KARYAWAN SWASTA, got %s", data.Pekerjaan)
	}
	if data.Polda != "POLDA METRO JAYA" {
		t.Errorf("expected Polda POLDA METRO JAYA, got %s", data.Polda)
	}
	if data.MasaBerlaku != "2029-01-01" {
		t.Errorf("expected MasaBerlaku 2029-01-01, got %s", data.MasaBerlaku)
	}

	t.Run("fallback SIM number detection", func(t *testing.T) {
		fallbackText := "SURAT IZIN MENGEMUDI 12345678901234 WANITA"
		res := ocr.ParseSIMFromRawText(fallbackText)
		if res.NomorSIM != "12345678901234" {
			t.Errorf("expected fallback NomorSIM 12345678901234, got %s", res.NomorSIM)
		}
		if res.JenisKelamin != "WANITA" {
			t.Errorf("expected fallback gender WANITA, got %s", res.JenisKelamin)
		}
	})
}

func BenchmarkParseSIMFromRawText(b *testing.B) {
	raw := `
KEPOLISIAN NEGARA REPUBLIK INDONESIA
SURAT IZIN MENGEMUDI
SIM A
No. SIM: 1234-5678-9012
1. Nama : BUDI SANTOSO
2. Tempat/Tgl Lahir : JAKARTA, 01-01-1990
3. Jenis Kelamin : PRIA
4. Alamat : JL. MERDEKA NO. 10
5. Pekerjaan : KARYAWAN SWASTA
Polda : METRO JAYA
Berlaku s/d : 01-01-2029
`
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = ocr.ParseSIMFromRawText(raw)
	}
}

func TestParsePassportFromRawText(t *testing.T) {
	raw := `
REPUBLIK INDONESIA / REPUBLIC OF INDONESIA
PASPOR / PASSPORT
Jenis / Type: P  Kode Negara / Country Code: IDN  Nomor Paspor / Passport No: X1234567
Nama Lengkap / Full Name: BUDI SANTOSO
Kewarganegaraan / Nationality: INDONESIA
Tanggal Lahir / Date of Birth: 01-01-1990
Tempat Lahir / Place of Birth: JAKARTA
Jenis Kelamin / Sex: LAKI-LAKI
Tanggal Pengeluaran / Date of Issue: 01-01-2020
Tanggal Habis Berlaku / Date of Expiry: 01-01-2030
Kantor yang Mengeluarkan / Issuing Office: KANIM JAKARTA SELATAN
P<IDNSANTOSO<<BUDI<<<<<<<<<<<<<<<<<<<<<<<<<<
X1234567<7IDN9001011M3001019<<<<<<<<<<<<<<<2
`
	data := ocr.ParsePassportFromRawText(raw)
	if data.PassportNumber != "X1234567" {
		t.Errorf("expected PassportNumber X1234567, got %s", data.PassportNumber)
	}
	if data.FullName != "BUDI SANTOSO" {
		t.Errorf("expected FullName BUDI SANTOSO, got %s", data.FullName)
	}
	if data.Nationality != "IDN" {
		t.Errorf("expected Nationality IDN, got %s", data.Nationality)
	}
	if data.DateOfBirth != "1990-01-01" {
		t.Errorf("expected DateOfBirth 1990-01-01, got %s", data.DateOfBirth)
	}
	if data.PlaceOfBirth != "JAKARTA" {
		t.Errorf("expected PlaceOfBirth JAKARTA, got %s", data.PlaceOfBirth)
	}
	if data.Gender != "LAKI-LAKI" {
		t.Errorf("expected Gender LAKI-LAKI, got %s", data.Gender)
	}
	if data.IssueDate != "2020-01-01" {
		t.Errorf("expected IssueDate 2020-01-01, got %s", data.IssueDate)
	}
	if data.ExpiryDate != "2030-01-01" {
		t.Errorf("expected ExpiryDate 2030-01-01, got %s", data.ExpiryDate)
	}
	if data.IssuingOffice != "KANIM JAKARTA SELATAN" {
		t.Errorf("expected IssuingOffice KANIM JAKARTA SELATAN, got %s", data.IssuingOffice)
	}
	if !strings.HasPrefix(data.MRZLine1, "P<IDN") {
		t.Errorf("expected MRZLine1 starting with P<IDN, got %s", data.MRZLine1)
	}
	if len(data.MRZLine2) != 44 {
		t.Errorf("expected MRZLine2 len 44, got %d", len(data.MRZLine2))
	}

	t.Run("fallback passport number and female gender detection", func(t *testing.T) {
		fallbackText := "PASPOR REPUBLIK INDONESIA A98765432 WANITA"
		res := ocr.ParsePassportFromRawText(fallbackText)
		if res.PassportNumber != "A98765432" {
			t.Errorf("expected fallback PassportNumber A98765432, got %s", res.PassportNumber)
		}
		if res.Gender != "PEREMPUAN" {
			t.Errorf("expected fallback gender PEREMPUAN, got %s", res.Gender)
		}
	})
}

func BenchmarkParsePassportFromRawText(b *testing.B) {
	raw := `
PASPOR REPUBLIK INDONESIA
Passport No: X1234567
Nama Lengkap: BUDI SANTOSO
Nationality: IDN
Date of Birth: 01-01-1990
Place of Birth: JAKARTA
Sex: M
Date of Issue: 01-01-2020
Date of Expiry: 01-01-2030
Issuing Office: JAKARTA SELATAN
P<IDNSANTOSO<<BUDI<<<<<<<<<<<<<<<<<<<<<<<<<<
X1234567<7IDN9001011M3001019<<<<<<<<<<<<<<<1
`
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = ocr.ParsePassportFromRawText(raw)
	}
}

func TestParseNPWPFromRawText(t *testing.T) {
	raw := `
KEMENTERIAN KEUANGAN REPUBLIK INDONESIA
DIREKTORAT JENDERAL PAJAK
NPWP : 09.254.294.3-407.000
NAMA : BUDI SANTOSO
NIK  : 3171010101900001
ALAMAT : JL. JENDERAL SUDIRMAN KAV. 21
KPP : KPP PRATAMA JAKARTA SETIABUDI SATU
TERDAFTAR : 17-08-2015
`
	data := ocr.ParseNPWPFromRawText(raw)
	if data.NPWP != "092542943407000" {
		t.Errorf("expected NPWP 092542943407000, got %s", data.NPWP)
	}
	if data.Nama != "BUDI SANTOSO" {
		t.Errorf("expected Nama BUDI SANTOSO, got %s", data.Nama)
	}
	if data.NIK != "3171010101900001" {
		t.Errorf("expected NIK 3171010101900001, got %s", data.NIK)
	}
	if data.Alamat != "JL. JENDERAL SUDIRMAN KAV. 21" {
		t.Errorf("expected Alamat JL. JENDERAL SUDIRMAN KAV. 21, got %s", data.Alamat)
	}
	if data.KPP != "KPP PRATAMA JAKARTA SETIABUDI SATU" {
		t.Errorf("expected KPP KPP PRATAMA JAKARTA SETIABUDI SATU, got %s", data.KPP)
	}
	if data.TanggalDaftar != "2015-08-17" {
		t.Errorf("expected TanggalDaftar 2015-08-17, got %s", data.TanggalDaftar)
	}

	t.Run("fallback 15 digit and 16 digit NPWP number detection", func(t *testing.T) {
		fallback15 := "KEMENTERIAN KEUANGAN 09.254.294.3-407.000 PAJAK"
		res15 := ocr.ParseNPWPFromRawText(fallback15)
		if res15.NPWP != "092542943407000" {
			t.Errorf("expected fallback 15-digit NPWP 092542943407000, got %s", res15.NPWP)
		}

		fallback16 := "DIREKTORAT JENDERAL PAJAK 0092542943407000 WAJIB PAJAK"
		res16 := ocr.ParseNPWPFromRawText(fallback16)
		if res16.NPWP != "0092542943407000" {
			t.Errorf("expected fallback 16-digit NPWP 0092542943407000, got %s", res16.NPWP)
		}
	})
}

func BenchmarkParseNPWPFromRawText(b *testing.B) {
	raw := `
KEMENTERIAN KEUANGAN REPUBLIK INDONESIA
DIREKTORAT JENDERAL PAJAK
NPWP : 09.254.294.3-407.000
NAMA : BUDI SANTOSO
NIK  : 3171010101900001
ALAMAT : JL. JENDERAL SUDIRMAN KAV. 21
KPP : KPP PRATAMA JAKARTA SETIABUDI SATU
TERDAFTAR : 17-08-2015
`
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = ocr.ParseNPWPFromRawText(raw)
	}
}

