package ocr_test

import (
	"testing"

	"github.com/amirfaisalz/lensio/services/ocr"
)

func TestValidateNIK(t *testing.T) {
	tests := []struct {
		name         string
		nik          string
		wantValid    bool
		wantFemale   bool
		wantDay      int
		wantMonth    int
		wantErrSub   string
	}{
		{
			name:       "valid male Jakarta NIK",
			nik:        "3171010101900001",
			wantValid:  true,
			wantFemale: false,
			wantDay:    1,
			wantMonth:  1,
		},
		{
			name:       "valid female Jabar NIK",
			nik:        "3201015505950002",
			wantValid:  true,
			wantFemale: true,
			wantDay:    15,
			wantMonth:  5,
		},
		{
			name:       "valid leap year Feb 29 NIK",
			nik:        "3578012902040003",
			wantValid:  true,
			wantFemale: false,
			wantDay:    29,
			wantMonth:  2,
		},
		{
			name:       "too short",
			nik:        "317101010190000",
			wantValid:  false,
			wantErrSub: "length must be exactly 16 digits",
		},
		{
			name:       "too long",
			nik:        "317101010190000199",
			wantValid:  false,
			wantErrSub: "length must be exactly 16 digits",
		},
		{
			name:       "contains letters",
			nik:        "317101010190000A",
			wantValid:  false,
			wantErrSub: "digits only",
		},
		{
			name:       "invalid province code",
			nik:        "9971010101900001",
			wantValid:  false,
			wantErrSub: "unknown or invalid province code",
		},
		{
			name:       "zero sequence",
			nik:        "3171010101900000",
			wantValid:  false,
			wantErrSub: "sequence number must be greater than 0000",
		},
		{
			name:       "invalid day zero",
			nik:        "3171010001900001",
			wantValid:  false,
			wantErrSub: "invalid day of birth in NIK: 0",
		},
		{
			name:       "invalid male day > 31",
			nik:        "3171013501900001",
			wantValid:  false,
			wantErrSub: "invalid day of birth in NIK: 35",
		},
		{
			name:       "invalid female day > 71",
			nik:        "3171017501900001",
			wantValid:  false,
			wantErrSub: "invalid day of birth in NIK: 35",
		},
		{
			name:       "invalid month 00",
			nik:        "3171010100900001",
			wantValid:  false,
			wantErrSub: "invalid month in NIK: 0",
		},
		{
			name:       "invalid month 13",
			nik:        "3171010113900001",
			wantValid:  false,
			wantErrSub: "invalid month in NIK: 13",
		},
		{
			name:       "day 31 in 30-day month April",
			nik:        "3171013104900001",
			wantValid:  false,
			wantErrSub: "exceeds max days (30) for month 4",
		},
		{
			name:       "day 30 in February",
			nik:        "3171013002900001",
			wantValid:  false,
			wantErrSub: "exceeds max days (29) for month 2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := ocr.ValidateNIK(tc.nik)
			if res.IsValid != tc.wantValid {
				t.Fatalf("expected isValid=%v, got %v (error: %s)", tc.wantValid, res.IsValid, res.Error)
			}
			if tc.wantValid {
				if res.IsFemale != tc.wantFemale {
					t.Errorf("expected isFemale=%v, got %v", tc.wantFemale, res.IsFemale)
				}
				if res.BirthDay != tc.wantDay {
					t.Errorf("expected BirthDay=%d, got %d", tc.wantDay, res.BirthDay)
				}
				if res.BirthMonth != tc.wantMonth {
					t.Errorf("expected BirthMonth=%d, got %d", tc.wantMonth, res.BirthMonth)
				}
			} else if tc.wantErrSub != "" {
				if res.Error == "" {
					t.Errorf("expected error containing %q, got empty error", tc.wantErrSub)
				}
			}
		})
	}
}

func TestValidateKTP(t *testing.T) {
	t.Run("nil data", func(t *testing.T) {
		_, conf, issues := ocr.ValidateKTP(nil)
		if conf != 0.0 || len(issues) == 0 {
			t.Fatalf("expected 0 confidence and issues for nil data, got conf=%f, issues=%v", conf, issues)
		}
	})

	t.Run("perfect valid synthetic male KTP", func(t *testing.T) {
		ktp := &ocr.KTPData{
			NIK:              "3171010101900001",
			Nama:             "BUDI SANTOSO",
			TempatLahir:      "JAKARTA",
			TanggalLahir:     "1990-01-01",
			JenisKelamin:     "LAKI-LAKI",
			Alamat:           "JL. MERDEKA NO. 10",
			RTRW:             "001/002",
			Kelurahan:        "GAMBIR",
			Kecamatan:        "GAMBIR",
			Agama:            "ISLAM",
			StatusPerkawinan: "KAWIN",
			Pekerjaan:        "KARYAWAN SWASTA",
			Kewarganegaraan:  "WNI",
		}

		normalized, conf, issues := ocr.ValidateKTP(ktp)
		if conf < 0.95 {
			t.Fatalf("expected high confidence >= 0.95, got %f with issues: %v", conf, issues)
		}
		if len(issues) != 0 {
			t.Errorf("expected zero issues, got %v", issues)
		}
		if normalized.Nama != "BUDI SANTOSO" {
			t.Errorf("expected uppercase normalized name, got %s", normalized.Nama)
		}
	})

	t.Run("female KTP with cross validation", func(t *testing.T) {
		ktp := &ocr.KTPData{
			NIK:              "3201015505950002",
			Nama:             "SITI AMINAH",
			TempatLahir:      "BOGOR",
			TanggalLahir:     "1995-05-15",
			JenisKelamin:     "PEREMPUAN",
			Alamat:           "JL. RAYA PAJAJARAN NO. 5",
			RTRW:             "002/003",
			Kelurahan:        "TEGAL GUNDIL",
			Kecamatan:        "BOGOR UTARA",
			Agama:            "ISLAM",
			StatusPerkawinan: "BELUM KAWIN",
			Pekerjaan:        "MAHASISWA",
			Kewarganegaraan:  "INDONESIA",
		}

		normalized, conf, issues := ocr.ValidateKTP(ktp)
		if conf < 0.95 {
			t.Fatalf("expected high confidence >= 0.95, got %f with issues: %v", conf, issues)
		}
		if normalized.Kewarganegaraan != "WNI" {
			t.Errorf("expected citizenship defaulted to WNI, got %s", normalized.Kewarganegaraan)
		}
	})

	t.Run("cross check mismatch DOB and gender", func(t *testing.T) {
		ktp := &ocr.KTPData{
			NIK:              "3171010101900001", // Male, 1990-01-01
			Nama:             "JANE DOE",
			TempatLahir:      "JAKARTA",
			TanggalLahir:     "1995-12-25", // Mismatch with NIK
			JenisKelamin:     "PEREMPUAN",  // Mismatch with male NIK
			Alamat:           "JL. MERDEKA NO. 10",
			RTRW:             "001/002",
			Kelurahan:        "GAMBIR",
			Kecamatan:        "GAMBIR",
			Agama:            "KRISTEN",
			StatusPerkawinan: "BELUM KAWIN",
			Pekerjaan:        "KARYAWAN",
			Kewarganegaraan:  "WNI",
		}

		_, conf, issues := ocr.ValidateKTP(ktp)
		if conf > 0.85 {
			t.Fatalf("expected lower confidence due to cross-check mismatches, got %f", conf)
		}
		hasDOBMismatch := false
		hasGenderMismatch := false
		for _, issue := range issues {
			if issue == "nik: birth date mismatch with tanggal_lahir" {
				hasDOBMismatch = true
			}
			if issue == "nik: gender encoding mismatch with jenis_kelamin" {
				hasGenderMismatch = true
			}
		}
		if !hasDOBMismatch || !hasGenderMismatch {
			t.Errorf("expected cross check issues, got %v", issues)
		}
	})

	t.Run("missing fields and invalid enums", func(t *testing.T) {
		ktp := &ocr.KTPData{
			NIK:              "1234",
			Nama:             "A",
			TempatLahir:      "",
			TanggalLahir:     "invalid-date",
			JenisKelamin:     "UNKNOWN",
			Alamat:           "",
			RTRW:             "",
			Kelurahan:        "",
			Kecamatan:        "",
			Agama:            "UNKNOWN",
			StatusPerkawinan: "UNKNOWN",
			Pekerjaan:        "",
			Kewarganegaraan:  "XYZ",
		}

		_, conf, issues := ocr.ValidateKTP(ktp)
		if conf >= 0.50 {
			t.Fatalf("expected low confidence < 0.50 for highly invalid data, got %f", conf)
		}
		if len(issues) < 6 {
			t.Errorf("expected many issues, got %v", issues)
		}
	})
}

func BenchmarkValidateNIK(b *testing.B) {
	nik := "3171010101900001"
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		res := ocr.ValidateNIK(nik)
		if !res.IsValid {
			b.Fatal("expected valid NIK")
		}
	}
}

func BenchmarkValidateKTP(b *testing.B) {
	ktp := &ocr.KTPData{
		NIK:              "3171010101900001",
		Nama:             "BUDI SANTOSO",
		TempatLahir:      "JAKARTA",
		TanggalLahir:     "1990-01-01",
		JenisKelamin:     "LAKI-LAKI",
		Alamat:           "JL. MERDEKA NO. 10",
		RTRW:             "001/002",
		Kelurahan:        "GAMBIR",
		Kecamatan:        "GAMBIR",
		Agama:            "ISLAM",
		StatusPerkawinan: "KAWIN",
		Pekerjaan:        "KARYAWAN SWASTA",
		Kewarganegaraan:  "WNI",
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, conf, _ := ocr.ValidateKTP(ktp)
		if conf == 0 {
			b.Fatal("unexpected zero confidence")
		}
	}
}
