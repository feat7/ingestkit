/**
 * IngestKit TypeScript SDK - Event Models
 * Auto-generated from schema version 1.0
 * DO NOT EDIT MANUALLY
 */


/**
 * Fired when a user views a blog article
 */
export interface ArticleViewed {
  tenant_id: string;
  event_id?: number;
  timestamp?: Date;
  session_id: string;
  category: string;
  tags?: Record<string, any>;
  source?: string;
  referrer?: string;
  article_id: string;
  article_title: string;
  author: string;
  read_time_seconds?: number;
  metadata?: Record<string, any>;
  user_id?: string;
}


/**
 * Fired when a user shares an article
 */
export interface ArticleShared {
  tenant_id: string;
  event_id?: number;
  timestamp?: Date;
  user_id?: string;
  session_id: string;
  article_id: string;
  article_title: string;
  platform: string;
  metadata?: Record<string, any>;
}


/**
 * Fired when a user posts a comment
 */
export interface CommentPosted {
  tenant_id: string;
  event_id?: number;
  timestamp?: Date;
  metadata?: Record<string, any>;
  user_id: string;
  session_id: string;
  article_id: string;
  comment_id: string;
  parent_comment_id?: string;
  comment_length: number;
}


/**
 * Fired when a user subscribes to newsletter
 */
export interface NewsletterSubscribed {
  tenant_id: string;
  event_id?: number;
  timestamp?: Date;
  source?: string;
  metadata?: Record<string, any>;
  user_id?: string;
  session_id: string;
  email: string;
  subscription_type: string;
}


/**
 * Fired when a user views a product page
 */
export interface ProductViewed {
  tenant_id: string;
  event_id?: number;
  timestamp?: Date;
  session_id: string;
  product_name: string;
  currency: string;
  user_id: string;
  product_id: string;
  product_category: string;
  price: number;
  source?: string;
  metadata?: Record<string, any>;
}


/**
 * Fired when a user searches for content
 */
export interface SearchPerformed {
  tenant_id: string;
  event_id?: number;
  timestamp?: Date;
  results_count: number;
  clicked_result_id?: string;
  metadata?: Record<string, any>;
  user_id?: string;
  session_id: string;
  query: string;
}


/**
 * Fired when a user adds an item to cart
 */
export interface AddedToCart {
  tenant_id: string;
  event_id?: number;
  timestamp?: Date;
  price: number;
  session_id: string;
  currency: string;
  cart_total?: number;
  metadata?: Record<string, any>;
  user_id: string;
  product_id: string;
  product_name: string;
  quantity: number;
}


/**
 * Fired when a user begins checkout process
 */
export interface CheckoutStarted {
  tenant_id: string;
  event_id?: number;
  timestamp?: Date;
  items: Record<string, any>;
  metadata?: Record<string, any>;
  user_id: string;
  session_id: string;
  cart_id: string;
  num_items: number;
  cart_total: number;
  currency: string;
}


/**
 * Fired when an order is successfully placed
 */
export interface OrderCompleted {
  tenant_id: string;
  event_id?: number;
  timestamp?: Date;
  cart_id?: string;
  total_amount: number;
  num_items: number;
  items: Record<string, any>;
  user_id: string;
  session_id: string;
  currency: string;
  payment_method: string;
  shipping_address: Record<string, any>;
  discount_code?: string;
  discount_amount?: number;
  metadata?: Record<string, any>;
  order_id: string;
}


/**
 * Fired when a new user signs up
 */
export interface UserSignup {
  tenant_id: string;
  event_id?: number;
  timestamp?: Date;
  email: string;  // User email address
  signup_source?: string;  // Where the signup originated
  utm_campaign?: string;  // Marketing campaign identifier
  metadata?: Record<string, any>;  // Additional flexible metadata
  user_id: string;  // Unique user identifier
}


/**
 * Fired when a user makes a purchase
 */
export interface Purchase {
  tenant_id: string;
  event_id?: number;
  timestamp?: Date;
  currency?: string;  // Currency code
  payment_method?: string;  // Payment method used
  items?: Record<string, any>;  // Array of purchased items
  user_id: string;  // User who made the purchase
  order_id: string;  // Unique order identifier
  amount: number;  // Purchase amount
}


/**
 * Fired when a user views a page
 */
export interface PageView {
  tenant_id: string;
  event_id?: number;
  timestamp?: Date;
  page_url: string;  // Full page URL
  page_title?: string;  // Page title
  referrer?: string;  // Referrer URL
  duration_ms?: number;  // Time spent on page (milliseconds)
  metadata?: Record<string, any>;  // Additional page metadata
  user_id?: string;  // User ID (if logged in)
  session_id: string;  // Session identifier
}


