package handlers

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/amirfaisalz/nusaid/apps/api/internal/http/middleware"
	"github.com/amirfaisalz/nusaid/apps/api/internal/http/response"
	"github.com/amirfaisalz/nusaid/apps/api/internal/store"
	"github.com/amirfaisalz/nusaid/services/ocr"
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

// OCRRequestMetadata represents the non-PII execution metadata query response.
type OCRRequestMetadata struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	DocType    string    `json:"doc_type"`
	Confidence float64   `json:"confidence"`
	LatencyMS  int       `json:"latency_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

// KTPOCRHandler processes an uploaded Indonesian KTP image, performs classification,
// deterministic field normalization/validation, and returns structured data.
// Uploaded image buffers are strictly processed in memory and discarded immediately.
func KTPOCRHandler(engine ocr.OCREngine, ocrStore store.OCRRequestStore) http.HandlerFunc {
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

		// Strictly validate magic bytes and dimensions
		if _, err := ocr.ValidateImage(imgBytes); err != nil {
			response.ErrorWithRequest(
				w,
				r,
				http.StatusBadRequest,
				response.CodeInvalidDocument,
				err.Error(),
			)
			return
		}

		if engine == nil {
			response.ErrorWithRequest(
				w,
				r,
				http.StatusBadGateway,
				response.CodeOCRFailed,
				"OCR engine not available",
			)
			return
		}

		ocrResult, err := engine.Extract(r.Context(), imgBytes)
		// Immediately release reference to image bytes
		imgBytes = nil

		if err != nil {
			if errors.Is(err, ocr.ErrUnsupportedDocument) {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusUnprocessableEntity,
					response.CodeUnsupportedDocument,
					"Uploaded image was not identified as an Indonesian KTP",
				)
				return
			}
			if errors.Is(err, ocr.ErrOCRFailed) {
				response.ErrorWithRequest(
					w,
					r,
					http.StatusBadGateway,
					response.CodeOCRFailed,
					"OCR engine processing timeout or failure",
				)
				return
			}

			response.ErrorWithRequest(
				w,
				r,
				http.StatusInternalServerError,
				response.CodeInternalError,
				"Internal error during document processing",
			)
			return
		}

		if ocrResult == nil || ocrResult.DocumentType != "ktp" {
			response.ErrorWithRequest(
				w,
				r,
				http.StatusUnprocessableEntity,
				response.CodeUnsupportedDocument,
				"Uploaded image was not identified as an Indonesian KTP",
			)
			return
		}

		// Run deterministic validation & normalization
		normalizedKTP, confidence, _ := ocr.ValidateKTP(ocrResult.Data)
		latencyMS := int(time.Since(startTime).Milliseconds())
		recordID := GenerateUUIDv4()

		// Persist non-PII execution metadata in PostgreSQL
		if ocrStore != nil {
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
				slog.ErrorContext(r.Context(), "failed saving ocr request metadata",
					slog.String("error", err.Error()),
					slog.String("record_id", recordID),
				)
			}
		}

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
