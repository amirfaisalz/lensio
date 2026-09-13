package ocr

import (
	"testing"
)

func TestCleanNomorKK(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"formatted with spaces", "3171 0101 0120 0001", "3171010101200001"},
		{"formatted with hyphens", "3171-0101-0120-0001", "3171010101200001"},
		{"formatted with dots", "31.71.01.01.0120.0001", "3171010101200001"},
		{"letters and noise", "NO: 3171-0101#0120*0001", "3171010101200001"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanNomorKK(tt.input)
			if got != tt.expected {
				t.Errorf("CleanNomorKK(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestValidateNomorKK(t *testing.T) {
	tests := []struct {
		name      string
		nomorKK   string
		wantValid bool
		wantProv  string
	}{
		{"valid jakarta kk", "3171010101200001", true, "DKI JAKARTA"},
		{"valid jabar kk formatted", "32.73.01.15.0821.0002", true, "JAWA BARAT"},
		{"valid leap year feb 29", "3171012902200001", true, "DKI JAKARTA"},
		{"invalid short length", "31710101", false, ""},
		{"invalid long length", "3171010101200001000", false, ""},
		{"invalid province code", "9971010101200001", false, ""},
		{"invalid sequence 0000", "3171010101200000", false, "DKI JAKARTA"},
		{"invalid month 00", "3171010100200001", false, "DKI JAKARTA"},
		{"invalid month 13", "3171010113200001", false, "DKI JAKARTA"},
		{"invalid day 00", "3171010001200001", false, "DKI JAKARTA"},
		{"invalid day 32", "3171013201200001", false, "DKI JAKARTA"},
		{"invalid non-leap feb 29", "3171012902210001", false, "DKI JAKARTA"},
		{"invalid april 31", "3171013104200001", false, "DKI JAKARTA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, _, provName, errMsg := ValidateNomorKK(tt.nomorKK)
			if valid != tt.wantValid {
				t.Errorf("ValidateNomorKK(%q) valid = %v, want %v (err: %s)", tt.nomorKK, valid, tt.wantValid, errMsg)
			}
			if tt.wantValid && provName != tt.wantProv {
				t.Errorf("ValidateNomorKK(%q) provName = %q, want %q", tt.nomorKK, provName, tt.wantProv)
			}
		})
	}
}

func TestValidateKK(t *testing.T) {
	validMembers := []KKFamilyMember{
		{
			Nama:             "BUDI SANTOSO",
			NIK:              "3171010101900001",
			JenisKelamin:     "LAKI-LAKI",
			TempatLahir:      "JAKARTA",
			TanggalLahir:     "1990-01-01",
			Agama:            "ISLAM",
			Pendidikan:       "STRATA I",
			JenisPekerjaan:   "KARYAWAN SWASTA",
			StatusPerkawinan: "KAWIN",
			StatusHubungan:   "KEPALA KELUARGA",
		},
		{
			Nama:             "SITI AMINAH",
			NIK:              "3171014101920002",
			JenisKelamin:     "PEREMPUAN",
			TempatLahir:      "BANDUNG",
			TanggalLahir:     "1992-01-01",
			Agama:            "ISLAM",
			Pendidikan:       "STRATA I",
			JenisPekerjaan:   "IBU RUMAH TANGGA",
			StatusPerkawinan: "KAWIN",
			StatusHubungan:   "ISTRI",
		},
		{
			Nama:             "RUDI SANTOSO",
			NIK:              "3171011505150003",
			JenisKelamin:     "LAKI-LAKI",
			TempatLahir:      "JAKARTA",
			TanggalLahir:     "2015-05-15",
			Agama:            "ISLAM",
			Pendidikan:       "BELUM/TIDAK BEKERJA",
			JenisPekerjaan:   "PELAJAR/MAHASISWA",
			StatusPerkawinan: "BELUM KAWIN",
			StatusHubungan:   "ANAK",
		},
	}

	validKK := &KKData{
		NomorKK:            "3171010101200001",
		KepalaKeluarga:     "BUDI SANTOSO",
		Alamat:             "JL. SUDIRMAN NO. 12",
		RTRW:               "001/002",
		KodePos:            "12190",
		KelurahanDesa:      "SENAYAN",
		Kecamatan:          "KEBAYORAN BARU",
		KabupatenKota:      "JAKARTA SELATAN",
		Provinsi:           "DKI JAKARTA",
		TanggalDikeluarkan: "2020-01-01",
		AnggotaKeluarga:    validMembers,
	}

	t.Run("valid complete KK", func(t *testing.T) {
		res := ValidateKK(validKK)
		if !res.IsValid {
			t.Fatalf("expected valid KK, got errors: %v", res.Errors)
		}
		if res.TotalMembers != 3 {
			t.Errorf("TotalMembers = %d, want 3", res.TotalMembers)
		}
		if !res.HeadOfFamilyFound {
			t.Errorf("HeadOfFamilyFound = false, want true")
		}
		if res.ProvinceName != "DKI JAKARTA" {
			t.Errorf("ProvinceName = %q, want 'DKI JAKARTA'", res.ProvinceName)
		}
	})

	t.Run("nil KK data", func(t *testing.T) {
		res := ValidateKK(nil)
		if res.IsValid {
			t.Fatal("expected nil KK data to be invalid")
		}
	})

	t.Run("invalid nomor KK", func(t *testing.T) {
		invalid := *validKK
		invalid.NomorKK = "123"
		res := ValidateKK(&invalid)
		if res.IsValid {
			t.Fatal("expected invalid nomor KK to fail validation")
		}
	})

	t.Run("missing kepala keluarga header", func(t *testing.T) {
		invalid := *validKK
		invalid.KepalaKeluarga = ""
		res := ValidateKK(&invalid)
		if res.IsValid {
			t.Fatal("expected missing kepala keluarga header to fail validation")
		}
	})

	t.Run("missing alamat", func(t *testing.T) {
		invalid := *validKK
		invalid.Alamat = "   "
		res := ValidateKK(&invalid)
		if res.IsValid {
			t.Fatal("expected missing alamat to fail validation")
		}
	})

	t.Run("empty anggota keluarga", func(t *testing.T) {
		invalid := *validKK
		invalid.AnggotaKeluarga = nil
		res := ValidateKK(&invalid)
		if res.IsValid {
			t.Fatal("expected empty anggota keluarga to fail validation")
		}
	})

	t.Run("member with missing name", func(t *testing.T) {
		members := make([]KKFamilyMember, len(validMembers))
		copy(members, validMembers)
		members[0].Nama = ""
		invalid := *validKK
		invalid.AnggotaKeluarga = members
		res := ValidateKK(&invalid)
		if res.IsValid {
			t.Fatal("expected missing member name to fail validation")
		}
	})

	t.Run("member with invalid NIK", func(t *testing.T) {
		members := make([]KKFamilyMember, len(validMembers))
		copy(members, validMembers)
		members[1].NIK = "12345"
		invalid := *validKK
		invalid.AnggotaKeluarga = members
		res := ValidateKK(&invalid)
		if res.IsValid {
			t.Fatal("expected invalid member NIK to fail validation")
		}
	})

	t.Run("member with invalid gender", func(t *testing.T) {
		members := make([]KKFamilyMember, len(validMembers))
		copy(members, validMembers)
		members[0].JenisKelamin = "UNKNOWN"
		invalid := *validKK
		invalid.AnggotaKeluarga = members
		res := ValidateKK(&invalid)
		if res.IsValid {
			t.Fatal("expected invalid gender to fail validation")
		}
	})

	t.Run("member with invalid status hubungan", func(t *testing.T) {
		members := make([]KKFamilyMember, len(validMembers))
		copy(members, validMembers)
		members[2].StatusHubungan = "TETANGGA"
		invalid := *validKK
		invalid.AnggotaKeluarga = members
		res := ValidateKK(&invalid)
		if res.IsValid {
			t.Fatal("expected invalid status hubungan to fail validation")
		}
	})

	t.Run("multiple kepala keluarga in members list", func(t *testing.T) {
		members := make([]KKFamilyMember, len(validMembers))
		copy(members, validMembers)
		members[1].StatusHubungan = "KEPALA KELUARGA"
		invalid := *validKK
		invalid.AnggotaKeluarga = members
		res := ValidateKK(&invalid)
		if res.IsValid {
			t.Fatal("expected multiple kepala keluarga to fail validation")
		}
	})

	t.Run("no kepala keluarga in members list", func(t *testing.T) {
		members := make([]KKFamilyMember, len(validMembers))
		copy(members, validMembers)
		members[0].StatusHubungan = "FAMILI LAIN"
		invalid := *validKK
		invalid.AnggotaKeluarga = members
		res := ValidateKK(&invalid)
		if res.IsValid {
			t.Fatal("expected missing kepala keluarga to fail validation")
		}
	})

	t.Run("kepala keluarga header mismatch with member list", func(t *testing.T) {
		invalid := *validKK
		invalid.KepalaKeluarga = "AHMAD JOKO"
		res := ValidateKK(&invalid)
		if res.IsValid {
			t.Fatal("expected mismatched kepala keluarga name to fail validation")
		}
	})
}

// Big O Benchmarks
func BenchmarkCleanNomorKK(b *testing.B) {
	raw := "31.71.01-010120-0001"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CleanNomorKK(raw)
	}
}

func BenchmarkValidateNomorKK(b *testing.B) {
	nomorKK := "3171010101200001"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = ValidateNomorKK(nomorKK)
	}
}

func BenchmarkValidateKK(b *testing.B) {
	kk := &KKData{
		NomorKK:        "3171010101200001",
		KepalaKeluarga: "BUDI SANTOSO",
		Alamat:         "JL. SUDIRMAN NO. 12",
		AnggotaKeluarga: []KKFamilyMember{
			{
				Nama:           "BUDI SANTOSO",
				NIK:            "3171010101900001",
				JenisKelamin:   "LAKI-LAKI",
				StatusHubungan: "KEPALA KELUARGA",
			},
			{
				Nama:           "SITI AMINAH",
				NIK:            "3171014101920002",
				JenisKelamin:   "PEREMPUAN",
				StatusHubungan: "ISTRI",
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateKK(kk)
	}
}
