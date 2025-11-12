import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

// Test configuration
export const options = {
  scenarios: {
    baseline: {
      executor: 'constant-arrival-rate',
      rate: 500,              // 500 RPS
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 10,    // Start with 10 VUs
      maxVUs: 50,             // Scale up to 50 if needed
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<100', 'p(99)<200'], // 95% < 100ms, 99% < 200ms
    http_req_failed: ['rate<0.01'],                 // Error rate < 1%
    errors: ['rate<0.01'],
  },
};

const API_URL = __ENV.API_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || 'dev_key_1234567890';

const eventTypes = ['user_signup', 'purchase', 'page_view'];

// Generate random event payload
function generateEvent() {
  const eventType = eventTypes[Math.floor(Math.random() * eventTypes.length)];

  const payloads = {
    user_signup: {
      user_id: `user_${Math.floor(Math.random() * 100000)}`,
      email: `user${Math.floor(Math.random() * 100000)}@example.com`,
      signup_source: ['web', 'mobile', 'api'][Math.floor(Math.random() * 3)],
      metadata: {
        ip: `192.168.1.${Math.floor(Math.random() * 255)}`,
        user_agent: 'k6-loadtest/1.0',
      },
    },
    purchase: {
      user_id: `user_${Math.floor(Math.random() * 100000)}`,
      order_id: `order_${Math.floor(Math.random() * 1000000)}`,
      amount: Math.floor(Math.random() * 50000) / 100,
      currency: 'USD',
      payment_method: ['card', 'paypal', 'stripe', 'razorpay'][Math.floor(Math.random() * 4)],
      items: [
        {
          product_id: `prod_${Math.floor(Math.random() * 1000)}`,
          quantity: Math.floor(Math.random() * 5) + 1,
          price: Math.floor(Math.random() * 10000) / 100,
        },
      ],
    },
    page_view: {
      user_id: `user_${Math.floor(Math.random() * 100000)}`,
      session_id: `session_${Math.floor(Math.random() * 10000)}`,
      page_url: `/page/${Math.floor(Math.random() * 100)}`,
      page_title: `Page ${Math.floor(Math.random() * 100)}`,
      duration_ms: Math.floor(Math.random() * 60000),
      metadata: {
        viewport_width: 1920,
        viewport_height: 1080,
      },
    },
  };

  return {
    eventType: eventType,
    payload: payloads[eventType],
  };
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

  // Check response
  const success = check(res, {
    'status is 202': (r) => r.status === 202,
    'has event_id': (r) => JSON.parse(r.body).event_id !== undefined,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });

  errorRate.add(!success);
}
