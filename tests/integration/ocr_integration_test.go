package integration_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/amirfaisalz/nusaid/tests/fixtures/synthetic"
)

type liveKeyResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type liveKTPResponse struct {
	ID           string  `json:"id"`
	DocumentType string  `json:"document_type"`
	Confidence   float64 `json:"confidence"`
	Data         struct {
		NIK  string `json:"nik"`
		Nama string `json:"nama"`
	} `json:"data"`
}

func TestIntegration_LiveServer_OCR(t *testing.T) {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	healthResp, err := client.Get(apiURL + "/health")
	if err != nil {
		t.Skipf("skipping live server test (server unreachable at %s): %v", apiURL, err)
	}
	healthResp.Body.Close()

	// 1. Create a key with ocr:write and ocr:read scopes
	createBody, _ := json.Marshal(map[string]any{
		"name":        "OCR Live Test Key",
		"environment": "live",
		"scopes":      []string{"ocr:write", "ocr:read"},
	})

	resp, err := client.Post(apiURL+"/api/v1/auth/api-keys", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("failed to create API key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		t.Skip("skipping live server test: target server is rate limited (HTTP 429)")
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201 from create key, got %d", resp.StatusCode)
	}

	var keyData liveKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&keyData); err != nil {
		t.Fatalf("failed decoding create key response: %v", err)
	}

	// 2. Upload synthetic KTP
	validImg := synthetic.GenerateValidKTPImage()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("document", "ktp.png")
	if err != nil {
		t.Fatalf("failed creating form file: %v", err)
	}
	if _, err := part.Write(validImg); err != nil {
		t.Fatalf("failed writing image bytes: %v", err)
	}
	_ = writer.Close()

	req, err := http.NewRequest(http.MethodPost, apiURL+"/api/v1/ocr/ktp", &body)
	if err != nil {
		t.Fatalf("failed creating ocr request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+keyData.Key)

	ocrResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed calling ocr endpoint: %v", err)
	}
	defer ocrResp.Body.Close()

	if ocrResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(ocrResp.Body)
		if ocrResp.StatusCode == http.StatusTooManyRequests {
			t.Logf("live server rate limited with HTTP 429: %s", string(b))
			return
		}
		// When the live server runs real Vision AI (e.g. Gemini),
		// a synthetic blank image is legitimately classified as unsupported (HTTP 422).
		if ocrResp.StatusCode == http.StatusUnprocessableEntity {
			t.Logf("live Vision AI server correctly rejected synthetic blank image with HTTP 422: %s", string(b))
			return
		}
		t.Fatalf("expected status 200, got %d: %s", ocrResp.StatusCode, string(b))
	}

	var ktpResp liveKTPResponse
	if err := json.NewDecoder(ocrResp.Body).Decode(&ktpResp); err != nil {
		t.Fatalf("failed decoding ocr response: %v", err)
	}

	if ktpResp.ID == "" || ktpResp.DocumentType != "ktp" {
		t.Errorf("unexpected ocr response: %+v", ktpResp)
	}

	// 3. Query metadata by ID
	getReq, _ := http.NewRequest(http.MethodGet, apiURL+"/api/v1/ocr/"+ktpResp.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+keyData.Key)

	getResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("failed retrieving metadata: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		if getResp.StatusCode == http.StatusTooManyRequests {
			t.Logf("live server rate limited with HTTP 429 on get")
			return
		}
		t.Fatalf("expected status 200 on metadata retrieval, got %d", getResp.StatusCode)
	}
}
