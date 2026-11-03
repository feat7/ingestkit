CREATE TABLE IF NOT EXISTS orders (
 order_id TEXT PRIMARY KEY,
 amount_cents INTEGER NOT NULL CHECK(amount_cents >= 0),
 event_id TEXT NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS outbox (
 event_id TEXT PRIMARY KEY,
 order_id TEXT NOT NULL UNIQUE REFERENCES orders(order_id),
 body TEXT NOT NULL,
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','acknowledged','rejected')),
 attempts INTEGER NOT NULL DEFAULT 0,
 last_error TEXT,
 acknowledged_at TEXT
);
