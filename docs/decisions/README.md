# Architecture Decision Records (ADRs)

> Comprehensive log of architectural decisions shaping the Lensio platform.  
> Following standard lightweight ADR format (Status, Context, Decision, Consequences).

---

## Decision Index

| ADR | Date | Status | Title | Summary |
|---|---|---|---|---|
| [`ADR-001`](./ADR-001-why-go-for-api-and-ocr-service.md) | 2026-09-12 | Accepted | Why Go for the Lensio API and OCR Orchestration Service | Selected Go 1.22+ for sub-80ms cold starts, <20MB idle footprint, and robust concurrency over Python/Node.js. |
| [`ADR-002`](./ADR-002-database-schema-and-api-key-hashing-strategy.md) | 2026-09-12 | Accepted | Database Schema and API Key Hashing Strategy | Adopted CSPRNG tokens with environment prefixes (`lensio_live_`) and one-way SHA-256 hashing for zero plaintext secrets at rest. |
| [`ADR-003`](./ADR-003-rate-limiting-and-quota-architecture.md) | 2026-09-12 | Accepted | Rate Limiting and Quota Architecture | Two-tier architecture: $O(1)$ in-memory token bucket limiter paired with pre-OCR quota checks and non-blocking asynchronous usage metering. |
| [`ADR-004`](./ADR-004-azure-container-apps-vs-kubernetes.md) | 2026-09-12 | Accepted | Azure Container Apps vs. Kubernetes (Deliberate Simplicity) | Selected serverless ACA over full AKS to eliminate cluster management overhead and enable sub-60-second revision rollbacks. |
| [`ADR-005`](./ADR-005-data-minimization-and-pii-protection-in-ocr-pipelines.md) | 2026-09-12 | Accepted | Data Minimization and PII Protection in OCR Pipelines | Enforced ephemeral in-memory processing, zero permanent raw image storage, and strict PII log scrubbing to comply with Indonesian PDP Law. |
| [`ADR-006`](./ADR-006-multi-instance-rate-limiting-tradeoffs.md) | 2026-09-12 | Accepted | Multi-Instance Rate Limiting Trade-offs (Distributed vs. Local) | Evaluated replica drift on Azure Container Apps; retained $O(1)$ in-memory token bucket for current scale ($N \le 3$) while defining pluggable `RateLimiter` interface and scaling triggers for future Redis adapter. |

---

## Guiding Principles

Every architectural decision in this directory adheres to Lensio's foundational philosophy:
> **"Build the smallest real product that forces us to solve real production engineering problems."**  
> We deliberately choose boring, proven, and minimal solutions (Ponytail Rule 1) over unneeded complexity.
