package ocr

import "context"

// OCREngine defines the pluggable document extraction contract.
type OCREngine interface {
	Extract(ctx context.Context, image []byte) (*OCRResult, error)
}

// OCRResult represents the structured result of an OCR extraction.
type OCRResult struct {
	DocumentType string         `json:"document_type"`
	Confidence   float64        `json:"confidence"`
	Data         map[string]any `json:"data"`
}
