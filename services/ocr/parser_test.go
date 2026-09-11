package ocr_test

import (
	"testing"

	"github.com/amirfaisalz/nusaid/services/ocr"
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
