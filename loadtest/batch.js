import http from 'k6/http';
import { check } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export const options = {
  scenarios: {
    batch: {
      executor: 'constant-arrival-rate',
      rate: 100,              // 100 RPS with 10 events/batch = 1000 EPS
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 10,
      maxVUs: 30,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<200', 'p(99)<500'],
    http_req_failed: ['rate<0.01'],
    errors: ['rate<0.01'],
  },
};

const API_URL = __ENV.API_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || 'dev_key_1234567890';
const BATCH_SIZE = parseInt(__ENV.BATCH_SIZE || '10');

const eventTypes = ['user_signup', 'purchase', 'page_view'];

function generateEvent() {
  const eventType = eventTypes[Math.floor(Math.random() * eventTypes.length)];

  const payloads = {
    user_signup: {
      user_id: `user_${Math.floor(Math.random() * 100000)}`,
      email: `user${Math.floor(Math.random() * 100000)}@example.com`,
      signup_source: ['web', 'mobile', 'api'][Math.floor(Math.random() * 3)],
      metadata: {},
    },
    purchase: {
      user_id: `user_${Math.floor(Math.random() * 100000)}`,
      order_id: `order_${Math.floor(Math.random() * 1000000)}`,
      amount: Math.floor(Math.random() * 50000) / 100,
      currency: 'USD',
      payment_method: 'card',
      items: [{
        product_id: `prod_${Math.floor(Math.random() * 1000)}`,
        quantity: 1,
        price: 10.0,
      }],
    },
    page_view: {
      user_id: `user_${Math.floor(Math.random() * 100000)}`,
      session_id: `session_${Math.floor(Math.random() * 10000)}`,
      page_url: `/page/${Math.floor(Math.random() * 100)}`,
      page_title: `Page ${Math.floor(Math.random() * 100)}`,
      metadata: {},
    },
  };

  return { eventType: eventType, payload: payloads[eventType] };
}

export default function () {
  // Pick a random event type for this batch
  const eventType = eventTypes[Math.floor(Math.random() * eventTypes.length)];

  // Generate batch of events of the same type
  const payloads = [];
  for (let i = 0; i < BATCH_SIZE; i++) {
    const event = generateEvent();
    // Only use events of the selected type
    if (event.eventType === eventType) {
      payloads.push(event.payload);
    } else {
      // Regenerate until we get the right type (simpler approach)
      i--;
    }
  }

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${API_KEY}`,
    },
  };

  const res = http.post(
    `${API_URL}/v1/events/${eventType}/batch`,
    JSON.stringify({ events: payloads }),
    params
  );

  const success = check(res, {
    'status is 202': (r) => r.status === 202,
    'has event_ids': (r) => {
      const body = JSON.parse(r.body);
      return body.event_ids && body.event_ids.length === BATCH_SIZE;
    },
    'response time < 1s': (r) => r.timings.duration < 1000,
  });

  errorRate.add(!success);
}
