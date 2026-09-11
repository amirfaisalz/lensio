package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Config encapsulates runtime parameters for VeriForm identity onboarding.
type Config struct {
	APIURL   string
	APIKey   string
	ImageURL string
	MinAge   int
	Timeout  time.Duration
}

// LensioResponse matches the OCR response envelope returned by Lensio API.
type LensioResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	DocumentType string `json:"document_type"`
	Confidence   float64 `json:"confidence"`
	Data         struct {
		NIK              string `json:"nik"`
		Nama             string `json:"nama"`
		TempatLahir      string `json:"tempat_lahir"`
		TanggalLahir     string `json:"tanggal_lahir"`
		JenisKelamin     string `json:"jenis_kelamin"`
		Alamat           string `json:"alamat"`
		RTRW             string `json:"rt_rw"`
		Kelurahan        string `json:"kelurahan"`
		Kecamatan        string `json:"kecamatan"`
		Agama            string `json:"agama"`
		StatusPerkawinan string `json:"status_perkawinan"`
		Pekerjaan        string `json:"pekerjaan"`
		Kewarganegaraan  string `json:"kewarganegaraan"`
	} `json:"data"`
	Processing struct {
		LatencyMS int64 `json:"latency_ms"`
	} `json:"processing"`
	Error *struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error,omitempty"`
}

// OnboardingResult represents VeriForm's business onboarding outcome.
type OnboardingResult struct {
	ApplicantID string    `json:"applicant_id"`
	Status      string    `json:"status"` // APPROVED or REJECTED
	Reason      string    `json:"reason,omitempty"`
	FullName    string    `json:"full_name"`
	NIK         string    `json:"nik"`
	Age         int       `json:"age"`
	Confidence  float64   `json:"ocr_confidence"`
	ProcessedAt time.Time `json:"processed_at"`
}

// SubmitKTP sends an in-memory image to the Lensio OCR endpoint.
func SubmitKTP(ctx context.Context, client *http.Client, apiURL, apiKey, filename string, imageBytes []byte) (*LensioResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("document", filepath.Base(filename))
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart form file: %w", err)
	}

	if _, err := part.Write(imageBytes); err != nil {
		return nil, fmt.Errorf("failed to write image to form file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	reqURL := fmt.Sprintf("%s/api/v1/ocr/ktp", apiURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, &body)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lensio api request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var lensioResp LensioResponse
	if err := json.Unmarshal(respBody, &lensioResp); err != nil {
		return nil, fmt.Errorf("failed to parse json response (status %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		if lensioResp.Error != nil {
			return nil, fmt.Errorf("lensio error (%s): %s", lensioResp.Error.Code, lensioResp.Error.Message)
		}
		return nil, fmt.Errorf("lensio returned non-200 status: %d", resp.StatusCode)
	}

	return &lensioResp, nil
}

// CalculateAge computes the integer age in years from YYYY-MM-DD.
func CalculateAge(dobStr string, referenceTime time.Time) (int, error) {
	dob, err := time.Parse("2006-01-02", dobStr)
	if err != nil {
		return 0, fmt.Errorf("invalid date of birth format (%s): %w", dobStr, err)
	}

	years := referenceTime.Year() - dob.Year()
	if referenceTime.YearDay() < dob.YearDay() {
		years--
	}
	return years, nil
}

// RunOnboarding executes the full VeriForm identity verification and onboarding flow.
func RunOnboarding(ctx context.Context, cfg Config, imageBytes []byte, refTime time.Time) (*OnboardingResult, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	client := &http.Client{Timeout: cfg.Timeout}

	ocrResp, err := SubmitKTP(ctx, client, cfg.APIURL, cfg.APIKey, cfg.ImageURL, imageBytes)
	if err != nil {
		return nil, err
	}

	// 1. Confidence Gate
	if ocrResp.Confidence < 0.75 {
		return &OnboardingResult{
			ApplicantID: fmt.Sprintf("vf_usr_%s", ocrResp.ID),
			Status:      "REJECTED",
			Reason:      fmt.Sprintf("OCR confidence too low (%.2f < 0.75)", ocrResp.Confidence),
			FullName:    ocrResp.Data.Nama,
			NIK:         ocrResp.Data.NIK,
			Confidence:  ocrResp.Confidence,
			ProcessedAt: time.Now().UTC(),
		}, nil
	}

	// 2. Age Gate
	age, err := CalculateAge(ocrResp.Data.TanggalLahir, refTime)
	if err != nil {
		return nil, fmt.Errorf("age validation error: %w", err)
	}

	if age < cfg.MinAge {
		return &OnboardingResult{
			ApplicantID: fmt.Sprintf("vf_usr_%s", ocrResp.ID),
			Status:      "REJECTED",
			Reason:      fmt.Sprintf("Applicant does not meet minimum age requirement (%d < %d)", age, cfg.MinAge),
			FullName:    ocrResp.Data.Nama,
			NIK:         ocrResp.Data.NIK,
			Age:         age,
			Confidence:  ocrResp.Confidence,
			ProcessedAt: time.Now().UTC(),
		}, nil
	}

	// 3. Approved
	return &OnboardingResult{
		ApplicantID: fmt.Sprintf("vf_usr_%s", ocrResp.ID),
		Status:      "APPROVED",
		FullName:    ocrResp.Data.Nama,
		NIK:         ocrResp.Data.NIK,
		Age:         age,
		Confidence:  ocrResp.Confidence,
		ProcessedAt: time.Now().UTC(),
	}, nil
}

func main() {
	apiURL := flag.String("api-url", "http://localhost:8080", "Lensio base API URL")
	apiKey := flag.String("api-key", os.Getenv("LENSIO_API_KEY"), "Lensio API key (Bearer token)")
	imagePath := flag.String("image", "tests/fixtures/synthetic/valid_ktp.jpg", "Path to synthetic KTP image file")
	minAge := flag.Int("min-age", 17, "Minimum required age for registration")
	flag.Parse()

	if *apiKey == "" {
		fmt.Fprintf(os.Stderr, "Error: --api-key or LENSIO_API_KEY environment variable is required\n")
		os.Exit(1)
	}

	data, err := os.ReadFile(*imagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading image file %q: %v\n", *imagePath, err)
		os.Exit(1)
	}

	cfg := Config{
		APIURL:   *apiURL,
		APIKey:   *apiKey,
		ImageURL: *imagePath,
		MinAge:   *minAge,
		Timeout:  10 * time.Second,
	}

	fmt.Printf("🔍 VeriForm Identity Onboarding Client\n")
	fmt.Printf("Connecting to Lensio at %s...\n", cfg.APIURL)

	res, err := RunOnboarding(context.Background(), cfg, data, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Verification failed: %v\n", err)
		os.Exit(1)
	}

	output, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(output))

	if res.Status == "APPROVED" {
		fmt.Printf("✅ Applicant %s successfully onboarded (ID: %s)\n", res.FullName, res.ApplicantID)
		os.Exit(0)
	} else {
		fmt.Printf("⛔ Onboarding rejected: %s\n", res.Reason)
		os.Exit(2)
	}
}
