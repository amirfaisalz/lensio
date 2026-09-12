package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/amirfaisalz/lensio/services/ocr"
)

const (
	defaultGeminiBaseURL = "https://generativelanguage.googleapis.com"
	defaultGeminiModel   = "gemini-2.0-flash"
)

// GeminiOCREngine implements ocr.OCREngine using Google AI Studio's Gemini Flash Vision API.
type GeminiOCREngine struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// GeminiOption configures the GeminiOCREngine.
type GeminiOption func(*GeminiOCREngine)

// WithBaseURL sets a custom base URL (useful for testing with httptest.Server).
func WithBaseURL(url string) GeminiOption {
	return func(g *GeminiOCREngine) {
		g.baseURL = strings.TrimRight(url, "/")
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) GeminiOption {
	return func(g *GeminiOCREngine) {
		g.httpClient = client
	}
}

// NewGeminiEngine constructs an engine adapter for Google Gemini Flash Vision AI.
func NewGeminiEngine(apiKey string, model string, opts ...GeminiOption) *GeminiOCREngine {
	if model == "" {
		model = defaultGeminiModel
	}

	engine := &GeminiOCREngine{
		apiKey:  apiKey,
		model:   model,
		baseURL: defaultGeminiBaseURL,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(engine)
	}

	return engine
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inline_data,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiGenConfig struct {
	ResponseMimeType string  `json:"responseMimeType"`
	Temperature      float64 `json:"temperature"`
}

type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig geminiGenConfig `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

type geminiExtractedJSON struct {
	DocumentType     string   `json:"document_type"`
	Confidence       float64  `json:"confidence"`
	RawText          string   `json:"raw_text"`
	NIK              string   `json:"nik"`
	Nama             string   `json:"nama"`
	TempatLahir      string   `json:"tempat_lahir"`
	TanggalLahir     string   `json:"tanggal_lahir"`
	JenisKelamin     string   `json:"jenis_kelamin"`
	Alamat           string   `json:"alamat"`
	RTRW             string   `json:"rt_rw"`
	Kelurahan        string   `json:"kelurahan"`
	Kecamatan        string   `json:"kecamatan"`
	Agama            string   `json:"agama"`
	StatusPerkawinan string   `json:"status_perkawinan"`
	Pekerjaan        string   `json:"pekerjaan"`
	Kewarganegaraan  string   `json:"kewarganegaraan"`
}

const geminiKTPPrompt = `You are a strict, high-accuracy Indonesian KTP OCR extraction engine.
Analyze the provided image.
First determine if the image is an Indonesian Kartu Tanda Penduduk (KTP).
If it is NOT an Indonesian KTP, output JSON with "document_type": "unsupported".
If it IS an Indonesian KTP, extract all visible fields into this exact JSON structure:
{
  "document_type": "ktp",
  "confidence": 0.95,
  "raw_text": "all raw text recognized on the card",
  "nik": "16 digit NIK without spaces",
  "nama": "FULL NAME",
  "tempat_lahir": "BIRTH PLACE",
  "tanggal_lahir": "YYYY-MM-DD",
  "jenis_kelamin": "LAKI-LAKI or PEREMPUAN",
  "alamat": "STREET ADDRESS",
  "rt_rw": "000/000",
  "kelurahan": "KELURAHAN/DESA",
  "kecamatan": "KECAMATAN",
  "agama": "RELIGION",
  "status_perkawinan": "MARITAL STATUS",
  "pekerjaan": "OCCUPATION",
  "kewarganegaraan": "WNI or WNA"
}`

// Extract invokes Gemini 2.0 Flash / 1.5 Flash Vision API with inline image bytes.
func (g *GeminiOCREngine) Extract(ctx context.Context, imageBytes []byte) (*ocr.OCRResult, error) {
	if g.apiKey == "" {
		return nil, fmt.Errorf("%w: gemini API key not configured", ocr.ErrOCRFailed)
	}

	mimeType := ocr.DetectMIMEType(imageBytes)
	if mimeType == "" {
		mimeType = ocr.MIMEJPEG
	}

	base64Image := base64.StdEncoding.EncodeToString(imageBytes)

	reqPayload := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: geminiKTPPrompt},
					{
						InlineData: &geminiInlineData{
							MimeType: mimeType,
							Data:     base64Image,
						},
					},
				},
			},
		},
		GenerationConfig: geminiGenConfig{
			ResponseMimeType: "application/json",
			Temperature:      0.1,
		},
	}

	reqBody, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("%w: failed marshaling gemini request: %v", ocr.ErrOCRFailed, err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", g.baseURL, g.model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("%w: failed creating gemini request: %v", ocr.ErrOCRFailed, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", g.apiKey)

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: gemini request failed: %v", ocr.ErrOCRFailed, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed reading gemini response: %v", ocr.ErrOCRFailed, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: gemini returned HTTP %d: %s", ocr.ErrOCRFailed, resp.StatusCode, string(bodyBytes))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return nil, fmt.Errorf("%w: failed unmarshaling gemini response: %v", ocr.ErrOCRFailed, err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("%w: gemini api error: %s", ocr.ErrOCRFailed, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("%w: empty candidate response from gemini", ocr.ErrOCRFailed)
	}

	var rawText string
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		if part.Text != "" {
			rawText = part.Text
			break
		}
	}
	if rawText == "" {
		return nil, fmt.Errorf("%w: empty text in candidate response from gemini", ocr.ErrOCRFailed)
	}

	cleanedText := strings.TrimSpace(rawText)
	cleanedText = strings.TrimPrefix(cleanedText, "```json")
	cleanedText = strings.TrimPrefix(cleanedText, "```")
	cleanedText = strings.TrimSuffix(cleanedText, "```")
	cleanedText = strings.TrimSpace(cleanedText)

	var extracted geminiExtractedJSON
	if err := json.Unmarshal([]byte(cleanedText), &extracted); err != nil {
		return nil, fmt.Errorf("%w: failed parsing model json output: %v", ocr.ErrOCRFailed, err)
	}

	if strings.EqualFold(extracted.DocumentType, "unsupported") || !strings.EqualFold(extracted.DocumentType, "ktp") {
		return nil, ocr.ErrUnsupportedDocument
	}

	// Validate and score extracted fields through our deterministic pipeline
	rawKTP := &ocr.KTPData{
		NIK:              extracted.NIK,
		Nama:             extracted.Nama,
		TempatLahir:      extracted.TempatLahir,
		TanggalLahir:     extracted.TanggalLahir,
		JenisKelamin:     extracted.JenisKelamin,
		Alamat:           extracted.Alamat,
		RTRW:             extracted.RTRW,
		Kelurahan:        extracted.Kelurahan,
		Kecamatan:        extracted.Kecamatan,
		Agama:            extracted.Agama,
		StatusPerkawinan: extracted.StatusPerkawinan,
		Pekerjaan:        extracted.Pekerjaan,
		Kewarganegaraan:  extracted.Kewarganegaraan,
	}

	validatedKTP, confidence, _ := ocr.ValidateKTP(rawKTP)

	return &ocr.OCRResult{
		DocumentType: "ktp",
		Confidence:   confidence,
		RawText:      extracted.RawText,
		Data:         validatedKTP,
	}, nil
}
