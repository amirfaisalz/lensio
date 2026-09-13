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
)

// GenerateUUIDv4 generates an RFC 4122 compliant UUID v4 string using crypto/rand.
func GenerateUUIDv4() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000-0000-4000-8000-000000000000"
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

// KTPOCRHandler processes an uploaded Indonesian KTP image, performs classification,
// deterministic field normalization/validation, and returns structured data.
// Uploaded image buffers are strictly processed in memory and discarded immediately.
func KTPOCRHandler(engine ocr.OCREngine, ocrStore store.OCRRequestStore, quotaChecker QuotaChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

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

		if quotaChecker != nil {
			allowed, _, _, err := quotaChecker.CheckQuota(r.Context(), key.OrgID)
			if err != nil {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusInternalServerError,
					response.CodeInternalError,
					"Failed checking quota availability",
				)
				return
			}
			if !allowed {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusTooManyRequests,
					response.CodeQuotaExceeded,
					"Monthly API quota exceeded. Please upgrade your plan.",
				)
				return
			}
		}

		// Enforce maximum upload body size (5MB + 1KB buffer for multipart headers)
		r.Body = http.MaxBytesReader(w, r.Body, ocr.MaxImageSizeBytes+1024)
		if err := r.ParseMultipartForm(ocr.MaxImageSizeBytes); err != nil {
			response.ErrorWithRequest(
				w,
				r,
				http.StatusBadRequest,
				response.CodeInvalidDocument,
				"File exceeds 5MB or invalid multipart form",
			)
			return
		}
		defer func() {
			if r.MultipartForm != nil {
				_ = r.MultipartForm.RemoveAll()
			}
		}()

		file, _, err := r.FormFile("document")
		if err != nil {
			response.ErrorWithRequest(
				w,
				r,
				http.StatusBadRequest,
				response.CodeInvalidDocument,
				"Missing 'document' image file in multipart form",
			)
			return
		}
		defer file.Close()

		imgBytes, err := io.ReadAll(file)
		if err != nil {
			response.ErrorWithRequest(
				w,
				r,
				http.StatusBadRequest,
				response.CodeInvalidDocument,
				"Failed reading uploaded document bytes",
			)
			return
		}

		tracer := telemetry.Tracer()

		// Stage 1: Validate Image
		valStart := time.Now()
		_, valSpan := tracer.Start(r.Context(), "ocr.validate_image")
		if _, err := ocr.ValidateImage(imgBytes); err != nil {
			valSpan.RecordError(err)
			valSpan.SetStatus(codes.Error, err.Error())
			valSpan.End()

			valDuration := time.Since(valStart).Seconds()
			telemetry.RecordOCRStageDuration(r.Context(), "validation", "error", valDuration)
			telemetry.RecordOCRError(r.Context(), "validation", response.CodeInvalidDocument)
			telemetry.RecordOCRRequest(r.Context(), "ktp", "failure", 0, time.Since(startTime).Seconds())

			response.ErrorWithRequest(
				w,
				r,
				http.StatusBadRequest,
				response.CodeInvalidDocument,
				err.Error(),
			)
			return
		}
		valSpan.SetStatus(codes.Ok, "valid")
		valSpan.End()
		telemetry.RecordOCRStageDuration(r.Context(), "validation", "success", time.Since(valStart).Seconds())

		if engine == nil {
			telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeOCRFailed)
			telemetry.RecordOCRRequest(r.Context(), "ktp", "failure", 0, time.Since(startTime).Seconds())
			response.ErrorWithRequest(
				w,
				r,
				http.StatusBadGateway,
				response.CodeOCRFailed,
				"OCR engine not available",
			)
			return
		}

		// Stage 2: OCR Engine Extraction
		engStart := time.Now()
		engCtx, engSpan := tracer.Start(r.Context(), "ocr.engine_extract")
		ocrResult, err := engine.Extract(engCtx, imgBytes)
		imgBytes = nil
		engDuration := time.Since(engStart).Seconds()

		if err != nil {
			slog.ErrorContext(r.Context(), "ocr engine extraction failed", slog.String("error", err.Error()))
			engSpan.RecordError(err)
			engSpan.SetStatus(codes.Error, err.Error())
			engSpan.End()
			telemetry.RecordOCRStageDuration(r.Context(), "ocr_engine", "error", engDuration)

			if errors.Is(err, ocr.ErrUnsupportedDocument) {
				telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeUnsupportedDocument)
				telemetry.RecordOCRRequest(r.Context(), "ktp", "failure", 0, time.Since(startTime).Seconds())
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnprocessableEntity,
					response.CodeUnsupportedDocument,
					"Uploaded image was not identified as an Indonesian KTP",
				)
				return
			}
			if errors.Is(err, ocr.ErrCircuitOpen) {
				telemetry.RecordOCRError(r.Context(), "circuit_breaker", response.CodeOCRFailed)
				telemetry.RecordOCRRequest(r.Context(), "ktp", "failure", 0, time.Since(startTime).Seconds())
				response.ErrorWithRequest(
					w,
					r,
					http.StatusGatewayTimeout,
					response.CodeOCRFailed,
					"OCR circuit breaker is open: upstream service temporarily unavailable",
				)
				return
			}
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline exceeded") {
				telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeOCRFailed)
				telemetry.RecordOCRRequest(r.Context(), "ktp", "failure", 0, time.Since(startTime).Seconds())
				response.ErrorWithRequest(
					w,
					r,
					http.StatusGatewayTimeout,
					response.CodeOCRFailed,
					"OCR engine processing timeout",
				)
				return
			}
			if errors.Is(err, ocr.ErrOCRFailed) {
				telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeOCRFailed)
				telemetry.RecordOCRRequest(r.Context(), "ktp", "failure", 0, time.Since(startTime).Seconds())
				response.ErrorWithRequest(
					w,
					r,
					http.StatusBadGateway,
					response.CodeOCRFailed,
					"OCR engine processing timeout or failure",
				)
				return
			}

			telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeInternalError)
			telemetry.RecordOCRRequest(r.Context(), "ktp", "failure", 0, time.Since(startTime).Seconds())
			response.ErrorWithRequest(
				w,
				r,
				http.StatusInternalServerError,
				response.CodeInternalError,
				"Internal error during document processing",
			)
			return
		}
		engSpan.SetStatus(codes.Ok, "extracted")
		engSpan.End()
		telemetry.RecordOCRStageDuration(r.Context(), "ocr_engine", "success", engDuration)

		if ocrResult == nil || ocrResult.DocumentType != "ktp" {
			telemetry.RecordOCRError(r.Context(), "classification", response.CodeUnsupportedDocument)
			telemetry.RecordOCRRequest(r.Context(), "ktp", "failure", 0, time.Since(startTime).Seconds())
			response.ErrorWithRequest(
				w,
				r,
				http.StatusUnprocessableEntity,
				response.CodeUnsupportedDocument,
				"Uploaded image was not identified as an Indonesian KTP",
			)
			return
		}

		// Stage 3: Field Extraction & Normalization
		normStart := time.Now()
		_, normSpan := tracer.Start(r.Context(), "ocr.normalize_and_validate")
		normalizedKTP, confidence, _ := ocr.ValidateKTP(ocrResult.Data)
		normDuration := time.Since(normStart).Seconds()
		normSpan.SetAttributes(
			attribute.Float64("ocr.confidence", confidence),
			attribute.String("ocr.doc_type", "ktp"),
		)
		normSpan.SetStatus(codes.Ok, "normalized")
		normSpan.End()
		telemetry.RecordOCRStageDuration(r.Context(), "field_extraction", "success", normDuration)

		latencyMS := int(time.Since(startTime).Milliseconds())
		totalDurationSec := float64(latencyMS) / 1000.0
		recordID := GenerateUUIDv4()

		// Stage 4: Persist non-PII execution metadata in PostgreSQL
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
				DocType:    "ktp",
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

		// Record overall business metric
		ocrStatus := "completed"
		if confidence < 0.7 {
			ocrStatus = "low_confidence"
		}
		telemetry.RecordOCRRequest(r.Context(), "ktp", ocrStatus, confidence, totalDurationSec)

		// Zero PII structured log
		slog.InfoContext(r.Context(), "ktp ocr request completed",
			slog.String("request_id", response.GetRequestID(r.Context())),
			slog.String("record_id", recordID),
			slog.String("org_id", key.OrgID),
			slog.Int("latency_ms", latencyMS),
			slog.Float64("confidence", confidence),
			slog.String("doc_type", "ktp"),
		)

		response.JSON(w, http.StatusOK, KTPResponse{
			ID:           recordID,
			DocumentType: "ktp",
			Confidence:   confidence,
			Data:         normalizedKTP,
			Processing: ProcessingDetail{
				LatencyMS: latencyMS,
			},
		})
	}
}

// SIMOCRHandler processes an uploaded Indonesian SIM image, performs classification,
// deterministic field normalization/validation, and returns structured data.
// Uploaded image buffers are strictly processed in memory and discarded immediately.
func SIMOCRHandler(engine ocr.OCREngine, ocrStore store.OCRRequestStore, quotaChecker QuotaChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

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

		if quotaChecker != nil {
			allowed, _, _, err := quotaChecker.CheckQuota(r.Context(), key.OrgID)
			if err != nil {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusInternalServerError,
					response.CodeInternalError,
					"Failed checking quota availability",
				)
				return
			}
			if !allowed {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusTooManyRequests,
					response.CodeQuotaExceeded,
					"Monthly API quota exceeded. Please upgrade your plan.",
				)
				return
			}
		}

		// Enforce maximum upload body size (5MB + 1KB buffer for multipart headers)
		r.Body = http.MaxBytesReader(w, r.Body, ocr.MaxImageSizeBytes+1024)
		if err := r.ParseMultipartForm(ocr.MaxImageSizeBytes); err != nil {
			response.ErrorWithRequest(
				w,
				r,
				http.StatusBadRequest,
				response.CodeInvalidDocument,
				"File exceeds 5MB or invalid multipart form",
			)
			return
		}
		defer func() {
			if r.MultipartForm != nil {
				_ = r.MultipartForm.RemoveAll()
			}
		}()

		file, _, err := r.FormFile("document")
		if err != nil {
			response.ErrorWithRequest(
				w,
				r,
				http.StatusBadRequest,
				response.CodeInvalidDocument,
				"Missing 'document' image file in multipart form",
			)
			return
		}
		defer file.Close()

		imgBytes, err := io.ReadAll(file)
		if err != nil {
			response.ErrorWithRequest(
				w,
				r,
				http.StatusBadRequest,
				response.CodeInvalidDocument,
				"Failed reading uploaded document bytes",
			)
			return
		}

		tracer := telemetry.Tracer()

		// Stage 1: Validate Image
		valStart := time.Now()
		_, valSpan := tracer.Start(r.Context(), "ocr.validate_image")
		if _, err := ocr.ValidateImage(imgBytes); err != nil {
			valSpan.RecordError(err)
			valSpan.SetStatus(codes.Error, err.Error())
			valSpan.End()

			valDuration := time.Since(valStart).Seconds()
			telemetry.RecordOCRStageDuration(r.Context(), "validation", "error", valDuration)
			telemetry.RecordOCRError(r.Context(), "validation", response.CodeInvalidDocument)
			telemetry.RecordOCRRequest(r.Context(), "sim", "failure", 0, time.Since(startTime).Seconds())

			response.ErrorWithRequest(
				w,
				r,
				http.StatusBadRequest,
				response.CodeInvalidDocument,
				err.Error(),
			)
			return
		}
		valSpan.SetStatus(codes.Ok, "valid")
		valSpan.End()
		telemetry.RecordOCRStageDuration(r.Context(), "validation", "success", time.Since(valStart).Seconds())

		if engine == nil {
			telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeOCRFailed)
			telemetry.RecordOCRRequest(r.Context(), "sim", "failure", 0, time.Since(startTime).Seconds())
			response.ErrorWithRequest(
				w,
				r,
				http.StatusBadGateway,
				response.CodeOCRFailed,
				"OCR engine not available",
			)
			return
		}

		// Stage 2: OCR Engine Extraction
		engStart := time.Now()
		engCtx, engSpan := tracer.Start(r.Context(), "ocr.engine_extract")
		ocrResult, err := engine.Extract(engCtx, imgBytes)
		imgBytes = nil
		engDuration := time.Since(engStart).Seconds()

		if err != nil {
			slog.ErrorContext(r.Context(), "sim ocr engine extraction failed", slog.String("error", err.Error()))
			engSpan.RecordError(err)
			engSpan.SetStatus(codes.Error, err.Error())
			engSpan.End()
			telemetry.RecordOCRStageDuration(r.Context(), "ocr_engine", "error", engDuration)

			if errors.Is(err, ocr.ErrUnsupportedDocument) || errors.Is(err, ocr.ErrUnsupportedSIMDocument) {
				telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeUnsupportedDocument)
				telemetry.RecordOCRRequest(r.Context(), "sim", "failure", 0, time.Since(startTime).Seconds())
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnprocessableEntity,
					response.CodeUnsupportedDocument,
					"Uploaded image was not identified as an Indonesian SIM",
				)
				return
			}
			if errors.Is(err, ocr.ErrCircuitOpen) {
				telemetry.RecordOCRError(r.Context(), "circuit_breaker", response.CodeOCRFailed)
				telemetry.RecordOCRRequest(r.Context(), "sim", "failure", 0, time.Since(startTime).Seconds())
				response.ErrorWithRequest(
					w,
					r,
					http.StatusGatewayTimeout,
					response.CodeOCRFailed,
					"OCR circuit breaker is open: upstream service temporarily unavailable",
				)
				return
			}
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline exceeded") {
				telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeOCRFailed)
				telemetry.RecordOCRRequest(r.Context(), "sim", "failure", 0, time.Since(startTime).Seconds())
				response.ErrorWithRequest(
					w,
					r,
					http.StatusGatewayTimeout,
					response.CodeOCRFailed,
					"OCR engine processing timeout",
				)
				return
			}
			if errors.Is(err, ocr.ErrOCRFailed) {
				telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeOCRFailed)
				telemetry.RecordOCRRequest(r.Context(), "sim", "failure", 0, time.Since(startTime).Seconds())
				response.ErrorWithRequest(
					w,
					r,
					http.StatusBadGateway,
					response.CodeOCRFailed,
					"OCR engine processing timeout or failure",
				)
				return
			}

			telemetry.RecordOCRError(r.Context(), "ocr_engine", response.CodeInternalError)
			telemetry.RecordOCRRequest(r.Context(), "sim", "failure", 0, time.Since(startTime).Seconds())
			response.ErrorWithRequest(
				w,
				r,
				http.StatusInternalServerError,
				response.CodeInternalError,
				"Internal error during document processing",
			)
			return
		}
		engSpan.SetStatus(codes.Ok, "extracted")
		engSpan.End()
		telemetry.RecordOCRStageDuration(r.Context(), "ocr_engine", "success", engDuration)

		if ocrResult == nil || ocrResult.DocumentType != "sim" {
			telemetry.RecordOCRError(r.Context(), "classification", response.CodeUnsupportedDocument)
			telemetry.RecordOCRRequest(r.Context(), "sim", "failure", 0, time.Since(startTime).Seconds())
			response.ErrorWithRequest(
				w,
				r,
				http.StatusUnprocessableEntity,
				response.CodeUnsupportedDocument,
				"Uploaded image was not identified as an Indonesian SIM",
			)
			return
		}

		// Stage 3: Field Extraction & Normalization
		normStart := time.Now()
		_, normSpan := tracer.Start(r.Context(), "ocr.normalize_and_validate")
		normalizedSIM, confidence, _ := ocr.ValidateSIM(ocrResult.SIMData)
		normDuration := time.Since(normStart).Seconds()
		normSpan.SetAttributes(
			attribute.Float64("ocr.confidence", confidence),
			attribute.String("ocr.doc_type", "sim"),
		)
		normSpan.SetStatus(codes.Ok, "normalized")
		normSpan.End()
		telemetry.RecordOCRStageDuration(r.Context(), "field_extraction", "success", normDuration)

		latencyMS := int(time.Since(startTime).Milliseconds())
		totalDurationSec := float64(latencyMS) / 1000.0
		recordID := GenerateUUIDv4()

		// Stage 4: Persist non-PII execution metadata in PostgreSQL
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
				DocType:    "sim",
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

		// Record overall business metric
		ocrStatus := "completed"
		if confidence < 0.7 {
			ocrStatus = "low_confidence"
		}
		telemetry.RecordOCRRequest(r.Context(), "sim", ocrStatus, confidence, totalDurationSec)

		// Zero PII structured log
		slog.InfoContext(r.Context(), "sim ocr request completed",
			slog.String("request_id", response.GetRequestID(r.Context())),
			slog.String("record_id", recordID),
			slog.String("org_id", key.OrgID),
			slog.Int("latency_ms", latencyMS),
			slog.Float64("confidence", confidence),
			slog.String("doc_type", "sim"),
		)

		response.JSON(w, http.StatusOK, SIMResponse{
			ID:           recordID,
			DocumentType: "sim",
			Confidence:   confidence,
			Data:         normalizedSIM,
			Processing: ProcessingDetail{
				LatencyMS: latencyMS,
			},
		})
	}
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
