# Lensio Load Testing Suite (`k6`)

Comprehensive performance, load, and stress test suites for the Lensio KTP OCR API.

---

## 1. Overview

This directory provides automated load testing scripts using [k6](https://k6.io/):

| Script | Profile & Workload | Primary Engineering Focus |
|---|---|---|
| [`k6-baseline.js`](./k6-baseline.js) | 100 concurrent VUs over 2 minutes | Empirical baseline throughput, RED metrics ($P_{50}, P_{90}, P_{95}, P_{99}$ latency), zero-error rate |
| [`k6-stress.js`](./k6-stress.js) | Ramp-up to 500 RPS & 1,000 RPS | Saturation point, in-memory token bucket `sync.Mutex` lock contention, rate limiter 429 distribution, PostgreSQL connection pool contention |

---

## 2. Prerequisites

You can run these tests either with a native `k6` binary or using the official `grafana/k6` Docker image.

### Option A: Native k6 (Recommended if installed)
```bash
# Verify installation
k6 version
```

### Option B: Docker (Zero Installation)
```bash
docker run --rm grafana/k6 version
```

---

## 3. Running Load Tests

The helper script `scripts/run-load-tests.sh` handles automated execution, server discovery, and metric collection:

```bash
# Run baseline load test (100 VUs over 2 minutes)
./scripts/run-load-tests.sh baseline

# Run stress load test (ramp-up to 1,000 RPS)
./scripts/run-load-tests.sh stress

# Run both suites sequentially
./scripts/run-load-tests.sh all
```

### Manual Native Execution

```bash
# 1. Baseline Test
k6 run \
  -e BASE_URL="http://localhost:8080" \
  -e VUS=100 \
  -e DURATION="2m" \
  tests/load/k6-baseline.js

# 2. Stress Test
k6 run \
  -e BASE_URL="http://localhost:8080" \
  -e STAGE_SECS=30 \
  tests/load/k6-stress.js
```

### Manual Docker Execution

```bash
# Baseline Test via Docker
docker run --rm -i \
  --network=host \
  -v "$(pwd):/lensio" \
  -w /lensio/tests/load \
  grafana/k6 run \
  -e BASE_URL="http://localhost:8080" \
  -e VUS=100 \
  -e DURATION="2m" \
  k6-baseline.js

# Stress Test via Docker
docker run --rm -i \
  --network=host \
  -v "$(pwd):/lensio" \
  -w /lensio/tests/load \
  grafana/k6 run \
  -e BASE_URL="http://localhost:8080" \
  -e STAGE_SECS=30 \
  k6-stress.js
```

---

## 4. Configuration Parameters

| Parameter | Default | Description |
|---|---|---|
| `BASE_URL` | `http://localhost:8080` | Target Lensio API base URL |
| `API_KEY` | *(Generated dynamically)* | Pre-provisioned API key (if omitted, `setup()` generates one) |
| `VUS` | `100` | Number of concurrent Virtual Users for baseline suite |
| `DURATION` | `2m` | Steady-state duration for baseline suite |
| `STAGE_SECS` | `30` | Duration per ramp stage in stress suite |

---

## 5. Correlating RED Metrics in Prometheus & Grafana

While tests run, Prometheus scrapes Lensio at `:8080/metrics`.
Open the pre-provisioned Grafana dashboard at `http://localhost:3000` (credentials: `admin`/`admin`):
- **Dashboard**: `Lensio - RED Metrics & System Overview` (`uid: lensio-red-metrics`)
- **Panels**:
  - HTTP Throughput (req/s)
  - HTTP Error Rate (%)
  - Latency Percentiles ($P_{50}, P_{95}, P_{99}$)
  - Rate Limiter 429 Rejections / Sec
  - PostgreSQL Connection Pool State (`open`, `in_use`, `idle`, `wait_count`)
