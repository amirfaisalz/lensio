package telemetry

import (
	"context"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/amirfaisalz/lensio/apps/api/internal/http/response"
	"go.opentelemetry.io/otel/trace"
)

var (
	// nikRegex matches exact 16-digit sequences representing Indonesian NIKs.
	nikRegex = regexp.MustCompile(`\b\d{16}\b`)

	// sensitiveKeys defines the set of lowercased attribute keys containing sensitive PII or credentials.
	sensitiveKeys = map[string]struct{}{
		"nik":           {},
		"nama":          {},
		"name":          {},
		"full_name":     {},
		"fullname":      {},
		"address":       {},
		"alamat":        {},
		"rt_rw":         {},
		"kelurahan":     {},
		"kecamatan":     {},
		"birth_date":    {},
		"tanggal_lahir": {},
		"tempat_lahir":  {},
		"image":         {},
		"document":      {},
		"raw_image":     {},
		"buffer":        {},
		"token":         {},
		"password":      {},
		"key_hash":      {},
		"secret":        {},
		"authorization": {},
		"api_key":       {},
	}
)

const (
	// RedactedValue is the replacement text for sensitive PII and credential attributes.
	RedactedValue = "[REDACTED]"
	// RedactedImageData is the replacement text for base64 or raw image buffers.
	RedactedImageData = "[REDACTED_IMAGE_DATA]"
)

// PIISanitizerHandler wraps an slog.Handler to inject OpenTelemetry trace/span correlation,
// request_id correlation, and strictly redact/mask all Personally Identifiable Information (PII).
type PIISanitizerHandler struct {
	handler slog.Handler
}

// NewPIISanitizerHandler constructs an slog.Handler that strips PII and correlates logs.
func NewPIISanitizerHandler(delegate slog.Handler) *PIISanitizerHandler {
	return &PIISanitizerHandler{
		handler: delegate,
	}
}

// Enabled implements slog.Handler.
func (h *PIISanitizerHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle processes the log record by adding correlation IDs, sanitizing the message,
// and redacting any sensitive PII attributes before delegating.
func (h *PIISanitizerHandler) Handle(ctx context.Context, r slog.Record) error {
	// 1. Sanitize the main message string
	sanitizedMsg := sanitizeString(r.Message)
	r.Message = sanitizedMsg

	// 2. Extract and correlate OpenTelemetry trace_id and span_id
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}

	// 3. Extract and correlate request_id
	if reqID := response.GetRequestID(ctx); reqID != "" {
		hasReqID := false
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "request_id" {
				hasReqID = true
				return false
			}
			return true
		})
		if !hasReqID {
			r.AddAttrs(slog.String("request_id", reqID))
		}
	}

	// 4. Sanitize all attributes attached to the record
	var sanitizedAttrs []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		sanitizedAttrs = append(sanitizedAttrs, sanitizeAttr(a))
		return true
	})

	// Reconstruct record with sanitized attributes
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	newRecord.AddAttrs(sanitizedAttrs...)

	return h.handler.Handle(ctx, newRecord)
}

// WithAttrs returns a new handler with sanitized attributes.
func (h *PIISanitizerHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitized := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		sanitized[i] = sanitizeAttr(a)
	}
	return &PIISanitizerHandler{
		handler: h.handler.WithAttrs(sanitized),
	}
}

// WithGroup returns a new handler with the given group name.
func (h *PIISanitizerHandler) WithGroup(name string) slog.Handler {
	return &PIISanitizerHandler{
		handler: h.handler.WithGroup(name),
	}
}

// sanitizeAttr inspects an attribute for sensitive keys or values.
func sanitizeAttr(a slog.Attr) slog.Attr {
	// 1. Key-based check (O(1) map lookup)
	keyLower := strings.ToLower(a.Key)
	if _, isSensitive := sensitiveKeys[keyLower]; isSensitive {
		return slog.String(a.Key, RedactedValue)
	}

	// 2. Value-based inspection
	switch a.Value.Kind() {
	case slog.KindString:
		val := a.Value.String()
		if isImageString(val) {
			return slog.String(a.Key, RedactedImageData)
		}
		if containsSensitivePatterns(val) {
			return slog.String(a.Key, sanitizeString(val))
		}
		return a

	case slog.KindGroup:
		groupAttrs := a.Value.Group()
		sanitizedGroup := make([]slog.Attr, len(groupAttrs))
		for i, subAttr := range groupAttrs {
			sanitizedGroup[i] = sanitizeAttr(subAttr)
		}
		return slog.Group(a.Key, anySlice(sanitizedGroup)...)

	default:
		return a
	}
}

// sanitizeString replaces any 16-digit NIK sequence with a full redaction marker.
func sanitizeString(s string) string {
	if len(s) < 16 {
		return s
	}
	return nikRegex.ReplaceAllString(s, RedactedValue)
}

// containsSensitivePatterns quickly checks if a string might contain an Indonesian NIK.
func containsSensitivePatterns(s string) bool {
	if len(s) < 16 {
		return false
	}
	return nikRegex.MatchString(s)
}

// isImageString detects base64 image data strings.
func isImageString(s string) bool {
	if strings.HasPrefix(s, "data:image/") {
		return true
	}
	// Check for raw base64 jpeg/png headers if length > 128
	if len(s) > 128 {
		if strings.HasPrefix(s, "/9j/") || strings.HasPrefix(s, "iVBORw0KGgo") {
			return true
		}
	}
	return false
}

func anySlice(attrs []slog.Attr) []any {
	res := make([]any, len(attrs))
	for i, a := range attrs {
		res[i] = a
	}
	return res
}

// InitLogger initializes and configures the default application structured logger
// with JSON formatting and the PII sanitizer handler.
func InitLogger(level slog.Level, out io.Writer) *slog.Logger {
	if out == nil {
		out = os.Stdout
	}
	baseHandler := slog.NewJSONHandler(out, &slog.HandlerOptions{
		Level: level,
	})
	sanitizedHandler := NewPIISanitizerHandler(baseHandler)
	return slog.New(sanitizedHandler)
}
