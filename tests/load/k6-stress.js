import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Binary fixture loading for synthetic KTP image upload
const validKtpPng = open('../fixtures/synthetic/valid_ktp.png', 'b');

// Metrics for stress evaluation
export const stressHealthDuration = new Trend('stress_health_duration', true);
export const stressAuthDuration = new Trend('stress_auth_duration', true);
export const stressUsageDuration = new Trend('stress_usage_duration', true);
export const stressOcrDuration = new Trend('stress_ocr_duration', true);
export const stress429RateLimitHits = new Counter('stress_429_rate_limit_hits');
export const stressServerErrors = new Counter('stress_server_errors');
export const stressSuccessRate = new Rate('stress_success_rate');

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const STAGE_SECS = parseInt(__ENV.STAGE_SECS || '30', 10);

export const options = {
  scenarios: {
    stress_ramp_arrival: {
      executor: 'ramping-arrival-rate',
      startRate: 50,
      timeUnit: '1s',
      preAllocatedVUs: 150,
      maxVUs: 1200,
      stages: [
        { target: 100, duration: `${Math.max(10, Math.floor(STAGE_SECS * 0.5))}s` }, // Warm up to 100 rps
        { target: 500, duration: `${STAGE_SECS}s` },                                 // Ramp to 500 rps
        { target: 500, duration: `${STAGE_SECS}s` },                                 // Sustain 500 rps
        { target: 1000, duration: `${STAGE_SECS}s` },                                // Ramp to 1000 rps
        { target: 1000, duration: `${STAGE_SECS}s` },                                // Sustain 1000 rps
        { target: 0, duration: `${Math.max(10, Math.floor(STAGE_SECS * 0.5))}s` },   // Recovery / ramp down
      ],
    },
  },
  thresholds: {
    // Under extreme stress up to 1,000 RPS, ensure zero 5xx server crashes
    stress_server_errors: ['count<20'],
    stress_success_rate: ['rate>0.95'],
    // Duration threshold across all requests
    http_req_duration: ['p(90)<1000', 'p(95)<2000'],
  },
};

export function setup() {
  if (__ENV.API_KEY) {
    return { apiKey: __ENV.API_KEY };
  }

  // Attempt to authenticate via session token if available or dev token
  let sessionToken = __ENV.SESSION_TOKEN || 'mock_jwt_admin';

  const keyPayload = JSON.stringify({
    name: 'k6-stress-loadtest-key',
    environment: 'stress',
    scopes: ['ocr:write', 'ocr:read', 'usage:read'],
  });

  const res = http.post(`${BASE_URL}/api/v1/auth/api-keys`, keyPayload, {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${sessionToken}`,
    },
  });

  if (res.status === 201) {
    const body = JSON.parse(res.body);
    return { apiKey: body.key };
  }

  // If server rejected unauthenticated key creation, try unauthenticated POST fallback
  const unauthRes = http.post(`${BASE_URL}/api/v1/auth/api-keys`, keyPayload, {
    headers: { 'Content-Type': 'application/json' },
  });

  if (unauthRes.status === 201) {
    const body = JSON.parse(unauthRes.body);
    return { apiKey: body.key };
  }

  // Fail explicitly rather than silently masking load test errors with an invalid key
  throw new Error(`k6 setup failed to create API key (status ${res.status}: ${res.body}). Pass a valid key via -e API_KEY=<key>.`);
}

export default function (data) {
  const apiKey = data.apiKey;
  const authHeaders = {
    Authorization: `Bearer ${apiKey}`,
  };

  const rand = Math.random();

  if (rand < 0.40) {
    // 1. High Throughput Probe (/health) - evaluates raw HTTP scheduling & middleware
    const t0 = Date.now();
    const res = http.get(`${BASE_URL}/health`);
    stressHealthDuration.add(Date.now() - t0);

    const isOk = check(res, {
      'stress health status 200': (r) => r.status === 200,
    });

    if (!isOk && res.status >= 500) {
      stressServerErrors.add(1);
    }
    stressSuccessRate.add(res.status < 500);

  } else if (rand < 0.70) {
    // 2. Token Bucket Mutex & Refill Contention (/api/v1/auth/verify)
    // Stresses sync.Mutex locks in Limiter.Allow() and plan cache lookups
    const t0 = Date.now();
    const res = http.get(`${BASE_URL}/api/v1/auth/verify`, { headers: authHeaders });
    stressAuthDuration.add(Date.now() - t0);

    if (res.status === 429) {
      stress429RateLimitHits.add(1);
    }

    const isOk = check(res, {
      'stress auth valid code': (r) => r.status === 200 || r.status === 429,
      'stress 429 has retry-after': (r) => {
        if (r.status === 429) {
          return r.headers['Retry-After'] !== undefined;
        }
        return true;
      },
    });

    if (!isOk && res.status >= 500) {
      stressServerErrors.add(1);
    }
    stressSuccessRate.add(res.status < 500);

  } else if (rand < 0.85) {
    // 3. DB Connection Pool & Transaction Contention (/api/v1/usage)
    const t0 = Date.now();
    const res = http.get(`${BASE_URL}/api/v1/usage`, { headers: authHeaders });
    stressUsageDuration.add(Date.now() - t0);

    if (res.status === 429) {
      stress429RateLimitHits.add(1);
    }

    const isOk = check(res, {
      'stress usage valid code': (r) => r.status === 200 || r.status === 429,
    });

    if (!isOk && res.status >= 500) {
      stressServerErrors.add(1);
    }
    stressSuccessRate.add(res.status < 500);

  } else {
    // 4. Memory Allocations & Multipart OCR Processing (POST /api/v1/ocr/ktp)
    const payload = {
      document: http.file(validKtpPng, 'valid_ktp.png', 'image/png'),
    };

    const t0 = Date.now();
    const res = http.post(`${BASE_URL}/api/v1/ocr/ktp`, payload, { headers: authHeaders });
    stressOcrDuration.add(Date.now() - t0);

    if (res.status === 429) {
      stress429RateLimitHits.add(1);
    }

    const isOk = check(res, {
      'stress ocr valid code': (r) => r.status === 200 || r.status === 429,
    });

    if (!isOk && res.status >= 500) {
      stressServerErrors.add(1);
    }
    stressSuccessRate.add(res.status < 500);
  }
}
