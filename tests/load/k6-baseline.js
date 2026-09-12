import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Binary fixture loading for synthetic KTP image upload
const validKtpPng = open('../fixtures/synthetic/valid_ktp.png', 'b');

// Custom metrics for granular RED metrics analysis
export const healthDuration = new Trend('health_duration', true);
export const authVerifyDuration = new Trend('auth_verify_duration', true);
export const usageDuration = new Trend('usage_duration', true);
export const ocrDuration = new Trend('ocr_duration', true);
export const rateLimitHits = new Counter('rate_limit_hits');
export const successRate = new Rate('successful_reqs');
export const serverErrors = new Counter('server_errors');

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TARGET_VUS = parseInt(__ENV.VUS || '100', 10);
const TEST_DURATION = __ENV.DURATION || '2m';

export const options = {
  stages: [
    { duration: '15s', target: Math.floor(TARGET_VUS * 0.5) }, // Ramp-up
    { duration: '15s', target: TARGET_VUS },                  // Full target
    { duration: TEST_DURATION, target: TARGET_VUS },          // Steady state
    { duration: '15s', target: 0 },                           // Ramp-down
  ],
  thresholds: {
    // 95% of requests should complete under 500ms, 99% under 1000ms
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    // Server errors (5xx) must remain under 10
    server_errors: ['count<10'],
    // Overall success rate (non-5xx)
    successful_reqs: ['rate>0.99'],
  },
};

// setup() executes once before VUs start, generating an API key if not supplied
export function setup() {
  if (__ENV.API_KEY) {
    return { apiKey: __ENV.API_KEY };
  }

  const keyPayload = JSON.stringify({
    name: 'k6-baseline-loadtest-key',
    environment: 'loadtest',
    scopes: ['ocr:write', 'ocr:read', 'usage:read'],
  });

  const res = http.post(`${BASE_URL}/api/v1/auth/api-keys`, keyPayload, {
    headers: { 'Content-Type': 'application/json' },
  });

  if (res.status === 201) {
    const body = JSON.parse(res.body);
    return { apiKey: body.key };
  }

  // If server is in ephemeral mode without DB or if key creation failed
  return { apiKey: 'lensio_live_ephemeral_test_key_dummy' };
}

export default function (data) {
  const apiKey = data.apiKey;
  const authHeaders = {
    Authorization: `Bearer ${apiKey}`,
  };

  // Distribution: Realistic mix of read probes, authenticated queries, and OCR processing
  const rand = Math.random();

  if (rand < 0.35) {
    // 1. Health Probe (unauthenticated, probe path)
    const t0 = Date.now();
    const res = http.get(`${BASE_URL}/health`);
    healthDuration.add(Date.now() - t0);

    const isOk = check(res, {
      'health status 200': (r) => r.status === 200,
      'health status ok': (r) => {
        try {
          return JSON.parse(r.body).status === 'ok';
        } catch {
          return false;
        }
      },
      'health has request id': (r) => r.headers['X-Request-Id'] !== undefined,
    });

    if (!isOk && res.status >= 500) {
      serverErrors.add(1);
    }
    successRate.add(res.status < 500);

  } else if (rand < 0.60) {
    // 2. Auth Verification (authenticated, cached key lookup, scope check)
    const t0 = Date.now();
    const res = http.get(`${BASE_URL}/api/v1/auth/verify`, { headers: authHeaders });
    authVerifyDuration.add(Date.now() - t0);

    const isRateLimited = res.status === 429;
    if (isRateLimited) {
      rateLimitHits.add(1);
    }

    const isOk = check(res, {
      'verify status 200 or 429': (r) => r.status === 200 || r.status === 429,
      'verify rate limit header present': (r) => r.headers['X-Ratelimit-Limit'] !== undefined,
    });

    if (!isOk && res.status >= 500) {
      serverErrors.add(1);
    }
    successRate.add(res.status < 500);

  } else if (rand < 0.80) {
    // 3. Usage Analytics (authenticated, DB aggregation query)
    const t0 = Date.now();
    const res = http.get(`${BASE_URL}/api/v1/usage`, { headers: authHeaders });
    usageDuration.add(Date.now() - t0);

    const isRateLimited = res.status === 429;
    if (isRateLimited) {
      rateLimitHits.add(1);
    }

    const isOk = check(res, {
      'usage status 200 or 429': (r) => r.status === 200 || r.status === 429,
    });

    if (!isOk && res.status >= 500) {
      serverErrors.add(1);
    }
    successRate.add(res.status < 500);

  } else {
    // 4. KTP OCR Processing (multipart synthetic payload, validation & extraction)
    const payload = {
      document: http.file(validKtpPng, 'valid_ktp.png', 'image/png'),
    };

    const t0 = Date.now();
    const res = http.post(`${BASE_URL}/api/v1/ocr/ktp`, payload, { headers: authHeaders });
    ocrDuration.add(Date.now() - t0);

    const isRateLimited = res.status === 429;
    if (isRateLimited) {
      rateLimitHits.add(1);
    }

    const isOk = check(res, {
      'ocr status 200 or 429': (r) => r.status === 200 || r.status === 429,
      'ocr response valid if 200': (r) => {
        if (r.status === 200) {
          try {
            const body = JSON.parse(r.body);
            return body.id !== undefined && body.document_type === 'ktp';
          } catch {
            return false;
          }
        }
        return true;
      },
    });

    if (!isOk && res.status >= 500) {
      serverErrors.add(1);
    }
    successRate.add(res.status < 500);
  }

  // Small jitter pause between VU requests (50ms - 150ms)
  sleep(0.05 + Math.random() * 0.1);
}
