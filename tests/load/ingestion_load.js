import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

export const options = {
  stages: [
    { duration: '30s', target: 50 },  // Ramp up to 50 virtual users
    { duration: '1m', target: 200 },  // Sustained burst of 200 virtual users
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<100', 'p(99)<250'], // 95% under 100ms, 99% under 250ms
    http_req_failed: ['rate<0.01'],                 // Less than 1% failure
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || 'rf_live_loadtest_dummy_key';
const APP_ID = __ENV.APP_ID || '00000000-0000-0000-0000-000000000001';

export default function () {
  const idempotencyKey = `k6_idemp_${uuidv4()}`;
  
  const payload = JSON.stringify({
    event_type: 'invoice.paid',
    payload: {
      order_id: `ord_${Math.floor(Math.random() * 1000000)}`,
      amount_cents: Math.floor(Math.random() * 50000) + 100,
      currency: 'USD',
      customer_email: 'test@example.com',
    },
    metadata: {
      source: 'k6-load-generator',
      timestamp: new Date().toISOString(),
    },
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${API_KEY}`,
      'Idempotency-Key': idempotencyKey,
    },
  };

  const res = http.post(`${BASE_URL}/api/v1/apps/${APP_ID}/events`, payload, params);

  check(res, {
    'status is 202 Accepted': (r) => r.status === 202,
    'has event_id': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.data && body.data.event_id !== undefined;
      } catch (e) {
        return false;
      }
    },
  });

  sleep(0.05); // 50ms pause between requests
}

