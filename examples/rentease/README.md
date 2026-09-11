# RentEase Vehicle Rental Verification Demo Consumer

> Fictional vehicle rental platform demonstrating external consumption of the NusaID KTP OCR API (`PRD Section 3`).

---

## 1. Overview

**RentEase** is an external client platform that automates driver license and identity clearance for self-drive car rentals. When a customer reserves a vehicle, RentEase sends their Indonesian KTP to NusaID's `POST /api/v1/ocr/ktp` endpoint, and executes rental business rules:
- **Image Quality & Confidence**: Rejects documents with confidence $< 0.70$.
- **Minimum Driving Age**: Requires the driver to be at least 21 years old.
- **Citizenship & Permit Clearance**: Optional requirement for Indonesian citizenship (`WNI`) for domestic self-drive clearance.
- **Digital Rental Pass**: Generates a cryptographic reservation pass (`RENTEASE-PASS-<year>-<token>`) valid for 48 hours.

```text
Customer submits KTP
       ↓
RentEase Consumer
       ↓
NusaID OCR API (POST /api/v1/ocr/ktp)
       ↓
KTP fields extracted (Name, DOB, Citizenship)
       ↓
RentEase Rental Gate (Age >= 21, WNI status)
       ↓
Digital Rental Pass Issued / Rejected
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

# Run RentEase verification client
go run ./examples/rentease/main.go \
  --api-url "http://localhost:8080" \
  --image "tests/fixtures/synthetic/valid_ktp.jpg" \
  --min-age 21 \
  --vehicle-class "SUV"
```

### Command Flags

| Flag | Default | Description |
|---|---|---|
| `--api-url` | `http://localhost:8080` | NusaID base API URL |
| `--api-key` | `$NUSAID_API_KEY` | Bearer API token |
| `--image` | `tests/fixtures/synthetic/valid_ktp.jpg` | Path to KTP image file |
| `--min-age` | `21` | Minimum required driver age |
| `--require-wni` | `false` | Restrict rental to Indonesian citizens |
| `--vehicle-class` | `SUV` | Target vehicle classification |

### Example Output (Approved)

```json
{
  "rental_pass_id": "RENTEASE-PASS-2026-e4a8b1c9",
  "status": "APPROVED",
  "driver_name": "BUDI SANTOSO",
  "driver_age": 34,
  "citizenship": "WNI",
  "vehicle_class": "SUV",
  "ocr_confidence": 0.98,
  "issued_at": "2026-09-12T01:50:00Z",
  "expires_at": "2026-09-14T01:50:00Z"
}
```

---

## 3. Running Automated Tests

```bash
go test -race -v ./examples/rentease/...
```
