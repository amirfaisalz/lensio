# VeriForm Identity Onboarding Demo Consumer

> Fictional identity onboarding platform demonstrating external consumption of the NusaID KTP OCR API (`PRD Section 3`).

---

## 1. Overview

**VeriForm** is an independent external client application. When a new user signs up, VeriForm collects the user's Indonesian KTP image, calls NusaID's `POST /api/v1/ocr/ktp` endpoint, and applies customer onboarding business rules:
- **OCR Confidence Check**: Requires confidence $\ge 0.75$.
- **Minimum Age Verification**: Asserts the applicant is at least 17 years old.
- **Identity Issuance**: Issues a digital applicant ID (`vf_usr_<id>`) on approval.

```text
User uploads KTP
       ↓
VeriForm Consumer
       ↓
NusaID OCR API (POST /api/v1/ocr/ktp)
       ↓
Structured identity data (NIK, Name, DOB)
       ↓
VeriForm Onboarding Gate (Age >= 17)
       ↓
Account Approved / Rejected
```

---

## 2. Usage

### Prerequisites
- Go 1.22+
- NusaID API running locally or on staging (e.g. `http://localhost:8080`)
- A valid NusaID API Key with `ocr:write` scope

### Running the Demo

```bash
# Set your API key
export NUSAID_API_KEY="nusa_live_xxxxxxxxxxxxxxxxxxxxxxxx"

# Run VeriForm client with default synthetic KTP fixture
go run ./examples/veriform/main.go \
  --api-url "http://localhost:8080" \
  --image "tests/fixtures/synthetic/valid_ktp.jpg" \
  --min-age 17
```

### Command Flags

| Flag | Default | Description |
|---|---|---|
| `--api-url` | `http://localhost:8080` | NusaID base API URL |
| `--api-key` | `$NUSAID_API_KEY` | Bearer API token |
| `--image` | `tests/fixtures/synthetic/valid_ktp.jpg` | Path to KTP image file |
| `--min-age` | `17` | Minimum age required to onboard |

### Example Output (Approved)

```json
{
  "applicant_id": "vf_usr_ocr_01JABC12345",
  "status": "APPROVED",
  "full_name": "BUDI SANTOSO",
  "nik": "3171012345670001",
  "age": 34,
  "ocr_confidence": 0.98,
  "processed_at": "2026-09-12T01:50:00Z"
}
```

---

## 3. Running Automated Tests

```bash
go test -race -v ./examples/veriform/...
```
