package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amirfaisalz/lensio/tests/fixtures/synthetic"
)

func TestCalculateDriverAge(t *testing.T) {
	refTime := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		dob         string
		expectedAge int
		wantErr     bool
	}{
		{
			name:        "Valid DOB in 1995",
			dob:         "1995-03-20",
			expectedAge: 31,
			wantErr:     false,
		},
		{
			name:        "Valid DOB in 2005 (21 years old)",
			dob:         "2005-08-01",
			expectedAge: 21,
			wantErr:     false,
		},
		{
			name:        "Invalid Date Format",
			dob:         "20-03-1995",
			expectedAge: 0,
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			age, err := CalculateDriverAge(tc.dob, refTime)
			if (err != nil) != tc.wantErr {
				t.Fatalf("CalculateDriverAge() err = %v, wantErr = %v", err, tc.wantErr)
			}
			if !tc.wantErr && age != tc.expectedAge {
				t.Errorf("CalculateDriverAge() = %d, want %d", age, tc.expectedAge)
			}
		})
	}
}

func TestVerifyDriverEligibility_Approved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test_key_rentease" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"invalid_api_key"}}`))
			return
		}

		resp := LensioResponse{
			ID:           "ocr_test_re_01",
			Status:       "completed",
			DocumentType: "ktp",
			Confidence:   0.94,
		}
		resp.Data.NIK = "3171012345670001"
		resp.Data.Nama = "SITI NURHALIZA"
		resp.Data.TanggalLahir = "1998-04-15" // 28 years old in 2026
		resp.Data.Kewarganegaraan = "WNI"

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		APIURL:       server.URL,
		APIKey:       "test_key_rentease",
		ImagePath:    "valid_ktp.jpg",
		MinDriverAge: 21,
		RequireWNI:   true,
		VehicleClass: "SEDAN",
		Timeout:      2 * time.Second,
	}

	img := synthetic.GenerateValidKTPImage()
	refTime := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)

	res, err := VerifyDriverEligibility(context.Background(), cfg, img, refTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "APPROVED" {
		t.Errorf("expected APPROVED, got %s (reason: %s)", res.Status, res.RejectionReason)
	}
	if res.DriverAge != 28 {
		t.Errorf("expected age 28, got %d", res.DriverAge)
	}
	if res.PassID == "" {
		t.Errorf("expected non-empty rental pass ID")
	}
}

func TestVerifyDriverEligibility_Underage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := LensioResponse{
			ID:           "ocr_test_re_02",
			Status:       "completed",
			DocumentType: "ktp",
			Confidence:   0.95,
		}
		resp.Data.Nama = "TEENAGER DRIVER"
		resp.Data.TanggalLahir = "2007-06-10" // 19 years old in 2026 (< 21)
		resp.Data.Kewarganegaraan = "WNI"

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		APIURL:       server.URL,
		APIKey:       "test_key_rentease",
		MinDriverAge: 21,
		VehicleClass: "SUV",
		Timeout:      2 * time.Second,
	}

	img := synthetic.GenerateValidKTPImage()
	refTime := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)

	res, err := VerifyDriverEligibility(context.Background(), cfg, img, refTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "REJECTED" {
		t.Errorf("expected REJECTED, got %s", res.Status)
	}
}

func TestVerifyDriverEligibility_NonWNIRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := LensioResponse{
			ID:           "ocr_test_re_03",
			Status:       "completed",
			DocumentType: "ktp",
			Confidence:   0.90,
		}
		resp.Data.Nama = "JOHN DOE"
		resp.Data.TanggalLahir = "1990-01-01"
		resp.Data.Kewarganegaraan = "WNA"

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		APIURL:       server.URL,
		APIKey:       "test_key_rentease",
		MinDriverAge: 21,
		RequireWNI:   true,
		VehicleClass: "VAN",
		Timeout:      2 * time.Second,
	}

	img := synthetic.GenerateValidKTPImage()
	refTime := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)

	res, err := VerifyDriverEligibility(context.Background(), cfg, img, refTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "REJECTED" {
		t.Errorf("expected REJECTED for WNA with RequireWNI=true, got %s", res.Status)
	}
}

func TestVerifyDriverEligibility_LowConfidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := LensioResponse{
			ID:           "ocr_test_re_04",
			Status:       "completed",
			DocumentType: "ktp",
			Confidence:   0.55, // below 0.70 threshold
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		APIURL:  server.URL,
		APIKey:  "test_key_rentease",
		Timeout: 2 * time.Second,
	}

	img := synthetic.GenerateValidKTPImage()
	res, err := VerifyDriverEligibility(context.Background(), cfg, img, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "REJECTED" {
		t.Errorf("expected REJECTED for low confidence, got %s", res.Status)
	}
}

func TestVerifyDriverEligibility_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":"ocr_failed","message":"OCR provider timeout"}}`))
	}))
	defer server.Close()

	cfg := Config{
		APIURL:  server.URL,
		APIKey:  "test_key_rentease",
		Timeout: 2 * time.Second,
	}

	img := synthetic.GenerateValidKTPImage()
	_, err := VerifyDriverEligibility(context.Background(), cfg, img, time.Now())
	if err == nil {
		t.Fatal("expected error on HTTP 503, got nil")
	}
}

func TestVerifyDriverEligibility_InvalidDOB(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := LensioResponse{
			ID:           "ocr_test_re_05",
			Status:       "completed",
			DocumentType: "ktp",
			Confidence:   0.90,
		}
		resp.Data.TanggalLahir = "bad-date-format"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		APIURL:  server.URL,
		APIKey:  "test_key_rentease",
		Timeout: 2 * time.Second,
	}

	img := synthetic.GenerateValidKTPImage()
	_, err := VerifyDriverEligibility(context.Background(), cfg, img, time.Now())
	if err == nil {
		t.Fatal("expected error on invalid DOB, got nil")
	}
}
