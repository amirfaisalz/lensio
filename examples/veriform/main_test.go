package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amirfaisalz/nusaid/tests/fixtures/synthetic"
)

func TestCalculateAge(t *testing.T) {
	refTime := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		dob         string
		expectedAge int
		wantErr     bool
	}{
		{
			name:        "Born in 1990 before September",
			dob:         "1990-05-12",
			expectedAge: 36,
			wantErr:     false,
		},
		{
			name:        "Born in 2008 before September (18 years old)",
			dob:         "2008-01-01",
			expectedAge: 18,
			wantErr:     false,
		},
		{
			name:        "Born in 2008 in December (17 years old)",
			dob:         "2008-12-25",
			expectedAge: 17,
			wantErr:     false,
		},
		{
			name:        "Invalid date format",
			dob:         "12-05-1990",
			expectedAge: 0,
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			age, err := CalculateAge(tc.dob, refTime)
			if (err != nil) != tc.wantErr {
				t.Fatalf("CalculateAge(%s) error = %v, wantErr = %v", tc.dob, err, tc.wantErr)
			}
			if !tc.wantErr && age != tc.expectedAge {
				t.Errorf("CalculateAge(%s) = %d, want %d", tc.dob, age, tc.expectedAge)
			}
		})
	}
}

func TestRunOnboarding_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test_key_123" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"invalid_api_key","message":"Invalid key"}}`))
			return
		}

		resp := NusaIDResponse{
			ID:           "ocr_test_01",
			Status:       "completed",
			DocumentType: "ktp",
			Confidence:   0.98,
		}
		resp.Data.NIK = "3171012345670001"
		resp.Data.Nama = "BUDI SANTOSO"
		resp.Data.TanggalLahir = "1992-08-17"
		resp.Processing.LatencyMS = 1200

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		APIURL:   server.URL,
		APIKey:   "test_key_123",
		ImageURL: "valid_ktp.jpg",
		MinAge:   17,
		Timeout:  2 * time.Second,
	}

	img := synthetic.GenerateValidKTPImage()
	refTime := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)

	res, err := RunOnboarding(context.Background(), cfg, img, refTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "APPROVED" {
		t.Errorf("expected APPROVED, got %s (reason: %s)", res.Status, res.Reason)
	}
	if res.FullName != "BUDI SANTOSO" {
		t.Errorf("expected BUDI SANTOSO, got %s", res.FullName)
	}
	if res.Age != 34 {
		t.Errorf("expected age 34, got %d", res.Age)
	}
}

func TestRunOnboarding_UnderageRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := NusaIDResponse{
			ID:           "ocr_test_02",
			Status:       "completed",
			DocumentType: "ktp",
			Confidence:   0.95,
		}
		resp.Data.NIK = "3171012345670002"
		resp.Data.Nama = "ANANDA PRATAMA"
		resp.Data.TanggalLahir = "2012-05-10" // 14 years old in 2026

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		APIURL:  server.URL,
		APIKey:  "test_key_123",
		MinAge:  17,
		Timeout: 2 * time.Second,
	}

	img := synthetic.GenerateValidKTPImage()
	refTime := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)

	res, err := RunOnboarding(context.Background(), cfg, img, refTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "REJECTED" {
		t.Errorf("expected REJECTED, got %s", res.Status)
	}
	if res.Age != 14 {
		t.Errorf("expected age 14, got %d", res.Age)
	}
}

func TestRunOnboarding_LowConfidenceRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := NusaIDResponse{
			ID:           "ocr_test_03",
			Status:       "completed",
			DocumentType: "ktp",
			Confidence:   0.62, // Below 0.75 threshold
		}
		resp.Data.NIK = "3171012345670003"
		resp.Data.Nama = "BLURRED USER"
		resp.Data.TanggalLahir = "1990-01-01"

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		APIURL:  server.URL,
		APIKey:  "test_key_123",
		MinAge:  17,
		Timeout: 2 * time.Second,
	}

	img := synthetic.GenerateValidKTPImage()
	refTime := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)

	res, err := RunOnboarding(context.Background(), cfg, img, refTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "REJECTED" {
		t.Errorf("expected REJECTED, got %s", res.Status)
	}
}

func TestRunOnboarding_APIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"rate_limit_exceeded","message":"Rate limit reached"}}`))
	}))
	defer server.Close()

	cfg := Config{
		APIURL:  server.URL,
		APIKey:  "test_key_123",
		Timeout: 2 * time.Second,
	}

	img := synthetic.GenerateValidKTPImage()
	_, err := RunOnboarding(context.Background(), cfg, img, time.Now())
	if err == nil {
		t.Fatal("expected error on HTTP 429, got nil")
	}
}

func TestRunOnboarding_InvalidDOB(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := NusaIDResponse{
			ID:           "ocr_test_04",
			Status:       "completed",
			DocumentType: "ktp",
			Confidence:   0.90,
		}
		resp.Data.TanggalLahir = "invalid-date"

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		APIURL:  server.URL,
		APIKey:  "test_key_123",
		Timeout: 2 * time.Second,
	}

	img := synthetic.GenerateValidKTPImage()
	_, err := RunOnboarding(context.Background(), cfg, img, time.Now())
	if err == nil {
		t.Fatal("expected error on invalid DOB, got nil")
	}
}
