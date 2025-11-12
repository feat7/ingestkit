import http from 'k6/http';
import { check } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export const options = {
  scenarios: {
    quick: {
      executor: 'constant-arrival-rate',
      rate: 100,              // 100 RPS
      timeUnit: '1s',
      duration: '10s',
      preAllocatedVUs: 5,
      maxVUs: 20,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<100'],
    http_req_failed: ['rate<0.01'],
  },
};

const API_URL = __ENV.API_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || 'dev_key_1234567890';

export default function () {
  const payload = {
    user_id: `user_${Math.floor(Math.random() * 10000)}`,
    email: `test${Math.floor(Math.random() * 10000)}@example.com`,
    signup_source: 'web',
    metadata: {},
  };

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${API_KEY}`,
    },
  };

  const res = http.post(`${API_URL}/v1/events/user_signup`, JSON.stringify(payload), params);
  const success = check(res, {
    'status is 202': (r) => r.status === 202,
    'has event_id': (r) => JSON.parse(r.body).event_id !== undefined,
  });

  errorRate.add(!success);
}
