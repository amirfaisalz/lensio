package handlers

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"github.com/amirfaisalz/lensio/apps/api/internal/store"
	"github.com/amirfaisalz/lensio/apps/api/internal/telemetry"
	"github.com/amirfaisalz/lensio/services/ocr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// GenerateUUIDv4 generates an RFC 4122 compliant UUID v4 string using crypto/rand.
func GenerateUUIDv4() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("00000000-0000-4000-8000-%012x", uint64(time.Now().UnixNano())&0xffffffffffff)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant RFC 4122
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ProcessingDetail captures execution timing.
type ProcessingDetail struct {
	LatencyMS int `json:"latency_ms"`
}

// KTPResponse represents the standard response payload for successful KTP OCR extraction.
type KTPResponse struct {
	ID           string           `json:"id"`
	DocumentType string           `json:"document_type"`
	Confidence   float64          `json:"confidence"`
	Data         *ocr.KTPData     `json:"data"`
	Processing   ProcessingDetail `json:"processing"`
}

// SIMResponse represents the standard response payload for successful SIM OCR extraction.
type SIMResponse struct {
	ID           string           `json:"id"`
	DocumentType string           `json:"document_type"`
	Confidence   float64          `json:"confidence"`
	Data         *ocr.SIMData     `json:"data"`
	Processing   ProcessingDetail `json:"processing"`
}

// PassportResponse represents the standard response payload for successful Passport OCR extraction.
type PassportResponse struct {
	ID           string            `json:"id"`
	DocumentType string            `json:"document_type"`
	Confidence   float64           `json:"confidence"`
	Data         *ocr.PassportData `json:"data"`
	Processing   ProcessingDetail  `json:"processing"`
}

// NPWPResponse represents the standard response payload for successful NPWP OCR extraction.
type NPWPResponse struct {
	ID           string           `json:"id"`
	DocumentType string           `json:"document_type"`
	Confidence   float64          `json:"confidence"`
	Data         *ocr.NPWPData    `json:"data"`
	Processing   ProcessingDetail `json:"processing"`
}

// KKResponse represents the standard response payload for successful Kartu Keluarga OCR extraction.
type KKResponse struct {
	ID           string           `json:"id"`
	DocumentType string           `json:"document_type"`
	Confidence   float64          `json:"confidence"`
	Data         *ocr.KKData      `json:"data"`
	Processing   ProcessingDetail `json:"processing"`
}

// InvoiceResponse represents the standard response payload for successful commercial invoice / e-faktur OCR extraction.
type InvoiceResponse struct {
	ID           string           `json:"id"`
	DocumentType string           `json:"document_type"`
	Confidence   float64          `json:"confidence"`
	Data         *ocr.InvoiceData `json:"data"`
	Processing   ProcessingDetail `json:"processing"`
}

// OCRRequestMetadata represents the non-PII execution metadata query response.
type OCRRequestMetadata struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	DocType    string    `json:"doc_type"`
	Confidence float64   `json:"confidence"`
	LatencyMS  int       `json:"latency_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

// QuotaChecker specifies the capability to evaluate remaining monthly request quota.
type QuotaChecker interface {
	CheckQuota(ctx context.Context, orgID string) (allowed bool, remaining int, limit int, err error)
}

type ocrDocConfig struct {
	docType                string
	docLabel               string
	engineErrorLog         string
	unsupportedErrs        []error
	authMessage            string
	missingFileMessage     string
	readFailMessage        string
	validSpanStatus        string
	handlerSpanName        string
	skipQuotaWhenOrgEmpty  bool
	formFieldFallback      string
	clearOnValidationError bool
	ocrFailedBranch        bool
	genericFailureStatus   int
	genericFailureCode     string
	genericFailureMessage  string
	classifyStage          string
	classify               func(*ocr.OCRResult) bool
	normalize              func(*ocr.OCRResult) (any, float64)
	respond                func(recordID string, confidence float64, latencyMS int, data any) any
}

func ocrPipeline(cfg ocrDocConfig, engine ocr.OCREngine, ocrStore store.OCRRequestStore, quotaChecker QuotaChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		ctx := r.Context()
		if cfg.handlerSpanName != "" {
			var handlerSpan trace.Span
			ctx, handlerSpan = telemetry.Tracer().Start(ctx, cfg.handlerSpanName)
			defer handlerSpan.End()
		}

		key := middleware.GetAPIKey(ctx)
		if key == nil {
			response.ErrorWithRequest(w, r, http.StatusUnauthorized, response.CodeInvalidAPIKey, cfg.authMessage)
			return
		}

		if quotaChecker != nil && !(cfg.skipQuotaWhenOrgEmpty && key.OrgID == "") {
			allowed, _, _, err := quotaChecker.CheckQuota(r.Context(), key.OrgID)
			if err != nil {
				response.ErrorWithRequest(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed checking quota availability")
				return
			}
			if !allowed {
				response.ErrorWithRequest(w, r, http.StatusTooManyRequests, response.CodeQuotaExceeded, "Monthly API quota exceeded. Please upgrade your plan.")
				return
			}
		}

		r.Body = http.MaxBytesReader(w, r.Body, ocr.MaxImageSizeBytes+1024)
		if err := r.ParseMultipartForm(ocr.MaxImageSizeBytes); err != nil {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidDocument, "File exceeds 5MB or invalid multipart form")
			return
		}
		defer func() {
			if r.MultipartForm != nil {
				_ = r.MultipartForm.RemoveAll()
			}
		}()

		file, _, err := r.FormFile("document")
		if err != nil && cfg.formFieldFallback != "" {
			file, _, err = r.FormFile(cfg.formFieldFallback)
		}
		if err != nil {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidDocument, cfg.missingFileMessage)
			return
		}
		defer file.Close()

		imgBytes, err := io.ReadAll(file)
		if err != nil {
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidDocument, cfg.readFailMessage)
			return
		}

		tracer := telemetry.Tracer()

		valStart := time.Now()
		_, valSpan := tracer.Start(r.Context(), "ocr.validate_image")
		if _, err := ocr.ValidateImage(imgBytes); err != nil {
			valSpan.RecordError(err)
			valSpan.SetStatus(codes.Error, err.Error())
			valSpan.End()

			valDuration := time.Since(valStart).Seconds()
			telemetry.RecordOCRStageDuration(r.Context(), "validation", "error", valDuration)
			telemetry.RecordOCRError(r.Context(), "validation", response.CodeInvalidDocument)
			telemetry.RecordOCRRequest(r.Context(), cfg.docType, "failure", 0, time.Since(startTime).Seconds())

			if cfg.clearOnValidationError {
				clear(imgBytes)
			}
			response.ErrorWithRequest(w, r, http.StatusBadRequest, response.CodeInvalidDocument, err.Error())
			return
		}
		valSpan.SetStatus(codes.Ok, cfg.validSpanStatus)
		valSpan.End()
		telemetry.RecordOCRStageDuration(r.Context(), "validation", "success", time.Since(valStart).Seconds())

		if engine == nil {
			telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeOCRFailed)
			telemetry.RecordOCRRequest(r.Context(), cfg.docType, "failure", 0, time.Since(startTime).Seconds())
			response.ErrorWithRequest(w, r, http.StatusBadGateway, response.CodeOCRFailed, "OCR engine not available")
			return
		}

		engStart := time.Now()
		engCtx, engSpan := tracer.Start(r.Context(), "ocr.engine_extract")
		ocrResult, err := engine.Extract(engCtx, imgBytes)
		clear(imgBytes)
		engDuration := time.Since(engStart).Seconds()

		if err != nil {
			slog.ErrorContext(r.Context(), cfg.engineErrorLog, slog.String("error", err.Error()))
			engSpan.RecordError(err)
			engSpan.SetStatus(codes.Error, err.Error())
			engSpan.End()
			telemetry.RecordOCRStageDuration(r.Context(), "ocr_engine", "error", engDuration)

			unsupported := errors.Is(err, ocr.ErrUnsupportedDocument)
			for _, sentinel := range cfg.unsupportedErrs {
				if errors.Is(err, sentinel) {
					unsupported = true
					break
				}
			}
			if unsupported {
				telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeUnsupportedDocument)
				telemetry.RecordOCRRequest(r.Context(), cfg.docType, "failure", 0, time.Since(startTime).Seconds())
				response.ErrorWithRequest(w, r, http.StatusUnprocessableEntity, response.CodeUnsupportedDocument, "Uploaded image was not identified as an "+cfg.docLabel)
				return
			}
			if errors.Is(err, ocr.ErrCircuitOpen) {
				telemetry.RecordOCRError(r.Context(), "circuit_breaker", response.CodeOCRFailed)
				telemetry.RecordOCRRequest(r.Context(), cfg.docType, "failure", 0, time.Since(startTime).Seconds())
				response.ErrorWithRequest(w, r, http.StatusGatewayTimeout, response.CodeOCRFailed, "OCR circuit breaker is open: upstream service temporarily unavailable")
				return
			}
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline exceeded") {
				telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeOCRFailed)
				telemetry.RecordOCRRequest(r.Context(), cfg.docType, "failure", 0, time.Since(startTime).Seconds())
				response.ErrorWithRequest(w, r, http.StatusGatewayTimeout, response.CodeOCRFailed, "OCR engine processing timeout")
				return
			}
			if cfg.ocrFailedBranch && errors.Is(err, ocr.ErrOCRFailed) {
				telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeOCRFailed)
				telemetry.RecordOCRRequest(r.Context(), cfg.docType, "failure", 0, time.Since(startTime).Seconds())
				response.ErrorWithRequest(w, r, http.StatusBadGateway, response.CodeOCRFailed, "OCR engine processing timeout or failure")
				return
			}

			telemetry.RecordOCRError(r.Context(), "ocr_engine", cfg.genericFailureCode)
			telemetry.RecordOCRRequest(r.Context(), cfg.docType, "failure", 0, time.Since(startTime).Seconds())
			response.ErrorWithRequest(w, r, cfg.genericFailureStatus, cfg.genericFailureCode, cfg.genericFailureMessage)
			return
		}
		engSpan.SetStatus(codes.Ok, "extracted")
		engSpan.End()
		telemetry.RecordOCRStageDuration(r.Context(), "ocr_engine", "success", engDuration)

		accepted := ocrResult != nil && ocrResult.DocumentType == cfg.docType
		if accepted && cfg.classify != nil {
			accepted = cfg.classify(ocrResult)
		}
		if !accepted {
			telemetry.RecordOCRError(r.Context(), cfg.classifyStage, response.CodeUnsupportedDocument)
			telemetry.RecordOCRRequest(r.Context(), cfg.docType, "failure", 0, time.Since(startTime).Seconds())
			response.ErrorWithRequest(w, r, http.StatusUnprocessableEntity, response.CodeUnsupportedDocument, "Uploaded image was not identified as an "+cfg.docLabel)
			return
		}

		normStart := time.Now()
		_, normSpan := tracer.Start(r.Context(), "ocr.normalize_and_validate")
		data, confidence := cfg.normalize(ocrResult)
		normDuration := time.Since(normStart).Seconds()
		normSpan.SetAttributes(
			attribute.Float64("ocr.confidence", confidence),
			attribute.String("ocr.doc_type", cfg.docType),
		)
		normSpan.SetStatus(codes.Ok, "normalized")
		normSpan.End()
		telemetry.RecordOCRStageDuration(r.Context(), "field_extraction", "success", normDuration)

		latencyMS := int(time.Since(startTime).Milliseconds())
		totalDurationSec := float64(latencyMS) / 1000.0
		recordID := GenerateUUIDv4()

		if ocrStore != nil {
			_, storeSpan := tracer.Start(r.Context(), "ocr.store_metadata")
			var apiKeyID *string
			if key.ID != "" {
				apiKeyID = &key.ID
			}

			ocrReq := &store.OCRRequest{
				ID:         recordID,
				OrgID:      key.OrgID,
				APIKeyID:   apiKeyID,
				Status:     "completed",
				Confidence: confidence,
				LatencyMS:  latencyMS,
				DocType:    cfg.docType,
			}

			if err := ocrStore.CreateOCRRequest(r.Context(), ocrReq); err != nil {
				storeSpan.RecordError(err)
				storeSpan.SetStatus(codes.Error, err.Error())
				slog.ErrorContext(r.Context(), "failed saving ocr request metadata",
					slog.String("error", err.Error()),
					slog.String("record_id", recordID),
				)
			} else {
				storeSpan.SetStatus(codes.Ok, "saved")
			}
			storeSpan.End()
		}

		ocrStatus := "completed"
		if confidence < 0.7 {
			ocrStatus = "low_confidence"
		}
		telemetry.RecordOCRRequest(r.Context(), cfg.docType, ocrStatus, confidence, totalDurationSec)

		slog.InfoContext(r.Context(), cfg.docType+" ocr request completed",
			slog.String("request_id", response.GetRequestID(r.Context())),
			slog.String("record_id", recordID),
			slog.String("org_id", key.OrgID),
			slog.Int("latency_ms", latencyMS),
			slog.Float64("confidence", confidence),
			slog.String("doc_type", cfg.docType),
		)

		response.JSON(w, http.StatusOK, cfg.respond(recordID, confidence, latencyMS, data))
	}
}

func ktpDocConfig() ocrDocConfig {
	return ocrDocConfig{
		docType:               "ktp",
		docLabel:              "Indonesian KTP",
		engineErrorLog:        "ocr engine extraction failed",
		authMessage:           "Unauthenticated request",
		missingFileMessage:    "Missing 'document' image file in multipart form",
		readFailMessage:       "Failed reading uploaded document bytes",
		validSpanStatus:       "valid",
		ocrFailedBranch:       true,
		genericFailureStatus:  http.StatusInternalServerError,
		genericFailureCode:    response.CodeInternalError,
		genericFailureMessage: "Internal error during document processing",
		classifyStage:         "classification",
		normalize: func(res *ocr.OCRResult) (any, float64) {
			normalized, confidence, _ := ocr.ValidateKTP(res.Data)
			return normalized, confidence
		},
		respond: func(recordID string, confidence float64, latencyMS int, data any) any {
			return KTPResponse{
				ID:           recordID,
				DocumentType: "ktp",
				Confidence:   confidence,
				Data:         data.(*ocr.KTPData),
				Processing:   ProcessingDetail{LatencyMS: latencyMS},
			}
		},
	}
}

func simDocConfig() ocrDocConfig {
	cfg := ktpDocConfig()
	cfg.docType = "sim"
	cfg.docLabel = "Indonesian SIM"
	cfg.engineErrorLog = "sim ocr engine extraction failed"
	cfg.unsupportedErrs = []error{ocr.ErrUnsupportedSIMDocument}
	cfg.normalize = func(res *ocr.OCRResult) (any, float64) {
		normalized, confidence, _ := ocr.ValidateSIM(res.SIMData)
		return normalized, confidence
	}
	cfg.respond = func(recordID string, confidence float64, latencyMS int, data any) any {
		return SIMResponse{
			ID:           recordID,
			DocumentType: "sim",
			Confidence:   confidence,
			Data:         data.(*ocr.SIMData),
			Processing:   ProcessingDetail{LatencyMS: latencyMS},
		}
	}
	return cfg
}

func passportDocConfig() ocrDocConfig {
	cfg := ktpDocConfig()
	cfg.docType = "passport"
	cfg.docLabel = "Indonesian Passport"
	cfg.engineErrorLog = "passport ocr engine extraction failed"
	cfg.unsupportedErrs = []error{ocr.ErrUnsupportedPassportDocument}
	cfg.normalize = func(res *ocr.OCRResult) (any, float64) {
		normalized, confidence, _ := ocr.ValidatePassport(res.PassportData)
		return normalized, confidence
	}
	cfg.respond = func(recordID string, confidence float64, latencyMS int, data any) any {
		return PassportResponse{
			ID:           recordID,
			DocumentType: "passport",
			Confidence:   confidence,
			Data:         data.(*ocr.PassportData),
			Processing:   ProcessingDetail{LatencyMS: latencyMS},
		}
	}
	return cfg
}

func npwpDocConfig() ocrDocConfig {
	cfg := ktpDocConfig()
	cfg.docType = "npwp"
	cfg.docLabel = "Indonesian NPWP"
	cfg.missingFileMessage = "Missing 'document' multipart field"
	cfg.readFailMessage = "Failed reading image payload"
	cfg.validSpanStatus = "validated"
	cfg.clearOnValidationError = true
	cfg.unsupportedErrs = []error{ocr.ErrUnsupportedNPWPDocument}
	cfg.ocrFailedBranch = false
	cfg.genericFailureStatus = http.StatusBadGateway
	cfg.genericFailureCode = response.CodeOCRFailed
	cfg.genericFailureMessage = "Upstream OCR engine error"
	cfg.classifyStage = "validation"
	cfg.classify = func(res *ocr.OCRResult) bool {
		return res != nil && res.DocumentType == "npwp" && res.NPWPData != nil
	}
	cfg.normalize = func(res *ocr.OCRResult) (any, float64) {
		npwpData := res.NPWPData
		npwpData.NPWP = ocr.CleanNPWP(npwpData.NPWP)
		valid, _ := ocr.ValidateNPWPData(npwpData)
		confidence := res.Confidence
		if !valid && confidence > 0.5 {
			confidence = 0.5
		}
		return npwpData, confidence
	}
	cfg.respond = func(recordID string, confidence float64, latencyMS int, data any) any {
		return NPWPResponse{
			ID:           recordID,
			DocumentType: "npwp",
			Confidence:   confidence,
			Data:         data.(*ocr.NPWPData),
			Processing:   ProcessingDetail{LatencyMS: latencyMS},
		}
	}
	return cfg
}

func kkDocConfig() ocrDocConfig {
	cfg := npwpDocConfig()
	cfg.docType = "kk"
	cfg.docLabel = "Indonesian Kartu Keluarga"
	cfg.engineErrorLog = "ocr engine extraction failed"
	cfg.authMessage = "Unauthenticated request: missing or invalid API key"
	cfg.handlerSpanName = "ocr.kk_handler"
	cfg.skipQuotaWhenOrgEmpty = true
	cfg.unsupportedErrs = []error{ocr.ErrUnsupportedKKDocument}
	cfg.classify = func(res *ocr.OCRResult) bool {
		return res != nil && res.DocumentType == "kk" && res.KKData != nil
	}
	cfg.normalize = func(res *ocr.OCRResult) (any, float64) {
		kkData := res.KKData
		kkData.NomorKK = ocr.CleanNomorKK(kkData.NomorKK)
		valRes := ocr.ValidateKK(kkData)
		confidence := res.Confidence
		if !valRes.IsValid && confidence > 0.5 {
			confidence = 0.5
		}
		return kkData, confidence
	}
	cfg.respond = func(recordID string, confidence float64, latencyMS int, data any) any {
		return KKResponse{
			ID:           recordID,
			DocumentType: "kk",
			Confidence:   confidence,
			Data:         data.(*ocr.KKData),
			Processing:   ProcessingDetail{LatencyMS: latencyMS},
		}
	}
	return cfg
}

func invoiceDocConfig() ocrDocConfig {
	cfg := npwpDocConfig()
	cfg.docType = "invoice"
	cfg.docLabel = "Indonesian commercial invoice or e-faktur"
	cfg.engineErrorLog = "invoice ocr engine extraction failed"
	cfg.authMessage = "Unauthenticated request: missing or invalid API key"
	cfg.skipQuotaWhenOrgEmpty = true
	cfg.formFieldFallback = "image"
	cfg.unsupportedErrs = []error{ocr.ErrUnsupportedInvoiceDocument}
	cfg.classify = func(res *ocr.OCRResult) bool {
		return res != nil && res.DocumentType == "invoice" && res.InvoiceData != nil
	}
	cfg.normalize = func(res *ocr.OCRResult) (any, float64) {
		invoiceData := res.InvoiceData
		valErr := ocr.ValidateInvoice(invoiceData)
		confidence := res.Confidence
		if valErr != nil && confidence > 0.5 {
			confidence = 0.5
		}
		return invoiceData, confidence
	}
	cfg.respond = func(recordID string, confidence float64, latencyMS int, data any) any {
		return InvoiceResponse{
			ID:           recordID,
			DocumentType: "invoice",
			Confidence:   confidence,
			Data:         data.(*ocr.InvoiceData),
			Processing:   ProcessingDetail{LatencyMS: latencyMS},
		}
	}
	return cfg
}

// KTPOCRHandler processes an uploaded Indonesian KTP image, performs classification,
// deterministic field normalization/validation, and returns structured data.
// Uploaded image buffers are strictly processed in memory and discarded immediately.
func KTPOCRHandler(engine ocr.OCREngine, ocrStore store.OCRRequestStore, quotaChecker QuotaChecker) http.HandlerFunc {
	return ocrPipeline(ktpDocConfig(), engine, ocrStore, quotaChecker)
}

// SIMOCRHandler processes an uploaded Indonesian SIM image, performs classification,
// deterministic field normalization/validation, and returns structured data.
// Uploaded image buffers are strictly processed in memory and discarded immediately.
func SIMOCRHandler(engine ocr.OCREngine, ocrStore store.OCRRequestStore, quotaChecker QuotaChecker) http.HandlerFunc {
	return ocrPipeline(simDocConfig(), engine, ocrStore, quotaChecker)
}

// PassportOCRHandler processes an uploaded Indonesian Passport image, performs classification,
// deterministic field normalization/validation, and returns structured data.
// Uploaded image buffers are strictly processed in memory and discarded immediately.
func PassportOCRHandler(engine ocr.OCREngine, ocrStore store.OCRRequestStore, quotaChecker QuotaChecker) http.HandlerFunc {
	return ocrPipeline(passportDocConfig(), engine, ocrStore, quotaChecker)
}

// NPWPOCRHandler processes an uploaded Indonesian NPWP image, performs classification,
// deterministic field normalization/validation, and returns structured data.
// Uploaded image buffers are strictly processed in memory and discarded immediately.
func NPWPOCRHandler(engine ocr.OCREngine, ocrStore store.OCRRequestStore, quotaChecker QuotaChecker) http.HandlerFunc {
	return ocrPipeline(npwpDocConfig(), engine, ocrStore, quotaChecker)
}

// KKOCRHandler processes an uploaded Indonesian Kartu Keluarga image, performs classification,
// validates Nomor KK and family member structure, enforces quota, and records telemetry.
func KKOCRHandler(engine ocr.OCREngine, ocrStore store.OCRRequestStore, quota QuotaChecker) http.HandlerFunc {
	return ocrPipeline(kkDocConfig(), engine, ocrStore, quota)
}

// InvoiceOCRHandler handles Indonesian commercial invoice and e-faktur OCR extraction requests.
func InvoiceOCRHandler(engine ocr.OCREngine, ocrStore store.OCRRequestStore, quota QuotaChecker) http.HandlerFunc {
	return ocrPipeline(invoiceDocConfig(), engine, ocrStore, quota)
}

// GetOCRRequestHandler retrieves previous OCR execution metadata (non-PII) by ID.
func GetOCRRequestHandler(ocrStore store.OCRRequestStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := middleware.GetAPIKey(r.Context())
		if key == nil {
			response.ErrorWithRequest(
				w,
				r,
				http.StatusUnauthorized,
				response.CodeInvalidAPIKey,
				"Unauthenticated request",
			)
			return
		}

		reqID := r.PathValue("id")
		if reqID == "" {
			response.ErrorWithRequest(
				w,
				r,
				http.StatusBadRequest,
				response.CodeInvalidRequest,
				"Missing required parameter: id",
			)
			return
		}

		if ocrStore == nil {
			response.ErrorWithRequest(
				w,
				r,
				http.StatusInternalServerError,
				response.CodeInternalError,
				"Store not available",
			)
			return
		}

		metadata, err := ocrStore.GetOCRRequestByID(r.Context(), key.OrgID, reqID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusNotFound,
					response.CodeInvalidRequest,
					"Resource not found",
				)
				return
			}

			response.ErrorWithRequest(
				w,
				r,
				http.StatusInternalServerError,
				response.CodeInternalError,
				"Failed retrieving ocr request metadata",
			)
			return
		}

		response.JSON(w, http.StatusOK, OCRRequestMetadata{
			ID:         metadata.ID,
			Status:     metadata.Status,
			DocType:    metadata.DocType,
			Confidence: metadata.Confidence,
			LatencyMS:  metadata.LatencyMS,
			CreatedAt:  metadata.CreatedAt,
		})
	}
}
