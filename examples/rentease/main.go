package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// Config holds runtime parameters for RentEase vehicle rental verification.
type Config struct {
	APIURL        string
	APIKey        string
	ImagePath     string
	MinDriverAge  int
	RequireWNI    bool
	VehicleClass  string
	Timeout       time.Duration
}

// NusaIDResponse matches NusaID OCR response structure.
type NusaIDResponse struct {
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

// RentalVerificationResult represents the rental verification decision.
type RentalVerificationResult struct {
	PassID          string    `json:"rental_pass_id,omitempty"`
	Status          string    `json:"status"` // APPROVED, REJECTED, or MANUAL_REVIEW
	DriverName      string    `json:"driver_name"`
	DriverAge       int       `json:"driver_age"`
	Citizenship     string    `json:"citizenship"`
	VehicleClass    string    `json:"vehicle_class"`
	RejectionReason string    `json:"rejection_reason,omitempty"`
	Confidence      float64   `json:"ocr_confidence"`
	IssuedAt        time.Time `json:"issued_at"`
	ExpiresAt       time.Time `json:"expires_at,omitempty"`
}

// SubmitKTP sends an image to NusaID OCR API.
func SubmitKTP(ctx context.Context, client *http.Client, apiURL, apiKey, filename string, imageBytes []byte) (*NusaIDResponse, error) {
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
		return nil, fmt.Errorf("nusaid api request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var nusaResp NusaIDResponse
	if err := json.Unmarshal(respBody, &nusaResp); err != nil {
		return nil, fmt.Errorf("failed to parse json response (status %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		if nusaResp.Error != nil {
			return nil, fmt.Errorf("nusaid error (%s): %s", nusaResp.Error.Code, nusaResp.Error.Message)
		}
		return nil, fmt.Errorf("nusaid returned non-200 status: %d", resp.StatusCode)
	}

	return &nusaResp, nil
}

// CalculateDriverAge calculates driver's age in full years from DOB.
func CalculateDriverAge(dobStr string, refTime time.Time) (int, error) {
	dob, err := time.Parse("2006-01-02", dobStr)
	if err != nil {
		return 0, fmt.Errorf("invalid date format %q: %w", dobStr, err)
	}

	years := refTime.Year() - dob.Year()
	if refTime.YearDay() < dob.YearDay() {
		years--
	}
	return years, nil
}

// VerifyDriverEligibility executes RentEase rental authorization business rules.
func VerifyDriverEligibility(ctx context.Context, cfg Config, imageBytes []byte, refTime time.Time) (*RentalVerificationResult, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	client := &http.Client{Timeout: cfg.Timeout}

	ocrResp, err := SubmitKTP(ctx, client, cfg.APIURL, cfg.APIKey, cfg.ImagePath, imageBytes)
	if err != nil {
		return nil, err
	}

	// Rule 1: Document confidence
	if ocrResp.Confidence < 0.70 {
		return &RentalVerificationResult{
			Status:          "REJECTED",
			RejectionReason: fmt.Sprintf("KTP image quality too low (confidence: %.2f < 0.70)", ocrResp.Confidence),
			DriverName:      ocrResp.Data.Nama,
			Confidence:      ocrResp.Confidence,
			IssuedAt:        refTime,
		}, nil
	}

	// Rule 2: Driver Age check (Rental cars require older age threshold, default: 21)
	age, err := CalculateDriverAge(ocrResp.Data.TanggalLahir, refTime)
	if err != nil {
		return nil, fmt.Errorf("driver age verification error: %w", err)
	}

	if age < cfg.MinDriverAge {
		return &RentalVerificationResult{
			Status:          "REJECTED",
			RejectionReason: fmt.Sprintf("Driver age %d does not meet minimum driving age requirement (%d)", age, cfg.MinDriverAge),
			DriverName:      ocrResp.Data.Nama,
			DriverAge:       age,
			Citizenship:     ocrResp.Data.Kewarganegaraan,
			VehicleClass:    cfg.VehicleClass,
			Confidence:      ocrResp.Confidence,
			IssuedAt:        refTime,
		}, nil
	}

	// Rule 3: Citizenship Check
	if cfg.RequireWNI && ocrResp.Data.Kewarganegaraan != "WNI" {
		return &RentalVerificationResult{
			Status:          "REJECTED",
			RejectionReason: fmt.Sprintf("Citizenship %s not eligible for domestic self-drive rental without international permit", ocrResp.Data.Kewarganegaraan),
			DriverName:      ocrResp.Data.Nama,
			DriverAge:       age,
			Citizenship:     ocrResp.Data.Kewarganegaraan,
			VehicleClass:    cfg.VehicleClass,
			Confidence:      ocrResp.Confidence,
			IssuedAt:        refTime,
		}, nil
	}

	// Rule 4: Issue Rental Pass
	hashInput := fmt.Sprintf("%s:%s:%d", ocrResp.Data.NIK, refTime.Format(time.RFC3339), age)
	h := sha256.Sum256([]byte(hashInput))
	passID := fmt.Sprintf("RENTEASE-PASS-%d-%s", refTime.Year(), hex.EncodeToString(h[:4]))

	return &RentalVerificationResult{
		PassID:       passID,
		Status:       "APPROVED",
		DriverName:   ocrResp.Data.Nama,
		DriverAge:    age,
		Citizenship:  ocrResp.Data.Kewarganegaraan,
		VehicleClass: cfg.VehicleClass,
		Confidence:   ocrResp.Confidence,
		IssuedAt:     refTime,
		ExpiresAt:    refTime.Add(48 * time.Hour), // Valid for 48h to complete pickup
	}, nil
}

func main() {
	apiURL := flag.String("api-url", "http://localhost:8080", "NusaID base API URL")
	apiKey := flag.String("api-key", os.Getenv("NUSAID_API_KEY"), "NusaID API key (Bearer token)")
	imagePath := flag.String("image", "tests/fixtures/synthetic/valid_ktp.jpg", "Path to synthetic KTP image file")
	minAge := flag.Int("min-age", 21, "Minimum driver age for vehicle rental")
	requireWNI := flag.Bool("require-wni", false, "Require Indonesian citizenship")
	vehicleClass := flag.String("vehicle-class", "SUV", "Requested vehicle class")
	flag.Parse()

	if *apiKey == "" {
		fmt.Fprintf(os.Stderr, "Error: --api-key or NUSAID_API_KEY environment variable is required\n")
		os.Exit(1)
	}

	data, err := os.ReadFile(*imagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading image file %q: %v\n", *imagePath, err)
		os.Exit(1)
	}

	cfg := Config{
		APIURL:       *apiURL,
		APIKey:       *apiKey,
		ImagePath:    *imagePath,
		MinDriverAge: *minAge,
		RequireWNI:   *requireWNI,
		VehicleClass: *vehicleClass,
		Timeout:      10 * time.Second,
	}

	fmt.Printf("🚗 RentEase Vehicle Rental Verification Client\n")
	fmt.Printf("Verifying driver credentials against NusaID API at %s...\n", cfg.APIURL)

	res, err := VerifyDriverEligibility(context.Background(), cfg, data, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Verification failed: %v\n", err)
		os.Exit(1)
	}

	output, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(output))

	if res.Status == "APPROVED" {
		fmt.Printf("✅ Rental Approved! Pass ID: %s (Driver: %s, Age: %d)\n", res.PassID, res.DriverName, res.DriverAge)
		os.Exit(0)
	} else {
		fmt.Printf("⛔ Rental Rejected: %s\n", res.RejectionReason)
		os.Exit(2)
	}
}
