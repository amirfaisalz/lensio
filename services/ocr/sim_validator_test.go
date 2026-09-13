package ocr_test

import (
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/services/ocr"
)

func TestValidateSIMNumber(t *testing.T) {
	tests := []struct {
		name       string
		nomor      string
		wantValid  bool
		wantClean  string
		wantErrSub string
	}{
		{
			name:      "valid 12 digit SIM",
			nomor:     "123456789012",
			wantValid: true,
			wantClean: "123456789012",
		},
		{
			name:      "valid formatted with hyphens",
			nomor:     "1234-5678-9012",
			wantValid: true,
			wantClean: "123456789012",
		},
		{
			name:      "valid formatted with spaces",
			nomor:     "1234 5678 9012",
			wantValid: true,
			wantClean: "123456789012",
		},
		{
			name:      "valid 14 digit older format",
			nomor:     "12345678901234",
			wantValid: true,
			wantClean: "12345678901234",
		},
		{
			name:      "valid 16 digit NIK aligned format",
			nomor:     "3171010101900001",
			wantValid: true,
			wantClean: "3171010101900001",
		},
		{
			name:      "valid with OCR confusion characters (O/I)",
			nomor:     "1234O6789OI2",
			wantValid: true,
			wantClean: "123406789012",
		},
		{
			name:       "too short",
			nomor:      "12345678901",
			wantValid:  false,
			wantErrSub: "length must be between 12 and 16 digits",
		},
		{
			name:       "too long",
			nomor:      "12345678901234567",
			wantValid:  false,
			wantErrSub: "length must be between 12 and 16 digits",
		},
		{
			name:       "contains invalid characters",
			nomor:      "12345678901X",
			wantValid:  false,
			wantErrSub: "digits only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ocr.ValidateSIMNumber(tt.nomor)
			if res.IsValid != tt.wantValid {
				t.Fatalf("expected isValid=%v, got=%v (error=%s)", tt.wantValid, res.IsValid, res.Error)
			}
			if tt.wantValid && res.Cleaned != tt.wantClean {
				t.Errorf("expected cleaned=%q, got=%q", tt.wantClean, res.Cleaned)
			}
			if !tt.wantValid && tt.wantErrSub != "" {
				if res.Error == "" {
					t.Errorf("expected error containing %q, got empty", tt.wantErrSub)
				}
			}
		})
	}
}

func TestValidateSIM(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		res, score, issues := ocr.ValidateSIM(nil)
		if res == nil || score != 0.0 || len(issues) == 0 {
			t.Fatalf("expected zero score and issues for nil input")
		}
	})

	t.Run("valid complete SIM record", func(t *testing.T) {
		sim := &ocr.SIMData{
			NomorSIM:      "1234-5678-9012",
			Golongan:      "SIM A",
			Nama:          "Budi Santoso",
			TempatLahir:   "Jakarta",
			TanggalLahir:  "1990-01-01",
			GolonganDarah: "O",
			JenisKelamin:  "PRIA",
			Alamat:        "Jl. Sudirman No. 45",
			Pekerjaan:     "Karyawan Swasta",
			Polda:         "POLDA METRO JAYA",
			MasaBerlaku:   "2029-01-01",
		}

		normalized, score, issues := ocr.ValidateSIM(sim)
		if len(issues) > 0 {
			t.Errorf("unexpected issues: %v", issues)
		}
		if score < 0.95 {
			t.Errorf("expected score >= 0.95, got %f", score)
		}
		if normalized.NomorSIM != "123456789012" {
			t.Errorf("expected cleaned nomor_sim, got %s", normalized.NomorSIM)
		}
		if normalized.Golongan != "A" {
			t.Errorf("expected normalized golongan 'A', got %s", normalized.Golongan)
		}
		if normalized.Polda != "POLDA METRO JAYA" {
			t.Errorf("expected normalized polda, got %s", normalized.Polda)
		}
	})

	t.Run("golongan variants normalization", func(t *testing.T) {
		variants := []struct {
			input string
			want  string
		}{
			{"C1", "C I"},
			{"C2", "C II"},
			{"B1", "B I"},
			{"B2", "B II"},
			{"D1", "D I"},
			{"SIM C", "C"},
			{"A UMUM", "A UMUM"},
			{"B I UMUM", "B I UMUM"},
			{"B II UMUM", "B II UMUM"},
			{"INTERNASIONAL", "INTERNASIONAL"},
		}

		for _, v := range variants {
			sim := &ocr.SIMData{
				NomorSIM:      "123456789012",
				Golongan:      v.input,
				Nama:          "Joko Susilo",
				TempatLahir:   "Surabaya",
				TanggalLahir:  "1995-05-15",
				GolonganDarah: "B",
				JenisKelamin:  "LAKI-LAKI",
				Alamat:        "Jl. Pemuda No. 1",
				Pekerjaan:     "Swasta",
				Polda:         "JAWA TIMUR",
				MasaBerlaku:   "2028-05-15",
			}

			norm, score, issues := ocr.ValidateSIM(sim)
			if norm.Golongan != v.want {
				t.Errorf("input %s: expected %s, got %s", v.input, v.want, norm.Golongan)
			}
			if norm.JenisKelamin != "PRIA" {
				t.Errorf("expected normalized gender PRIA, got %s", norm.JenisKelamin)
			}
			if score < 0.95 {
				t.Errorf("expected high score for %s, got %f (issues: %v)", v.input, score, issues)
			}
		}
	})

	t.Run("female gender normalization", func(t *testing.T) {
		genders := []string{"PEREMPUAN", "WANITA", "P", "W"}
		for _, g := range genders {
			sim := &ocr.SIMData{
				NomorSIM:      "987654321098",
				Golongan:      "C",
				Nama:          "Siti Aminah",
				TempatLahir:   "Bandung",
				TanggalLahir:  "1992-08-20",
				GolonganDarah: "AB",
				JenisKelamin:  g,
				Alamat:        "Jl. Asia Afrika No. 10",
				Pekerjaan:     "Wiraswasta",
				Polda:         "JAWA BARAT",
				MasaBerlaku:   "2028-08-20",
			}
			norm, _, _ := ocr.ValidateSIM(sim)
			if norm.JenisKelamin != "WANITA" {
				t.Errorf("gender %s: expected WANITA, got %s", g, norm.JenisKelamin)
			}
		}
	})

	t.Run("underage driver detection", func(t *testing.T) {
		// Under 17 years old
		tenYearsAgo := time.Now().AddDate(-10, 0, 0).Format("2006-01-02")
		sim := &ocr.SIMData{
			NomorSIM:      "123456789012",
			Golongan:      "A",
			Nama:          "Anak Kecil",
			TempatLahir:   "Jakarta",
			TanggalLahir:  tenYearsAgo,
			GolonganDarah: "A",
			JenisKelamin:  "PRIA",
			Alamat:        "Jl. Merdeka No. 1",
			Pekerjaan:     "Pelajar",
			Polda:         "METRO JAYA",
			MasaBerlaku:   "2028-01-01",
		}
		_, _, issues := ocr.ValidateSIM(sim)
		found := false
		for _, iss := range issues {
			if iss == "tanggal_lahir: driver age must be at least 17 years old" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected underage driver issue, got: %v", issues)
		}
	})

	t.Run("missing and invalid fields penalties", func(t *testing.T) {
		sim := &ocr.SIMData{
			NomorSIM:      "invalid",
			Golongan:      "INVALID_GOL",
			Nama:          "A", // too short
			TempatLahir:   "",
			TanggalLahir:  "invalid-date",
			GolonganDarah: "INVALID_BLOOD",
			JenisKelamin:  "UNKNOWN",
			Alamat:        "",
			Pekerjaan:     "",
			Polda:         "",
			MasaBerlaku:   "1990-01-01", // out of range (<2000)
		}

		_, score, issues := ocr.ValidateSIM(sim)
		if score > 0.20 {
			t.Errorf("expected very low score for invalid SIM, got %f", score)
		}
		if len(issues) < 6 {
			t.Errorf("expected multiple issues, got %d: %v", len(issues), issues)
		}
	})

	t.Run("empty blood type defaults to hyphen", func(t *testing.T) {
		sim := &ocr.SIMData{
			NomorSIM:      "123456789012",
			Golongan:      "A",
			Nama:          "Doni",
			TempatLahir:   "Medan",
			TanggalLahir:  "1985-03-10",
			GolonganDarah: "",
			JenisKelamin:  "PRIA",
			Alamat:        "Jl. Gatot Subroto",
			Pekerjaan:     "PNS",
			Polda:         "SUMATERA UTARA",
			MasaBerlaku:   "2027-03-10",
		}
		norm, score, _ := ocr.ValidateSIM(sim)
		if norm.GolonganDarah != "-" {
			t.Errorf("expected '-' for empty blood type, got %s", norm.GolonganDarah)
		}
		if score < 0.90 {
			t.Errorf("expected high score, got %f", score)
		}
	})
}

func BenchmarkValidateSIMNumber(b *testing.B) {
	nomor := "1234-5678-9012"
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		res := ocr.ValidateSIMNumber(nomor)
		if !res.IsValid {
			b.Fatal("expected valid SIM number")
		}
	}
}

func BenchmarkValidateSIM(b *testing.B) {
	sim := &ocr.SIMData{
		NomorSIM:      "1234-5678-9012",
		Golongan:      "SIM A",
		Nama:          "BUDI SANTOSO",
		TempatLahir:   "JAKARTA",
		TanggalLahir:  "1990-01-01",
		GolonganDarah: "O",
		JenisKelamin:  "PRIA",
		Alamat:        "JL. MERDEKA NO. 10",
		Pekerjaan:     "KARYAWAN SWASTA",
		Polda:         "METRO JAYA",
		MasaBerlaku:   "2029-01-01",
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, conf, _ := ocr.ValidateSIM(sim)
		if conf == 0 {
			b.Fatal("unexpected zero confidence")
		}
	}
}
