# ADR-005: Data Minimization and PII Protection in OCR Pipelines

- **Status**: Accepted
- **Date**: 2026-09-12
- **Deciders**: NusaID Engineering Team
- **Technical Context**: `PRD Section 13`, `PRD Section 14`, `PRD Section 15`, `AGENTS.md` (Security Non-Negotiables #2, #3, #4)

---

## 1. Context and Problem Statement

The Indonesian Kartu Tanda Penduduk (KTP) is the primary legal national identity document in Indonesia. It contains critical personally identifiable information (PII), including:
- **NIK (Nomor Induk Kependudukan)**: A 16-digit unique national identity number.
- **Full Legal Name & Place/Date of Birth**
- **Home Address, RT/RW, Kelurahan, Kecamatan**
- **Religion, Marital Status, Occupation, and Citizenship**
- **Facial Photograph and Handwritten Signature**

Under Indonesia's Personal Data Protection Law (**UU No. 27 Tahun 2022 tentang Pelindungan Data Pribadi - UU PDP**), handling sensitive identity documents requires strict principles of **purpose limitation**, **data minimization**, and **storage limitation**. A security breach or unintentional exposure of KTP images or NIKs in application logs or database backups constitutes a severe legal violation and privacy breach.

We needed an explicit architecture that delivers OCR extraction while strictly eliminating persistent identity exposure.

---

## 2. Decision Drivers

- **Compliance with UU PDP**: Adhere to legal principles of data minimization and confidentiality.
- **Minimization of Blast Radius**: If the NusaID database or log aggregation systems (e.g. Grafana Loki, CloudWatch) are compromised, zero citizen PII or document scans must be retrievable.
- **Zero Raw Image Storage**: NusaID is an extraction API, not a document archive.
- **Deterministic Validation**: Never trust raw LLM/vision text blindly; enforce deterministic validation on NIK and dates.
- **Operational Debuggability**: Maintain the ability to debug failed requests using non-PII correlation identifiers (`request_id`, `trace_id`, `span_id`).

---

## 3. Considered Options

### Option 1: Store Raw Images in Cloud Object Storage (Azure Blob / S3)
- *Pros*: Allows offline re-processing of failed OCR runs, model fine-tuning, and long-term customer audit retrieval.
- *Cons*: Extremely high security liability. Requires maintaining encrypted storage vaults, access keys, retention deletion jobs, and increases GDPR/PDP compliance risk exponentially.

### Option 2: Masked Image Storage & Redacted Text Logs
- *Pros*: Preserves blurred thumbnails for dashboard preview.
- *Cons*: High implementation complexity, irreversible redaction errors, and ongoing risk of unmasked data leakage.

### Option 3: Strict Data Minimization Pipeline (Selected)
- *Pros*:
  - **Ephemeral Memory Processing**: The uploaded multipart file is buffered in RAM, streamed to the vision inference engine (Gemini Flash or Mock fixture), and the memory buffer is immediately reclaimed by the Go runtime garbage collector. No image is written to local disk or cloud object storage.
  - **Zero PII in Application Logs**: Application loggers (`log/slog`) are strictly prohibited from logging NIK, names, addresses, or raw response text. Log messages record only operational metadata: `request_id`, `trace_id`, `org_id`, `status_code`, `confidence`, and `latency_ms`.
  - **Deterministic Boundary Validation**: Before returning the response, the extracted NIK is validated deterministically:
    1. Exactly 16 numeric digits.
    2. Valid Indonesian province code prefix (e.g., `31` for DKI Jakarta, `32` for West Java).
    3. Valid birth date encoding within the NIK (digits 7–12, factoring in the +40 offset for female citizens).
  - **Stateless Extraction Contract**: The API caller is responsible for archiving their own customer documents. NusaID acts purely as a stateless processor.
- *Cons*: Cannot re-run extraction on failed images without the client re-uploading the image.

---

## 4. Decision Outcome

**Chosen Option**: **Option 3 (Strict Data Minimization Pipeline)**

### Data Flow & Isolation Architecture:

```text
Client Upload (HTTPS)
      │
      ▼
[In-Memory Buffer]  ──► [Magic Byte / MIME Check]  (Pure Go, max 5MB)
      │
      ▼
[OCREngine Interface]
      │
      ├──► Streaming payload to Gemini Flash Vision API (TLS 1.3)
      │    (Prompt enforces JSON schema response)
      │
      ▼
[Extraction Pipeline]
      │
      ├──► Deterministic NIK & Date Validation (Regex + checksum)
      ├──► Compute Confidence Score
      │
      ▼
[Client Response: 200 OK] (Returns structured JSON to authenticated client)
      │
      ▼
[Ephemeral Memory Released] (Buffer garbage collected; zero disk/blob writes)

Application Logs:
  INFO ocr_completed request_id=req_01JABC status=200 latency_ms=1420 confidence=0.98
  (ZERO NIK, ZERO NAMES, ZERO IMAGES IN LOGS)
```

---

## 5. Consequences

### Positive
- **Guaranteed PDP Compliance**: Minimizes regulatory and legal exposure under Indonesian privacy laws.
- **Zero PII Leakage Risk**: Even a full root-level database dump will expose only tenant organization accounts, hashed API keys, and aggregate request counters—zero citizen names, NIKs, or pictures.
- **High Performance & Low Storage Costs**: Zero disk I/O and zero blob storage transfer costs; lower latency since no multi-part S3 upload step is needed.

### Negative / Trade-offs
- If a client reports an inaccurate extraction, engineers cannot inspect the original image from NusaID backend storage. The client must submit their anonymized synthetic test image during support inquiries.

---

## 6. References
- `AGENTS.md` - Security & Privacy Non-Negotiables
- `PRD Section 13` - Image Preprocessing & Validation
- `PRD Section 14` - Field Normalization & Validation
- `PRD Section 15` - Data Minimization & Privacy
- UU No. 27 Tahun 2022 tentang Pelindungan Data Pribadi (UU PDP)
