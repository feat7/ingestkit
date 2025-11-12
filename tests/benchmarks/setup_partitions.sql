-- Create default partitions for normalized tables
-- In production, you'd create per-tenant partitions, but for benchmarking we'll use defaults

CREATE TABLE IF NOT EXISTS events_user_signup_default PARTITION OF events_user_signup DEFAULT;
CREATE TABLE IF NOT EXISTS events_purchase_default PARTITION OF events_purchase DEFAULT;
CREATE TABLE IF NOT EXISTS events_page_view_default PARTITION OF events_page_view DEFAULT;
