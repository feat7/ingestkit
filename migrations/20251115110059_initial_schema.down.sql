-- reverse: create index "idx_user_signup_user_id" to table: "events_user_signup"
DROP INDEX "public"."idx_user_signup_user_id";
-- reverse: create index "idx_user_signup_timestamp" to table: "events_user_signup"
DROP INDEX "public"."idx_user_signup_timestamp";
-- reverse: create "events_user_signup" table
DROP TABLE "public"."events_user_signup";
-- reverse: create index "idx_search_performed_user_id" to table: "events_search_performed"
DROP INDEX "public"."idx_search_performed_user_id";
-- reverse: create index "idx_search_performed_timestamp" to table: "events_search_performed"
DROP INDEX "public"."idx_search_performed_timestamp";
-- reverse: create index "idx_search_performed_session_id" to table: "events_search_performed"
DROP INDEX "public"."idx_search_performed_session_id";
-- reverse: create index "idx_search_performed_query" to table: "events_search_performed"
DROP INDEX "public"."idx_search_performed_query";
-- reverse: create index "idx_search_performed_clicked_result_id" to table: "events_search_performed"
DROP INDEX "public"."idx_search_performed_clicked_result_id";
-- reverse: create "events_search_performed" table
DROP TABLE "public"."events_search_performed";
-- reverse: create index "idx_purchase_user_id" to table: "events_purchase"
DROP INDEX "public"."idx_purchase_user_id";
-- reverse: create index "idx_purchase_timestamp" to table: "events_purchase"
DROP INDEX "public"."idx_purchase_timestamp";
-- reverse: create index "idx_purchase_order_id" to table: "events_purchase"
DROP INDEX "public"."idx_purchase_order_id";
-- reverse: create "events_purchase" table
DROP TABLE "public"."events_purchase";
-- reverse: create index "idx_product_viewed_user_id" to table: "events_product_viewed"
DROP INDEX "public"."idx_product_viewed_user_id";
-- reverse: create index "idx_product_viewed_timestamp" to table: "events_product_viewed"
DROP INDEX "public"."idx_product_viewed_timestamp";
-- reverse: create index "idx_product_viewed_source" to table: "events_product_viewed"
DROP INDEX "public"."idx_product_viewed_source";
-- reverse: create index "idx_product_viewed_session_id" to table: "events_product_viewed"
DROP INDEX "public"."idx_product_viewed_session_id";
-- reverse: create index "idx_product_viewed_product_id" to table: "events_product_viewed"
DROP INDEX "public"."idx_product_viewed_product_id";
-- reverse: create index "idx_product_viewed_product_category" to table: "events_product_viewed"
DROP INDEX "public"."idx_product_viewed_product_category";
-- reverse: create "events_product_viewed" table
DROP TABLE "public"."events_product_viewed";
-- reverse: create index "idx_page_view_user_id" to table: "events_page_view"
DROP INDEX "public"."idx_page_view_user_id";
-- reverse: create index "idx_page_view_timestamp" to table: "events_page_view"
DROP INDEX "public"."idx_page_view_timestamp";
-- reverse: create index "idx_page_view_session_id" to table: "events_page_view"
DROP INDEX "public"."idx_page_view_session_id";
-- reverse: create "events_page_view" table
DROP TABLE "public"."events_page_view";
-- reverse: create index "idx_order_completed_user_id" to table: "events_order_completed"
DROP INDEX "public"."idx_order_completed_user_id";
-- reverse: create index "idx_order_completed_timestamp" to table: "events_order_completed"
DROP INDEX "public"."idx_order_completed_timestamp";
-- reverse: create index "idx_order_completed_session_id" to table: "events_order_completed"
DROP INDEX "public"."idx_order_completed_session_id";
-- reverse: create index "idx_order_completed_order_id" to table: "events_order_completed"
DROP INDEX "public"."idx_order_completed_order_id";
-- reverse: create index "idx_order_completed_cart_id" to table: "events_order_completed"
DROP INDEX "public"."idx_order_completed_cart_id";
-- reverse: create "events_order_completed" table
DROP TABLE "public"."events_order_completed";
-- reverse: create index "idx_newsletter_subscribed_user_id" to table: "events_newsletter_subscribed"
DROP INDEX "public"."idx_newsletter_subscribed_user_id";
-- reverse: create index "idx_newsletter_subscribed_timestamp" to table: "events_newsletter_subscribed"
DROP INDEX "public"."idx_newsletter_subscribed_timestamp";
-- reverse: create index "idx_newsletter_subscribed_subscription_type" to table: "events_newsletter_subscribed"
DROP INDEX "public"."idx_newsletter_subscribed_subscription_type";
-- reverse: create index "idx_newsletter_subscribed_source" to table: "events_newsletter_subscribed"
DROP INDEX "public"."idx_newsletter_subscribed_source";
-- reverse: create index "idx_newsletter_subscribed_session_id" to table: "events_newsletter_subscribed"
DROP INDEX "public"."idx_newsletter_subscribed_session_id";
-- reverse: create index "idx_newsletter_subscribed_email" to table: "events_newsletter_subscribed"
DROP INDEX "public"."idx_newsletter_subscribed_email";
-- reverse: create "events_newsletter_subscribed" table
DROP TABLE "public"."events_newsletter_subscribed";
-- reverse: create index "idx_comment_posted_user_id" to table: "events_comment_posted"
DROP INDEX "public"."idx_comment_posted_user_id";
-- reverse: create index "idx_comment_posted_timestamp" to table: "events_comment_posted"
DROP INDEX "public"."idx_comment_posted_timestamp";
-- reverse: create index "idx_comment_posted_session_id" to table: "events_comment_posted"
DROP INDEX "public"."idx_comment_posted_session_id";
-- reverse: create index "idx_comment_posted_parent_comment_id" to table: "events_comment_posted"
DROP INDEX "public"."idx_comment_posted_parent_comment_id";
-- reverse: create index "idx_comment_posted_comment_id" to table: "events_comment_posted"
DROP INDEX "public"."idx_comment_posted_comment_id";
-- reverse: create index "idx_comment_posted_article_id" to table: "events_comment_posted"
DROP INDEX "public"."idx_comment_posted_article_id";
-- reverse: create "events_comment_posted" table
DROP TABLE "public"."events_comment_posted";
-- reverse: create index "idx_checkout_started_user_id" to table: "events_checkout_started"
DROP INDEX "public"."idx_checkout_started_user_id";
-- reverse: create index "idx_checkout_started_timestamp" to table: "events_checkout_started"
DROP INDEX "public"."idx_checkout_started_timestamp";
-- reverse: create index "idx_checkout_started_session_id" to table: "events_checkout_started"
DROP INDEX "public"."idx_checkout_started_session_id";
-- reverse: create index "idx_checkout_started_cart_id" to table: "events_checkout_started"
DROP INDEX "public"."idx_checkout_started_cart_id";
-- reverse: create "events_checkout_started" table
DROP TABLE "public"."events_checkout_started";
-- reverse: create index "idx_article_viewed_user_id" to table: "events_article_viewed"
DROP INDEX "public"."idx_article_viewed_user_id";
-- reverse: create index "idx_article_viewed_timestamp" to table: "events_article_viewed"
DROP INDEX "public"."idx_article_viewed_timestamp";
-- reverse: create index "idx_article_viewed_source" to table: "events_article_viewed"
DROP INDEX "public"."idx_article_viewed_source";
-- reverse: create index "idx_article_viewed_session_id" to table: "events_article_viewed"
DROP INDEX "public"."idx_article_viewed_session_id";
-- reverse: create index "idx_article_viewed_category" to table: "events_article_viewed"
DROP INDEX "public"."idx_article_viewed_category";
-- reverse: create index "idx_article_viewed_author" to table: "events_article_viewed"
DROP INDEX "public"."idx_article_viewed_author";
-- reverse: create index "idx_article_viewed_article_id" to table: "events_article_viewed"
DROP INDEX "public"."idx_article_viewed_article_id";
-- reverse: create "events_article_viewed" table
DROP TABLE "public"."events_article_viewed";
-- reverse: create index "idx_article_shared_user_id" to table: "events_article_shared"
DROP INDEX "public"."idx_article_shared_user_id";
-- reverse: create index "idx_article_shared_timestamp" to table: "events_article_shared"
DROP INDEX "public"."idx_article_shared_timestamp";
-- reverse: create index "idx_article_shared_session_id" to table: "events_article_shared"
DROP INDEX "public"."idx_article_shared_session_id";
-- reverse: create index "idx_article_shared_platform" to table: "events_article_shared"
DROP INDEX "public"."idx_article_shared_platform";
-- reverse: create index "idx_article_shared_article_id" to table: "events_article_shared"
DROP INDEX "public"."idx_article_shared_article_id";
-- reverse: create "events_article_shared" table
DROP TABLE "public"."events_article_shared";
-- reverse: create index "idx_added_to_cart_user_id" to table: "events_added_to_cart"
DROP INDEX "public"."idx_added_to_cart_user_id";
-- reverse: create index "idx_added_to_cart_timestamp" to table: "events_added_to_cart"
DROP INDEX "public"."idx_added_to_cart_timestamp";
-- reverse: create index "idx_added_to_cart_session_id" to table: "events_added_to_cart"
DROP INDEX "public"."idx_added_to_cart_session_id";
-- reverse: create index "idx_added_to_cart_product_id" to table: "events_added_to_cart"
DROP INDEX "public"."idx_added_to_cart_product_id";
-- reverse: create "events_added_to_cart" table
DROP TABLE "public"."events_added_to_cart";
-- drop "dead_letter_queue" table
DROP TABLE IF EXISTS "ingestkit_meta"."dead_letter_queue";
-- drop "ingestkit_meta" schema
DROP SCHEMA IF EXISTS "ingestkit_meta";

