-- create "ingestkit_meta" schema
CREATE SCHEMA IF NOT EXISTS "ingestkit_meta";
-- create "dead_letter_queue" table
CREATE TABLE "ingestkit_meta"."dead_letter_queue" (
  "id" serial PRIMARY KEY,
  "event_type" character varying(255) NOT NULL,
  "tenant_id" character varying(255) NOT NULL,
  "event_data" jsonb NOT NULL,
  "error_message" text NOT NULL,
  "retry_count" integer DEFAULT 0,
  "created_at" timestamp DEFAULT now()
);

-- create "events_added_to_cart" table
CREATE TABLE "public"."events_added_to_cart" (
  "tenant_id" character varying(255) NOT NULL,
  "event_id" bigserial NOT NULL,
  "timestamp" timestamptz NOT NULL DEFAULT now(),
  "product_name" character varying(255) NOT NULL,
  "cart_total" numeric(20,2) NULL,
  "metadata" jsonb NULL,
  "quantity" bigint NOT NULL,
  "price" numeric(20,2) NOT NULL,
  "currency" character varying(255) NOT NULL DEFAULT 'USD',
  "user_id" character varying(255) NOT NULL,
  "session_id" character varying(255) NOT NULL,
  "product_id" character varying(255) NOT NULL,
  PRIMARY KEY ("tenant_id", "event_id")
) PARTITION BY LIST ("tenant_id");
-- create index "idx_added_to_cart_product_id" to table: "events_added_to_cart"
CREATE INDEX "idx_added_to_cart_product_id" ON "public"."events_added_to_cart" ("product_id");
-- create index "idx_added_to_cart_session_id" to table: "events_added_to_cart"
CREATE INDEX "idx_added_to_cart_session_id" ON "public"."events_added_to_cart" ("session_id");
-- create index "idx_added_to_cart_timestamp" to table: "events_added_to_cart"
CREATE INDEX "idx_added_to_cart_timestamp" ON "public"."events_added_to_cart" ("timestamp");
-- create index "idx_added_to_cart_user_id" to table: "events_added_to_cart"
CREATE INDEX "idx_added_to_cart_user_id" ON "public"."events_added_to_cart" ("user_id");
-- create "events_article_shared" table
CREATE TABLE "public"."events_article_shared" (
  "tenant_id" character varying(255) NOT NULL,
  "event_id" bigserial NOT NULL,
  "timestamp" timestamptz NOT NULL DEFAULT now(),
  "platform" character varying(255) NOT NULL,
  "metadata" jsonb NULL,
  "user_id" character varying(255) NULL,
  "session_id" character varying(255) NOT NULL,
  "article_id" character varying(255) NOT NULL,
  "article_title" character varying(255) NOT NULL,
  PRIMARY KEY ("tenant_id", "event_id")
) PARTITION BY LIST ("tenant_id");
-- create index "idx_article_shared_article_id" to table: "events_article_shared"
CREATE INDEX "idx_article_shared_article_id" ON "public"."events_article_shared" ("article_id");
-- create index "idx_article_shared_platform" to table: "events_article_shared"
CREATE INDEX "idx_article_shared_platform" ON "public"."events_article_shared" ("platform");
-- create index "idx_article_shared_session_id" to table: "events_article_shared"
CREATE INDEX "idx_article_shared_session_id" ON "public"."events_article_shared" ("session_id");
-- create index "idx_article_shared_timestamp" to table: "events_article_shared"
CREATE INDEX "idx_article_shared_timestamp" ON "public"."events_article_shared" ("timestamp");
-- create index "idx_article_shared_user_id" to table: "events_article_shared"
CREATE INDEX "idx_article_shared_user_id" ON "public"."events_article_shared" ("user_id");
-- create "events_article_viewed" table
CREATE TABLE "public"."events_article_viewed" (
  "tenant_id" character varying(255) NOT NULL,
  "event_id" bigserial NOT NULL,
  "timestamp" timestamptz NOT NULL DEFAULT now(),
  "category" character varying(255) NOT NULL,
  "read_time_seconds" bigint NULL,
  "session_id" character varying(255) NOT NULL,
  "article_id" character varying(255) NOT NULL,
  "article_title" character varying(255) NOT NULL,
  "author" character varying(255) NOT NULL,
  "tags" jsonb NULL,
  "source" character varying(255) NULL,
  "referrer" character varying(255) NULL,
  "metadata" jsonb NULL,
  "user_id" character varying(255) NULL,
  PRIMARY KEY ("tenant_id", "event_id")
) PARTITION BY LIST ("tenant_id");
-- create index "idx_article_viewed_article_id" to table: "events_article_viewed"
CREATE INDEX "idx_article_viewed_article_id" ON "public"."events_article_viewed" ("article_id");
-- create index "idx_article_viewed_author" to table: "events_article_viewed"
CREATE INDEX "idx_article_viewed_author" ON "public"."events_article_viewed" ("author");
-- create index "idx_article_viewed_category" to table: "events_article_viewed"
CREATE INDEX "idx_article_viewed_category" ON "public"."events_article_viewed" ("category");
-- create index "idx_article_viewed_session_id" to table: "events_article_viewed"
CREATE INDEX "idx_article_viewed_session_id" ON "public"."events_article_viewed" ("session_id");
-- create index "idx_article_viewed_source" to table: "events_article_viewed"
CREATE INDEX "idx_article_viewed_source" ON "public"."events_article_viewed" ("source");
-- create index "idx_article_viewed_timestamp" to table: "events_article_viewed"
CREATE INDEX "idx_article_viewed_timestamp" ON "public"."events_article_viewed" ("timestamp");
-- create index "idx_article_viewed_user_id" to table: "events_article_viewed"
CREATE INDEX "idx_article_viewed_user_id" ON "public"."events_article_viewed" ("user_id");
-- create "events_checkout_started" table
CREATE TABLE "public"."events_checkout_started" (
  "tenant_id" character varying(255) NOT NULL,
  "event_id" bigserial NOT NULL,
  "timestamp" timestamptz NOT NULL DEFAULT now(),
  "currency" character varying(255) NOT NULL DEFAULT 'USD',
  "items" jsonb NOT NULL,
  "metadata" jsonb NULL,
  "user_id" character varying(255) NOT NULL,
  "session_id" character varying(255) NOT NULL,
  "cart_id" character varying(255) NOT NULL,
  "num_items" bigint NOT NULL,
  "cart_total" numeric(20,2) NOT NULL,
  PRIMARY KEY ("tenant_id", "event_id")
) PARTITION BY LIST ("tenant_id");
-- create index "idx_checkout_started_cart_id" to table: "events_checkout_started"
CREATE INDEX "idx_checkout_started_cart_id" ON "public"."events_checkout_started" ("cart_id");
-- create index "idx_checkout_started_session_id" to table: "events_checkout_started"
CREATE INDEX "idx_checkout_started_session_id" ON "public"."events_checkout_started" ("session_id");
-- create index "idx_checkout_started_timestamp" to table: "events_checkout_started"
CREATE INDEX "idx_checkout_started_timestamp" ON "public"."events_checkout_started" ("timestamp");
-- create index "idx_checkout_started_user_id" to table: "events_checkout_started"
CREATE INDEX "idx_checkout_started_user_id" ON "public"."events_checkout_started" ("user_id");
-- create "events_comment_posted" table
CREATE TABLE "public"."events_comment_posted" (
  "tenant_id" character varying(255) NOT NULL,
  "event_id" bigserial NOT NULL,
  "timestamp" timestamptz NOT NULL DEFAULT now(),
  "user_id" character varying(255) NOT NULL,
  "session_id" character varying(255) NOT NULL,
  "article_id" character varying(255) NOT NULL,
  "comment_id" character varying(255) NOT NULL,
  "parent_comment_id" character varying(255) NULL,
  "comment_length" bigint NOT NULL,
  "metadata" jsonb NULL,
  PRIMARY KEY ("tenant_id", "event_id")
) PARTITION BY LIST ("tenant_id");
-- create index "idx_comment_posted_article_id" to table: "events_comment_posted"
CREATE INDEX "idx_comment_posted_article_id" ON "public"."events_comment_posted" ("article_id");
-- create index "idx_comment_posted_comment_id" to table: "events_comment_posted"
CREATE INDEX "idx_comment_posted_comment_id" ON "public"."events_comment_posted" ("comment_id");
-- create index "idx_comment_posted_parent_comment_id" to table: "events_comment_posted"
CREATE INDEX "idx_comment_posted_parent_comment_id" ON "public"."events_comment_posted" ("parent_comment_id");
-- create index "idx_comment_posted_session_id" to table: "events_comment_posted"
CREATE INDEX "idx_comment_posted_session_id" ON "public"."events_comment_posted" ("session_id");
-- create index "idx_comment_posted_timestamp" to table: "events_comment_posted"
CREATE INDEX "idx_comment_posted_timestamp" ON "public"."events_comment_posted" ("timestamp");
-- create index "idx_comment_posted_user_id" to table: "events_comment_posted"
CREATE INDEX "idx_comment_posted_user_id" ON "public"."events_comment_posted" ("user_id");
-- create "events_newsletter_subscribed" table
CREATE TABLE "public"."events_newsletter_subscribed" (
  "tenant_id" character varying(255) NOT NULL,
  "event_id" bigserial NOT NULL,
  "timestamp" timestamptz NOT NULL DEFAULT now(),
  "metadata" jsonb NULL,
  "user_id" character varying(255) NULL,
  "session_id" character varying(255) NOT NULL,
  "email" character varying(255) NOT NULL,
  "subscription_type" character varying(255) NOT NULL,
  "source" character varying(255) NULL,
  PRIMARY KEY ("tenant_id", "event_id")
) PARTITION BY LIST ("tenant_id");
-- create index "idx_newsletter_subscribed_email" to table: "events_newsletter_subscribed"
CREATE INDEX "idx_newsletter_subscribed_email" ON "public"."events_newsletter_subscribed" ("email");
-- create index "idx_newsletter_subscribed_session_id" to table: "events_newsletter_subscribed"
CREATE INDEX "idx_newsletter_subscribed_session_id" ON "public"."events_newsletter_subscribed" ("session_id");
-- create index "idx_newsletter_subscribed_source" to table: "events_newsletter_subscribed"
CREATE INDEX "idx_newsletter_subscribed_source" ON "public"."events_newsletter_subscribed" ("source");
-- create index "idx_newsletter_subscribed_subscription_type" to table: "events_newsletter_subscribed"
CREATE INDEX "idx_newsletter_subscribed_subscription_type" ON "public"."events_newsletter_subscribed" ("subscription_type");
-- create index "idx_newsletter_subscribed_timestamp" to table: "events_newsletter_subscribed"
CREATE INDEX "idx_newsletter_subscribed_timestamp" ON "public"."events_newsletter_subscribed" ("timestamp");
-- create index "idx_newsletter_subscribed_user_id" to table: "events_newsletter_subscribed"
CREATE INDEX "idx_newsletter_subscribed_user_id" ON "public"."events_newsletter_subscribed" ("user_id");
-- create "events_order_completed" table
CREATE TABLE "public"."events_order_completed" (
  "tenant_id" character varying(255) NOT NULL,
  "event_id" bigserial NOT NULL,
  "timestamp" timestamptz NOT NULL DEFAULT now(),
  "cart_id" character varying(255) NULL,
  "currency" character varying(255) NOT NULL DEFAULT 'USD',
  "payment_method" character varying(255) NOT NULL,
  "num_items" bigint NOT NULL,
  "shipping_address" jsonb NOT NULL,
  "items" jsonb NOT NULL,
  "user_id" character varying(255) NOT NULL,
  "total_amount" numeric(20,2) NOT NULL,
  "discount_code" character varying(255) NULL,
  "discount_amount" numeric(20,2) NULL,
  "metadata" jsonb NULL,
  "session_id" character varying(255) NOT NULL,
  "order_id" character varying(255) NOT NULL,
  PRIMARY KEY ("tenant_id", "event_id")
) PARTITION BY LIST ("tenant_id");
-- create index "idx_order_completed_cart_id" to table: "events_order_completed"
CREATE INDEX "idx_order_completed_cart_id" ON "public"."events_order_completed" ("cart_id");
-- create index "idx_order_completed_order_id" to table: "events_order_completed"
CREATE INDEX "idx_order_completed_order_id" ON "public"."events_order_completed" ("order_id");
-- create index "idx_order_completed_session_id" to table: "events_order_completed"
CREATE INDEX "idx_order_completed_session_id" ON "public"."events_order_completed" ("session_id");
-- create index "idx_order_completed_timestamp" to table: "events_order_completed"
CREATE INDEX "idx_order_completed_timestamp" ON "public"."events_order_completed" ("timestamp");
-- create index "idx_order_completed_user_id" to table: "events_order_completed"
CREATE INDEX "idx_order_completed_user_id" ON "public"."events_order_completed" ("user_id");
-- create "events_page_view" table
CREATE TABLE "public"."events_page_view" (
  "tenant_id" character varying(255) NOT NULL,
  "event_id" bigserial NOT NULL,
  "timestamp" timestamptz NOT NULL DEFAULT now(),
  "metadata" jsonb NULL,
  "user_id" character varying(255) NULL,
  "session_id" character varying(255) NOT NULL,
  "page_url" character varying(255) NOT NULL,
  "page_title" character varying(255) NULL,
  "referrer" character varying(255) NULL,
  "duration_ms" bigint NULL,
  PRIMARY KEY ("tenant_id", "event_id")
) PARTITION BY LIST ("tenant_id");
-- create index "idx_page_view_session_id" to table: "events_page_view"
CREATE INDEX "idx_page_view_session_id" ON "public"."events_page_view" ("session_id");
-- create index "idx_page_view_timestamp" to table: "events_page_view"
CREATE INDEX "idx_page_view_timestamp" ON "public"."events_page_view" ("timestamp");
-- create index "idx_page_view_user_id" to table: "events_page_view"
CREATE INDEX "idx_page_view_user_id" ON "public"."events_page_view" ("user_id");
-- create "events_product_viewed" table
CREATE TABLE "public"."events_product_viewed" (
  "tenant_id" character varying(255) NOT NULL,
  "event_id" bigserial NOT NULL,
  "timestamp" timestamptz NOT NULL DEFAULT now(),
  "user_id" character varying(255) NOT NULL,
  "session_id" character varying(255) NOT NULL,
  "product_name" character varying(255) NOT NULL,
  "source" character varying(255) NULL,
  "product_id" character varying(255) NOT NULL,
  "product_category" character varying(255) NOT NULL,
  "price" numeric(20,2) NOT NULL,
  "currency" character varying(255) NOT NULL DEFAULT 'USD',
  "metadata" jsonb NULL,
  PRIMARY KEY ("tenant_id", "event_id")
) PARTITION BY LIST ("tenant_id");
-- create index "idx_product_viewed_product_category" to table: "events_product_viewed"
CREATE INDEX "idx_product_viewed_product_category" ON "public"."events_product_viewed" ("product_category");
-- create index "idx_product_viewed_product_id" to table: "events_product_viewed"
CREATE INDEX "idx_product_viewed_product_id" ON "public"."events_product_viewed" ("product_id");
-- create index "idx_product_viewed_session_id" to table: "events_product_viewed"
CREATE INDEX "idx_product_viewed_session_id" ON "public"."events_product_viewed" ("session_id");
-- create index "idx_product_viewed_source" to table: "events_product_viewed"
CREATE INDEX "idx_product_viewed_source" ON "public"."events_product_viewed" ("source");
-- create index "idx_product_viewed_timestamp" to table: "events_product_viewed"
CREATE INDEX "idx_product_viewed_timestamp" ON "public"."events_product_viewed" ("timestamp");
-- create index "idx_product_viewed_user_id" to table: "events_product_viewed"
CREATE INDEX "idx_product_viewed_user_id" ON "public"."events_product_viewed" ("user_id");
-- create "events_purchase" table
CREATE TABLE "public"."events_purchase" (
  "tenant_id" character varying(255) NOT NULL,
  "event_id" bigserial NOT NULL,
  "timestamp" timestamptz NOT NULL DEFAULT now(),
  "payment_method" character varying(255) NULL,
  "items" jsonb NULL,
  "user_id" character varying(255) NOT NULL,
  "order_id" character varying(255) NOT NULL,
  "amount" numeric(20,2) NOT NULL,
  "currency" character varying(255) NULL DEFAULT 'USD',
  PRIMARY KEY ("tenant_id", "event_id")
) PARTITION BY LIST ("tenant_id");
-- create index "idx_purchase_order_id" to table: "events_purchase"
CREATE INDEX "idx_purchase_order_id" ON "public"."events_purchase" ("order_id");
-- create index "idx_purchase_timestamp" to table: "events_purchase"
CREATE INDEX "idx_purchase_timestamp" ON "public"."events_purchase" ("timestamp");
-- create index "idx_purchase_user_id" to table: "events_purchase"
CREATE INDEX "idx_purchase_user_id" ON "public"."events_purchase" ("user_id");
-- create "events_search_performed" table
CREATE TABLE "public"."events_search_performed" (
  "tenant_id" character varying(255) NOT NULL,
  "event_id" bigserial NOT NULL,
  "timestamp" timestamptz NOT NULL DEFAULT now(),
  "metadata" jsonb NULL,
  "user_id" character varying(255) NULL,
  "session_id" character varying(255) NOT NULL,
  "query" character varying(255) NOT NULL,
  "results_count" bigint NOT NULL,
  "clicked_result_id" character varying(255) NULL,
  PRIMARY KEY ("tenant_id", "event_id")
) PARTITION BY LIST ("tenant_id");
-- create index "idx_search_performed_clicked_result_id" to table: "events_search_performed"
CREATE INDEX "idx_search_performed_clicked_result_id" ON "public"."events_search_performed" ("clicked_result_id");
-- create index "idx_search_performed_query" to table: "events_search_performed"
CREATE INDEX "idx_search_performed_query" ON "public"."events_search_performed" ("query");
-- create index "idx_search_performed_session_id" to table: "events_search_performed"
CREATE INDEX "idx_search_performed_session_id" ON "public"."events_search_performed" ("session_id");
-- create index "idx_search_performed_timestamp" to table: "events_search_performed"
CREATE INDEX "idx_search_performed_timestamp" ON "public"."events_search_performed" ("timestamp");
-- create index "idx_search_performed_user_id" to table: "events_search_performed"
CREATE INDEX "idx_search_performed_user_id" ON "public"."events_search_performed" ("user_id");
-- create "events_user_signup" table
CREATE TABLE "public"."events_user_signup" (
  "tenant_id" character varying(255) NOT NULL,
  "event_id" bigserial NOT NULL,
  "timestamp" timestamptz NOT NULL DEFAULT now(),
  "email" character varying(255) NOT NULL,
  "signup_source" character varying(255) NULL,
  "signup_device" character varying(255) NULL,
  "referrer" character varying(255) NULL,
  "utm_campaign" character varying(255) NULL,
  "metadata" jsonb NULL,
  "user_id" character varying(255) NOT NULL,
  PRIMARY KEY ("tenant_id", "event_id")
) PARTITION BY LIST ("tenant_id");
-- create index "idx_user_signup_timestamp" to table: "events_user_signup"
CREATE INDEX "idx_user_signup_timestamp" ON "public"."events_user_signup" ("timestamp");
-- create index "idx_user_signup_user_id" to table: "events_user_signup"
CREATE INDEX "idx_user_signup_user_id" ON "public"."events_user_signup" ("user_id");
