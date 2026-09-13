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
	defaultGeminiModel   = "gemini-3.6-flash"
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

// WithTimeout sets a custom HTTP client timeout.
func WithTimeout(timeout time.Duration) GeminiOption {
	return func(g *GeminiOCREngine) {
		if g.httpClient != nil {
			g.httpClient.Timeout = timeout
		}
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
			Timeout: 25 * time.Second,
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
	DocumentType string  `json:"document_type"`
	Confidence   float64 `json:"confidence"`
	RawText      string  `json:"raw_text"`

	// KTP fields
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

	// SIM fields
	NomorSIM      string `json:"nomor_sim"`
	Golongan      string `json:"golongan"`
	GolonganDarah string `json:"golongan_darah"`
	Polda         string `json:"polda"`
	MasaBerlaku   string `json:"masa_berlaku"`

	// Passport fields
	PassportNumber string `json:"passport_number"`
	FullName       string `json:"full_name"`
	Nationality    string `json:"nationality"`
	PlaceOfBirth   string `json:"place_of_birth"`
	DateOfBirth    string `json:"date_of_birth"`
	Gender         string `json:"gender"`
	IssueDate      string `json:"issue_date"`
	ExpiryDate     string `json:"expiry_date"`
	IssuingOffice  string `json:"issuing_office"`
	MRZLine1       string `json:"mrz_line1"`
	MRZLine2       string `json:"mrz_line2"`

	// NPWP fields
	NPWP          string `json:"npwp"`
	KotaKabupaten string `json:"kota_kabupaten"`
	Provinsi      string `json:"provinsi"`
	KPP           string `json:"kpp"`
	TanggalDaftar string `json:"tanggal_daftar"`

	// KK fields
	NomorKK            string               `json:"nomor_kk"`
	KepalaKeluarga     string               `json:"kepala_keluarga"`
	KodePos            string               `json:"kode_pos"`
	KelurahanDesa      string               `json:"kelurahan_desa"`
	KabupatenKota      string               `json:"kabupaten_kota"`
	TanggalDikeluarkan string               `json:"tanggal_dikeluarkan"`
	AnggotaKeluarga    []ocr.KKFamilyMember `json:"anggota_keluarga"`

	// Invoice fields
	InvoiceNumber string                `json:"invoice_number"`
	InvoiceDate   string                `json:"invoice_date"`
	DueDate       string                `json:"due_date"`
	SellerName    string                `json:"seller_name"`
	SellerNPWP    string                `json:"seller_npwp"`
	SellerAddress string                `json:"seller_address"`
	BuyerName     string                `json:"buyer_name"`
	BuyerNPWP     string                `json:"buyer_npwp"`
	BuyerAddress  string                `json:"buyer_address"`
	Currency      string                `json:"currency"`
	Subtotal      float64               `json:"subtotal"`
	Discount      float64               `json:"discount"`
	DPP           float64               `json:"dpp"`
	PPN           float64               `json:"ppn"`
	GrandTotal    float64               `json:"grand_total"`
	LineItems     []ocr.InvoiceLineItem `json:"line_items"`
}

const geminiDocumentPrompt = `You are a strict, high-accuracy Indonesian identity document OCR extraction engine.
Analyze the provided image.
First determine if the image is an Indonesian Kartu Tanda Penduduk (KTP), Surat Izin Mengemudi (SIM), Paspor Republik Indonesia (Passport), Nomor Pokok Wajib Pajak (NPWP), Kartu Keluarga (KK), or Commercial Invoice / Faktur Pajak (invoice).
If it is neither, output JSON with "document_type": "unsupported".

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
}

If it IS an Indonesian SIM (Surat Izin Mengemudi), extract all visible fields into this exact JSON structure:
{
  "document_type": "sim",
  "confidence": 0.95,
  "raw_text": "all raw text recognized on the card",
  "nomor_sim": "12-16 digit SIM number",
  "golongan": "A, B I, B II, C, C I, C II, D, or D I",
  "nama": "FULL NAME",
  "tempat_lahir": "BIRTH PLACE",
  "tanggal_lahir": "YYYY-MM-DD",
  "golongan_darah": "A, B, AB, O, or -",
  "jenis_kelamin": "PRIA or WANITA",
  "alamat": "STREET ADDRESS",
  "pekerjaan": "OCCUPATION",
  "polda": "POLDA REGION",
  "masa_berlaku": "YYYY-MM-DD"
}

If it IS an Indonesian Passport (Paspor Republik Indonesia), extract all visible fields into this exact JSON structure:
{
  "document_type": "passport",
  "confidence": 0.95,
  "raw_text": "all raw text recognized on the document",
  "passport_number": "8-9 character passport number",
  "full_name": "FULL NAME",
  "nationality": "IDN",
  "place_of_birth": "BIRTH PLACE",
  "date_of_birth": "YYYY-MM-DD",
  "gender": "LAKI-LAKI or PEREMPUAN",
  "issue_date": "YYYY-MM-DD",
  "expiry_date": "YYYY-MM-DD",
  "issuing_office": "ISSUING OFFICE / KANIM",
  "mrz_line1": "44 character MRZ line 1",
  "mrz_line2": "44 character MRZ line 2"
}

If it IS an Indonesian NPWP (Nomor Pokok Wajib Pajak), extract all visible fields into this exact JSON structure:
{
  "document_type": "npwp",
  "confidence": 0.95,
  "raw_text": "all raw text recognized on the card",
  "npwp": "15 or 16 digit NPWP",
  "nama": "TAXPAYER NAME",
  "nik": "16 digit NIK (if present)",
  "alamat": "STREET ADDRESS",
  "kelurahan": "KELURAHAN",
  "kecamatan": "KECAMATAN",
  "kota_kabupaten": "CITY OR REGENCY",
  "provinsi": "PROVINCE",
  "kpp": "KANTOR PELAYANAN PAJAK",
  "tanggal_daftar": "YYYY-MM-DD"
}

If it IS an Indonesian Kartu Keluarga (KK), extract all visible fields into this exact JSON structure:
{
  "document_type": "kk",
  "confidence": 0.95,
  "raw_text": "all raw text recognized on the document",
  "nomor_kk": "16 digit Nomor KK",
  "kepala_keluarga": "HEAD OF HOUSEHOLD NAME",
  "alamat": "STREET ADDRESS",
  "rt_rw": "000/000",
  "kode_pos": "POSTAL CODE",
  "kelurahan_desa": "KELURAHAN/DESA",
  "kecamatan": "KECAMATAN",
  "kabupaten_kota": "KABUPATEN/KOTA",
  "provinsi": "PROVINCE",
  "tanggal_dikeluarkan": "YYYY-MM-DD",
  "anggota_keluarga": [
    {
      "nama": "FULL NAME",
      "nik": "16 digit NIK",
      "jenis_kelamin": "LAKI-LAKI or PEREMPUAN",
      "tempat_lahir": "BIRTH PLACE",
      "tanggal_lahir": "YYYY-MM-DD",
      "agama": "RELIGION",
      "pendidikan": "EDUCATION LEVEL",
      "jenis_pekerjaan": "OCCUPATION",
      "golongan_darah": "BLOOD TYPE",
      "status_perkawinan": "MARITAL STATUS",
      "status_hubungan": "KEPALA KELUARGA or SUAMI or ISTRI or ANAK or etc",
      "kewarganegaraan": "WNI or WNA",
      "nama_ayah": "FATHER NAME",
      "nama_ibu": "MOTHER NAME"
    }
  ]
}

If it IS an Indonesian Commercial Invoice or Faktur Pajak, extract all visible fields into this exact JSON structure:
{
  "document_type": "invoice",
  "confidence": 0.95,
  "raw_text": "all raw text recognized on the invoice",
  "invoice_number": "INVOICE NUMBER OR FAKTUR NUMBER",
  "invoice_date": "YYYY-MM-DD",
  "due_date": "YYYY-MM-DD",
  "seller_name": "SELLER/PKP NAME",
  "seller_npwp": "SELLER NPWP",
  "seller_address": "SELLER ADDRESS",
  "buyer_name": "BUYER NAME",
  "buyer_npwp": "BUYER NPWP",
  "buyer_address": "BUYER ADDRESS",
  "currency": "IDR",
  "subtotal": 10000000,
  "discount": 0,
  "dpp": 10000000,
  "ppn": 1100000,
  "grand_total": 11100000,
  "line_items": [
    {
      "description": "ITEM DESCRIPTION",
      "quantity": 1,
      "unit_price": 10000000,
      "total_price": 10000000
    }
  ]
}`

const geminiKTPPrompt = geminiDocumentPrompt

var jsonMarshal = json.Marshal

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

	reqBody, err := jsonMarshal(reqPayload)
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
		return nil, fmt.Errorf("%w: gemini request failed: %w", ocr.ErrOCRFailed, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed reading gemini response: %w", ocr.ErrOCRFailed, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: gemini returned HTTP %d: %s", ocr.ErrOCRFailed, resp.StatusCode, string(bodyBytes))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return nil, fmt.Errorf("%w: failed unmarshaling gemini response: %w", ocr.ErrOCRFailed, err)
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

	if strings.EqualFold(extracted.DocumentType, "unsupported") {
		return nil, ocr.ErrUnsupportedDocument
	}

	if strings.EqualFold(extracted.DocumentType, "sim") {
		rawSIM := &ocr.SIMData{
			NomorSIM:      extracted.NomorSIM,
			Golongan:      extracted.Golongan,
			Nama:          extracted.Nama,
			TempatLahir:   extracted.TempatLahir,
			TanggalLahir:  extracted.TanggalLahir,
			GolonganDarah: extracted.GolonganDarah,
			JenisKelamin:  extracted.JenisKelamin,
			Alamat:        extracted.Alamat,
			Pekerjaan:     extracted.Pekerjaan,
			Polda:         extracted.Polda,
			MasaBerlaku:   extracted.MasaBerlaku,
		}

		validatedSIM, confidence, _ := ocr.ValidateSIM(rawSIM)

		return &ocr.OCRResult{
			DocumentType: "sim",
			Confidence:   confidence,
			RawText:      extracted.RawText,
			SIMData:      validatedSIM,
		}, nil
	}

	if strings.EqualFold(extracted.DocumentType, "passport") {
		name := extracted.FullName
		if name == "" {
			name = extracted.Nama
		}
		dob := extracted.DateOfBirth
		if dob == "" {
			dob = extracted.TanggalLahir
		}
		pob := extracted.PlaceOfBirth
		if pob == "" {
			pob = extracted.TempatLahir
		}
		gender := extracted.Gender
		if gender == "" {
			gender = extracted.JenisKelamin
		}
		rawPassport := &ocr.PassportData{
			PassportNumber: extracted.PassportNumber,
			FullName:       name,
			Nationality:    extracted.Nationality,
			DateOfBirth:    dob,
			PlaceOfBirth:   pob,
			Gender:         gender,
			IssueDate:      extracted.IssueDate,
			ExpiryDate:     extracted.ExpiryDate,
			IssuingOffice:  extracted.IssuingOffice,
			MRZLine1:       extracted.MRZLine1,
			MRZLine2:       extracted.MRZLine2,
		}

		validatedPassport, confidence, _ := ocr.ValidatePassport(rawPassport)

		return &ocr.OCRResult{
			DocumentType: "passport",
			Confidence:   confidence,
			RawText:      extracted.RawText,
			PassportData: validatedPassport,
		}, nil
	}

	if strings.EqualFold(extracted.DocumentType, "npwp") {
		rawNPWP := &ocr.NPWPData{
			NPWP:          ocr.CleanNPWP(extracted.NPWP),
			Nama:          extracted.Nama,
			NIK:           ocr.CleanNPWP(extracted.NIK),
			Alamat:        extracted.Alamat,
			Kelurahan:     extracted.Kelurahan,
			Kecamatan:     extracted.Kecamatan,
			KotaKabupaten: extracted.KotaKabupaten,
			Provinsi:      extracted.Provinsi,
			KPP:           extracted.KPP,
			TanggalDaftar: extracted.TanggalDaftar,
		}
		confidence := extracted.Confidence
		if confidence <= 0 {
			confidence = 0.95
		}
		return &ocr.OCRResult{
			DocumentType: "npwp",
			Confidence:   confidence,
			RawText:      extracted.RawText,
			NPWPData:     rawNPWP,
		}, nil
	}

	if strings.EqualFold(extracted.DocumentType, "kk") {
		rawKK := &ocr.KKData{
			NomorKK:            ocr.CleanNomorKK(extracted.NomorKK),
			KepalaKeluarga:     extracted.KepalaKeluarga,
			Alamat:             extracted.Alamat,
			RTRW:               extracted.RTRW,
			KodePos:            extracted.KodePos,
			KelurahanDesa:      extracted.KelurahanDesa,
			Kecamatan:          extracted.Kecamatan,
			KabupatenKota:      extracted.KabupatenKota,
			Provinsi:           extracted.Provinsi,
			TanggalDikeluarkan: extracted.TanggalDikeluarkan,
			AnggotaKeluarga:    extracted.AnggotaKeluarga,
		}
		if rawKK.KepalaKeluarga == "" {
			rawKK.KepalaKeluarga = extracted.Nama
		}
		if rawKK.KelurahanDesa == "" {
			rawKK.KelurahanDesa = extracted.Kelurahan
		}
		valRes := ocr.ValidateKK(rawKK)
		confidence := extracted.Confidence
		if confidence <= 0 {
			confidence = 0.95
		}
		if !valRes.IsValid {
			confidence = confidence * 0.7
		}
		return &ocr.OCRResult{
			DocumentType: "kk",
			Confidence:   confidence,
			RawText:      extracted.RawText,
			KKData:       rawKK,
		}, nil
	}

	if strings.EqualFold(extracted.DocumentType, "invoice") {
		rawInvoice := &ocr.InvoiceData{
			InvoiceNumber: extracted.InvoiceNumber,
			InvoiceDate:   extracted.InvoiceDate,
			DueDate:       extracted.DueDate,
			SellerName:    extracted.SellerName,
			SellerNPWP:    extracted.SellerNPWP,
			SellerAddress: extracted.SellerAddress,
			BuyerName:     extracted.BuyerName,
			BuyerNPWP:     extracted.BuyerNPWP,
			BuyerAddress:  extracted.BuyerAddress,
			Currency:      extracted.Currency,
			Subtotal:      extracted.Subtotal,
			Discount:      extracted.Discount,
			DPP:           extracted.DPP,
			PPN:           extracted.PPN,
			GrandTotal:    extracted.GrandTotal,
			LineItems:     extracted.LineItems,
		}
		if rawInvoice.Currency == "" {
			rawInvoice.Currency = "IDR"
		}
		confidence := extracted.Confidence
		if confidence <= 0 {
			confidence = 0.95
		}
		if err := ocr.ValidateInvoice(rawInvoice); err != nil {
			confidence = confidence * 0.7
		}
		return &ocr.OCRResult{
			DocumentType: "invoice",
			Confidence:   confidence,
			RawText:      extracted.RawText,
			InvoiceData:  rawInvoice,
		}, nil
	}

	if strings.EqualFold(extracted.DocumentType, "ktp") {
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

	return nil, ocr.ErrUnsupportedDocument
}
