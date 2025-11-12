import http from 'k6/http';
import { check } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export const options = {
  scenarios: {
    high_throughput: {
      executor: 'constant-arrival-rate',
      rate: 5000,             // 5000 RPS - requires RATE_LIMIT_RPS=10000
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 50,
      maxVUs: 200,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<150', 'p(99)<300'],
    http_req_failed: ['rate<0.05'],  // Allow higher error rate for stress testing
    errors: ['rate<0.05'],
  },
};

const API_URL = __ENV.API_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || 'dev_key_1234567890';

const eventTypes = ['user_signup', 'purchase', 'page_view'];

function generateEvent() {
  const eventType = eventTypes[Math.floor(Math.random() * eventTypes.length)];

  const payloads = {
    user_signup: {
      user_id: `user_${Math.floor(Math.random() * 100000)}`,
      email: `user${Math.floor(Math.random() * 100000)}@example.com`,
      signup_source: 'web',
      metadata: {},
    },
    purchase: {
      user_id: `user_${Math.floor(Math.random() * 100000)}`,
      order_id: `order_${Math.floor(Math.random() * 1000000)}`,
      amount: 99.99,
      currency: 'USD',
      payment_method: 'card',
      items: [{ product_id: 'prod_123', quantity: 1, price: 99.99 }],
    },
    page_view: {
      user_id: `user_${Math.floor(Math.random() * 100000)}`,
      session_id: `session_${Math.floor(Math.random() * 10000)}`,
      page_url: '/home',
      page_title: 'Home',
      metadata: {},
    },
  };

  return { eventType: eventType, payload: payloads[eventType] };
}

export default function () {
  const event = generateEvent();
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${API_KEY}`,
    },
  };

  const res = http.post(
    `${API_URL}/v1/events/${event.eventType}`,
    JSON.stringify(event.payload),
    params
  );
  const success = check(res, {
    'status is 202': (r) => r.status === 202,
    'has event_id': (r) => JSON.parse(r.body).event_id !== undefined,
  });

  errorRate.add(!success);
}
