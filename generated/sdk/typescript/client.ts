/**
 * IngestKit TypeScript SDK - Client
 * Auto-generated from schema version 1.0
 * DO NOT EDIT MANUALLY
 */

import * as Models from './models';

// Configuration constants
const DEFAULT_API_URL = 'http://localhost:8080';
const DEFAULT_TENANT_ID = 'default';
const DEFAULT_TIMEOUT = 30000; // milliseconds
const DEFAULT_MAX_RETRIES = 3;
const DEFAULT_RETRY_BACKOFF_BASE = 1000; // milliseconds
const DEFAULT_MAX_BATCH_SIZE = 1000;

// HTTP status code ranges
const HTTP_CLIENT_ERROR_START = 400;
const HTTP_CLIENT_ERROR_END = 499;
const HTTP_SERVER_ERROR_START = 500;
const HTTP_SERVER_ERROR_END = 599;

/**
 * Base error class for IngestKit errors
 */
export class IngestKitError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'IngestKitError';
  }
}

/**
 * 4xx client errors (non-retriable)
 */
export class ClientError extends IngestKitError {
  constructor(
    public statusCode: number,
    message: string,
    public requestId?: string
  ) {
    super(`HTTP ${statusCode}: ${message}${requestId ? ` [request_id: ${requestId}]` : ''}`);
    this.name = 'ClientError';
  }
}

/**
 * 5xx server errors (retriable)
 */
export class ServerError extends IngestKitError {
  constructor(
    public statusCode: number,
    message: string,
    public requestId?: string
  ) {
    super(`HTTP ${statusCode}: ${message}${requestId ? ` [request_id: ${requestId}]` : ''}`);
    this.name = 'ServerError';
  }
}

/**
 * Network errors (retriable)
 */
export class NetworkError extends IngestKitError {
  constructor(message: string) {
    super(message);
    this.name = 'NetworkError';
  }
}

export interface IngestKitClientConfig {
  apiUrl?: string;
  apiKey?: string;
  tenantId?: string;
  timeout?: number;
  maxRetries?: number;
  retryBackoffBase?: number;
  debug?: boolean;
  logger?: Logger;
  onSuccess?: (eventData: any, result: APIResponse) => void;
  onFailure?: (eventData: any, error: Error) => void;
}

export interface APIResponse {
  success: boolean;
  message?: string;
  [key: string]: any;
}

export interface Logger {
  debug(message: string): void;
  info(message: string): void;
  warn(message: string): void;
  error(message: string): void;
}

/**
 * Default console logger
 */
class ConsoleLogger implements Logger {
  constructor(private enabled: boolean = false) {}

  debug(message: string): void {
    if (this.enabled) console.debug(`[IngestKit DEBUG] ${message}`);
  }

  info(message: string): void {
    if (this.enabled) console.info(`[IngestKit INFO] ${message}`);
  }

  warn(message: string): void {
    console.warn(`[IngestKit WARN] ${message}`);
  }

  error(message: string): void {
    console.error(`[IngestKit ERROR] ${message}`);
  }
}

/**
 * Validate API key format and warn if it appears to be hardcoded
 */
function validateApiKey(apiKey: string, logger: Logger): void {
  if (!apiKey) {
    throw new Error('API key cannot be empty');
  }

  // Warn if API key doesn't look like an environment variable placeholder
  if (!apiKey.startsWith('${') && apiKey.length < 10) {
    logger.warn(
      'API key appears to be hardcoded and short. ' +
      'Consider using environment variables: ${INGESTKIT_API_KEY}'
    );
  }
}

/**
 * Redact API key for logging (show first 4 and last 4 characters)
 */
function redactApiKey(apiKey: string): string {
  if (apiKey.length <= 8) {
    return '****';
  }
  return `${apiKey.substring(0, 4)}...${apiKey.substring(apiKey.length - 4)}`;
}

/**
 * Generate UUID v4 for request IDs
 */
function generateRequestId(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  // Fallback for older environments
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = Math.random() * 16 | 0;
    const v = c === 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

/**
 * Load configuration from ingestkit.config.json (Node.js only)
 * Returns empty object in browser environments
 */
function loadConfig(): Partial<IngestKitClientConfig> {
  // Check if we're in a Node.js environment
  if (typeof process === 'undefined' || typeof require === 'undefined') {
    return {}; // Browser environment - skip file loading
  }

  try {
    // Dynamic import for Node.js only
    const fs = require('fs');
    const path = require('path');

    const configPath = path.join(process.cwd(), 'ingestkit.config.json');
    if (fs.existsSync(configPath)) {
      const configData = fs.readFileSync(configPath, 'utf-8');
      const config = JSON.parse(configData);

      // Substitute environment variables in config values
      const substituteEnv = (value: any): any => {
        if (typeof value === 'string' && typeof process !== 'undefined') {
          return value.replace(/\$\{([^}]+)\}/g, (_, varName) =>
            process.env[varName] || ''
          );
        }
        return value;
      };

      return {
        apiUrl: substituteEnv(config.apiUrl),
        apiKey: substituteEnv(config.apiKey),
        tenantId: substituteEnv(config.tenantId),
      };
    }
  } catch (error) {
    // Config file doesn't exist or is invalid, continue with defaults
  }
  return {};
}

/**
 * Sleep for specified milliseconds
 */
function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * IngestKit API client with Promise-based delivery guarantees.
 *
 * Features:
 * - GUARANTEED DELIVERY: Promise-based confirmation (like Kafka producer)
 * - Success/failure callbacks for async notifications
 * - Automatic retry with exponential backoff for server errors
 * - Request ID tracking for debugging
 * - Browser and Node.js compatible
 * - Configurable timeouts and retry behavior
 *
 * Usage:
 * ```typescript
 * // Pattern 1: Fire and forget with callbacks
 * const client = new IngestKitClient({
 *   onSuccess: (eventData, result) => console.log('Delivered:', result),
 *   onFailure: (eventData, error) => console.error('Failed:', error)
 * });
 *
 * // Send events - returns Promise
 * await client.sendUserSignup(event);  // Awaits delivery confirmation
 *
 * // Pattern 2: Fire and forget (no await)
 * client.sendUserSignup(event);  // Don't await - callbacks will be called
 *
 * // Pattern 3: Explicit wait with timeout
 * try {
 *   const result = await Promise.race([
 *     client.sendUserSignup(event),
 *     new Promise((_, reject) => setTimeout(() => reject(new Error('Timeout')), 5000))
 *   ]);
 *   console.log('Guaranteed delivered:', result);
 * } catch (error) {
 *   console.error('Failed or timeout:', error);
 * }
 * ```
 */
export class IngestKitClient {
  private apiUrl: string;
  private apiKey: string;
  private tenantId: string;
  private timeout: number;
  private maxRetries: number;
  private retryBackoffBase: number;
  private debug: boolean;
  private logger: Logger;
  private onSuccess?: (eventData: any, result: APIResponse) => void;
  private onFailure?: (eventData: any, error: Error) => void;

  constructor(config?: IngestKitClientConfig) {
    const loadedConfig = loadConfig();
    const mergedConfig = { ...loadedConfig, ...config };

    this.apiUrl = (
      mergedConfig.apiUrl ||
      (typeof process !== 'undefined' ? process.env?.INGESTKIT_API_URL : undefined) ||
      DEFAULT_API_URL
    ).replace(/\/$/, '');

    this.apiKey =
      mergedConfig.apiKey ||
      (typeof process !== 'undefined' ? process.env?.INGESTKIT_API_KEY : undefined) ||
      '';

    this.tenantId =
      mergedConfig.tenantId ||
      (typeof process !== 'undefined' ? process.env?.INGESTKIT_TENANT_ID : undefined) ||
      DEFAULT_TENANT_ID;

    if (!this.apiKey) {
      throw new Error(
        'API key is required. Provide via constructor, ingestkit.config.json, ' +
        'or INGESTKIT_API_KEY environment variable.'
      );
    }

    this.timeout = mergedConfig.timeout || DEFAULT_TIMEOUT;
    this.maxRetries = mergedConfig.maxRetries || DEFAULT_MAX_RETRIES;
    this.retryBackoffBase = mergedConfig.retryBackoffBase || DEFAULT_RETRY_BACKOFF_BASE;
    this.debug = mergedConfig.debug || false;
    this.logger = mergedConfig.logger || new ConsoleLogger(this.debug);
    this.onSuccess = mergedConfig.onSuccess;
    this.onFailure = mergedConfig.onFailure;

    // Validate API key
    validateApiKey(this.apiKey, this.logger);

    if (this.debug) {
      this.logger.debug(
        `IngestKit client initialized: api_url=${this.apiUrl}, ` +
        `tenant_id=${this.tenantId}, api_key=${redactApiKey(this.apiKey)}`
      );
    }
  }


  /**
   * Send a article_viewed event.
   * Fired when a user views a blog article
   *
   * @param event - ArticleViewed event to send
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   */
  async sendArticleViewed(event: Models.ArticleViewed, options?: { sync?: boolean }): Promise<APIResponse> {
    return this.sendEvent('article_viewed', event, options?.sync);
  }

  /**
   * Send a batch of article_viewed events.
   *
   * @param events - Array of ArticleViewed events to send (max 1000)
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   * @throws {Error} If batch size exceeds maximum
   */
  async sendArticleViewedBatch(events: Models.ArticleViewed[], options?: { sync?: boolean }): Promise<APIResponse> {
    if (events.length > DEFAULT_MAX_BATCH_SIZE) {
      throw new Error(
        `Batch size ${events.length} exceeds maximum ${DEFAULT_MAX_BATCH_SIZE}. ` +
        `Please split into smaller batches.`
      );
    }
    return this.sendBatch('article_viewed', events, options?.sync);
  }

  /**
   * Send a article_shared event.
   * Fired when a user shares an article
   *
   * @param event - ArticleShared event to send
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   */
  async sendArticleShared(event: Models.ArticleShared, options?: { sync?: boolean }): Promise<APIResponse> {
    return this.sendEvent('article_shared', event, options?.sync);
  }

  /**
   * Send a batch of article_shared events.
   *
   * @param events - Array of ArticleShared events to send (max 1000)
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   * @throws {Error} If batch size exceeds maximum
   */
  async sendArticleSharedBatch(events: Models.ArticleShared[], options?: { sync?: boolean }): Promise<APIResponse> {
    if (events.length > DEFAULT_MAX_BATCH_SIZE) {
      throw new Error(
        `Batch size ${events.length} exceeds maximum ${DEFAULT_MAX_BATCH_SIZE}. ` +
        `Please split into smaller batches.`
      );
    }
    return this.sendBatch('article_shared', events, options?.sync);
  }

  /**
   * Send a comment_posted event.
   * Fired when a user posts a comment
   *
   * @param event - CommentPosted event to send
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   */
  async sendCommentPosted(event: Models.CommentPosted, options?: { sync?: boolean }): Promise<APIResponse> {
    return this.sendEvent('comment_posted', event, options?.sync);
  }

  /**
   * Send a batch of comment_posted events.
   *
   * @param events - Array of CommentPosted events to send (max 1000)
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   * @throws {Error} If batch size exceeds maximum
   */
  async sendCommentPostedBatch(events: Models.CommentPosted[], options?: { sync?: boolean }): Promise<APIResponse> {
    if (events.length > DEFAULT_MAX_BATCH_SIZE) {
      throw new Error(
        `Batch size ${events.length} exceeds maximum ${DEFAULT_MAX_BATCH_SIZE}. ` +
        `Please split into smaller batches.`
      );
    }
    return this.sendBatch('comment_posted', events, options?.sync);
  }

  /**
   * Send a newsletter_subscribed event.
   * Fired when a user subscribes to newsletter
   *
   * @param event - NewsletterSubscribed event to send
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   */
  async sendNewsletterSubscribed(event: Models.NewsletterSubscribed, options?: { sync?: boolean }): Promise<APIResponse> {
    return this.sendEvent('newsletter_subscribed', event, options?.sync);
  }

  /**
   * Send a batch of newsletter_subscribed events.
   *
   * @param events - Array of NewsletterSubscribed events to send (max 1000)
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   * @throws {Error} If batch size exceeds maximum
   */
  async sendNewsletterSubscribedBatch(events: Models.NewsletterSubscribed[], options?: { sync?: boolean }): Promise<APIResponse> {
    if (events.length > DEFAULT_MAX_BATCH_SIZE) {
      throw new Error(
        `Batch size ${events.length} exceeds maximum ${DEFAULT_MAX_BATCH_SIZE}. ` +
        `Please split into smaller batches.`
      );
    }
    return this.sendBatch('newsletter_subscribed', events, options?.sync);
  }

  /**
   * Send a product_viewed event.
   * Fired when a user views a product page
   *
   * @param event - ProductViewed event to send
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   */
  async sendProductViewed(event: Models.ProductViewed, options?: { sync?: boolean }): Promise<APIResponse> {
    return this.sendEvent('product_viewed', event, options?.sync);
  }

  /**
   * Send a batch of product_viewed events.
   *
   * @param events - Array of ProductViewed events to send (max 1000)
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   * @throws {Error} If batch size exceeds maximum
   */
  async sendProductViewedBatch(events: Models.ProductViewed[], options?: { sync?: boolean }): Promise<APIResponse> {
    if (events.length > DEFAULT_MAX_BATCH_SIZE) {
      throw new Error(
        `Batch size ${events.length} exceeds maximum ${DEFAULT_MAX_BATCH_SIZE}. ` +
        `Please split into smaller batches.`
      );
    }
    return this.sendBatch('product_viewed', events, options?.sync);
  }

  /**
   * Send a search_performed event.
   * Fired when a user searches for content
   *
   * @param event - SearchPerformed event to send
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   */
  async sendSearchPerformed(event: Models.SearchPerformed, options?: { sync?: boolean }): Promise<APIResponse> {
    return this.sendEvent('search_performed', event, options?.sync);
  }

  /**
   * Send a batch of search_performed events.
   *
   * @param events - Array of SearchPerformed events to send (max 1000)
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   * @throws {Error} If batch size exceeds maximum
   */
  async sendSearchPerformedBatch(events: Models.SearchPerformed[], options?: { sync?: boolean }): Promise<APIResponse> {
    if (events.length > DEFAULT_MAX_BATCH_SIZE) {
      throw new Error(
        `Batch size ${events.length} exceeds maximum ${DEFAULT_MAX_BATCH_SIZE}. ` +
        `Please split into smaller batches.`
      );
    }
    return this.sendBatch('search_performed', events, options?.sync);
  }

  /**
   * Send a added_to_cart event.
   * Fired when a user adds an item to cart
   *
   * @param event - AddedToCart event to send
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   */
  async sendAddedToCart(event: Models.AddedToCart, options?: { sync?: boolean }): Promise<APIResponse> {
    return this.sendEvent('added_to_cart', event, options?.sync);
  }

  /**
   * Send a batch of added_to_cart events.
   *
   * @param events - Array of AddedToCart events to send (max 1000)
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   * @throws {Error} If batch size exceeds maximum
   */
  async sendAddedToCartBatch(events: Models.AddedToCart[], options?: { sync?: boolean }): Promise<APIResponse> {
    if (events.length > DEFAULT_MAX_BATCH_SIZE) {
      throw new Error(
        `Batch size ${events.length} exceeds maximum ${DEFAULT_MAX_BATCH_SIZE}. ` +
        `Please split into smaller batches.`
      );
    }
    return this.sendBatch('added_to_cart', events, options?.sync);
  }

  /**
   * Send a checkout_started event.
   * Fired when a user begins checkout process
   *
   * @param event - CheckoutStarted event to send
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   */
  async sendCheckoutStarted(event: Models.CheckoutStarted, options?: { sync?: boolean }): Promise<APIResponse> {
    return this.sendEvent('checkout_started', event, options?.sync);
  }

  /**
   * Send a batch of checkout_started events.
   *
   * @param events - Array of CheckoutStarted events to send (max 1000)
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   * @throws {Error} If batch size exceeds maximum
   */
  async sendCheckoutStartedBatch(events: Models.CheckoutStarted[], options?: { sync?: boolean }): Promise<APIResponse> {
    if (events.length > DEFAULT_MAX_BATCH_SIZE) {
      throw new Error(
        `Batch size ${events.length} exceeds maximum ${DEFAULT_MAX_BATCH_SIZE}. ` +
        `Please split into smaller batches.`
      );
    }
    return this.sendBatch('checkout_started', events, options?.sync);
  }

  /**
   * Send a order_completed event.
   * Fired when an order is successfully placed
   *
   * @param event - OrderCompleted event to send
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   */
  async sendOrderCompleted(event: Models.OrderCompleted, options?: { sync?: boolean }): Promise<APIResponse> {
    return this.sendEvent('order_completed', event, options?.sync);
  }

  /**
   * Send a batch of order_completed events.
   *
   * @param events - Array of OrderCompleted events to send (max 1000)
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   * @throws {Error} If batch size exceeds maximum
   */
  async sendOrderCompletedBatch(events: Models.OrderCompleted[], options?: { sync?: boolean }): Promise<APIResponse> {
    if (events.length > DEFAULT_MAX_BATCH_SIZE) {
      throw new Error(
        `Batch size ${events.length} exceeds maximum ${DEFAULT_MAX_BATCH_SIZE}. ` +
        `Please split into smaller batches.`
      );
    }
    return this.sendBatch('order_completed', events, options?.sync);
  }

  /**
   * Send a user_signup event.
   * Fired when a new user signs up
   *
   * @param event - UserSignup event to send
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   */
  async sendUserSignup(event: Models.UserSignup, options?: { sync?: boolean }): Promise<APIResponse> {
    return this.sendEvent('user_signup', event, options?.sync);
  }

  /**
   * Send a batch of user_signup events.
   *
   * @param events - Array of UserSignup events to send (max 1000)
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   * @throws {Error} If batch size exceeds maximum
   */
  async sendUserSignupBatch(events: Models.UserSignup[], options?: { sync?: boolean }): Promise<APIResponse> {
    if (events.length > DEFAULT_MAX_BATCH_SIZE) {
      throw new Error(
        `Batch size ${events.length} exceeds maximum ${DEFAULT_MAX_BATCH_SIZE}. ` +
        `Please split into smaller batches.`
      );
    }
    return this.sendBatch('user_signup', events, options?.sync);
  }

  /**
   * Send a purchase event.
   * Fired when a user makes a purchase
   *
   * @param event - Purchase event to send
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   */
  async sendPurchase(event: Models.Purchase, options?: { sync?: boolean }): Promise<APIResponse> {
    return this.sendEvent('purchase', event, options?.sync);
  }

  /**
   * Send a batch of purchase events.
   *
   * @param events - Array of Purchase events to send (max 1000)
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   * @throws {Error} If batch size exceeds maximum
   */
  async sendPurchaseBatch(events: Models.Purchase[], options?: { sync?: boolean }): Promise<APIResponse> {
    if (events.length > DEFAULT_MAX_BATCH_SIZE) {
      throw new Error(
        `Batch size ${events.length} exceeds maximum ${DEFAULT_MAX_BATCH_SIZE}. ` +
        `Please split into smaller batches.`
      );
    }
    return this.sendBatch('purchase', events, options?.sync);
  }

  /**
   * Send a page_view event.
   * Fired when a user views a page
   *
   * @param event - PageView event to send
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   */
  async sendPageView(event: Models.PageView, options?: { sync?: boolean }): Promise<APIResponse> {
    return this.sendEvent('page_view', event, options?.sync);
  }

  /**
   * Send a batch of page_view events.
   *
   * @param events - Array of PageView events to send (max 1000)
   * @param options - Optional parameters
   * @param options.sync - If true, waits for Kafka ACK (default: false for async)
   * @returns API response
   * @throws {ClientError} For 4xx client errors (non-retriable)
   * @throws {ServerError} For 5xx server errors (after retries exhausted)
   * @throws {NetworkError} For network errors (after retries exhausted)
   * @throws {Error} If batch size exceeds maximum
   */
  async sendPageViewBatch(events: Models.PageView[], options?: { sync?: boolean }): Promise<APIResponse> {
    if (events.length > DEFAULT_MAX_BATCH_SIZE) {
      throw new Error(
        `Batch size ${events.length} exceeds maximum ${DEFAULT_MAX_BATCH_SIZE}. ` +
        `Please split into smaller batches.`
      );
    }
    return this.sendBatch('page_view', events, options?.sync);
  }


  /**
   * Send a single event with retry logic.
   *
   * @param eventType - Event type name
   * @param event - Event data
   * @param sync - If true, waits for Kafka ACK (sends ?sync=true)
   * @returns API response
   */
  private async sendEvent(eventType: string, event: any, sync?: boolean): Promise<APIResponse> {
    let url = `${this.apiUrl}/v1/events/${eventType}`;

    // Add ?sync=true for guaranteed Kafka delivery
    if (sync) {
      url += '?sync=true';
    }

    const requestId = generateRequestId();

    try {
      const result = await this.requestWithRetry('POST', url, event, requestId);

      // SUCCESS: Call success callback if provided
      if (this.onSuccess) {
        try {
          this.onSuccess(event, result);
        } catch (error) {
          this.logger.error(`Success callback failed: ${error}`);
        }
      }

      return result;
    } catch (error) {
      // FAILURE: Call failure callback if provided
      if (this.onFailure) {
        try {
          this.onFailure(event, error as Error);
        } catch (cbError) {
          this.logger.error(`Failure callback failed: ${cbError}`);
        }
      }

      throw error;
    }
  }

  /**
   * Send a batch of events with retry logic.
   *
   * @param eventType - Event type name
   * @param events - Array of events
   * @param sync - If true, waits for Kafka ACK (sends ?sync=true)
   * @returns API response
   */
  private async sendBatch(eventType: string, events: any[], sync?: boolean): Promise<APIResponse> {
    let url = `${this.apiUrl}/v1/events/${eventType}/batch`;

    // Add ?sync=true for guaranteed Kafka delivery
    if (sync) {
      url += '?sync=true';
    }

    const requestId = generateRequestId();

    const payload = { events };

    try {
      const result = await this.requestWithRetry('POST', url, payload, requestId);

      // SUCCESS: Call success callback if provided
      if (this.onSuccess) {
        try {
          this.onSuccess(payload, result);
        } catch (error) {
          this.logger.error(`Success callback failed: ${error}`);
        }
      }

      return result;
    } catch (error) {
      // FAILURE: Call failure callback if provided
      if (this.onFailure) {
        try {
          this.onFailure(payload, error as Error);
        } catch (cbError) {
          this.logger.error(`Failure callback failed: ${cbError}`);
        }
      }

      throw error;
    }
  }

  /**
   * Make HTTP request with exponential backoff retry logic.
   *
   * @param method - HTTP method
   * @param url - Request URL
   * @param body - Request body
   * @param requestId - Unique request ID for tracking
   * @returns API response
   * @throws {ClientError} For 4xx errors (non-retriable)
   * @throws {ServerError} For 5xx errors after retries exhausted
   * @throws {NetworkError} For network errors after retries exhausted
   */
  private async requestWithRetry(
    method: string,
    url: string,
    body: any,
    requestId: string
  ): Promise<APIResponse> {
    let lastError: Error | null = null;

    for (let attempt = 1; attempt <= this.maxRetries; attempt++) {
      try {
        if (this.debug) {
          this.logger.debug(
            `Request attempt ${attempt}/${this.maxRetries}: ` +
            `${method} ${url} [request_id: ${requestId}]`
          );
        }

        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), this.timeout);

        try {
          const response = await fetch(url, {
            method,
            headers: {
              'Authorization': `Bearer ${this.apiKey}`,
              'Content-Type': 'application/json',
              'X-Request-ID': requestId
            },
            body: JSON.stringify(body),
            signal: controller.signal
          });

          clearTimeout(timeoutId);

          // Check response status
          if (response.ok) {
            if (this.debug) {
              this.logger.debug(
                `Request successful: HTTP ${response.status} [request_id: ${requestId}]`
              );
            }
            return response.json();
          }

          // Get error message
          let errorMsg: string;
          try {
            errorMsg = await response.text();
          } catch {
            errorMsg = response.statusText;
          }

          // Classify error
          if (
            response.status >= HTTP_CLIENT_ERROR_START &&
            response.status <= HTTP_CLIENT_ERROR_END
          ) {
            // 4xx errors are non-retriable client errors
            this.logger.error(
              `Client error: HTTP ${response.status} - ${errorMsg} ` +
              `[request_id: ${requestId}]`
            );
            throw new ClientError(response.status, errorMsg, requestId);
          } else if (
            response.status >= HTTP_SERVER_ERROR_START &&
            response.status <= HTTP_SERVER_ERROR_END
          ) {
            // 5xx errors are retriable server errors
            lastError = new ServerError(response.status, errorMsg, requestId);

            if (attempt < this.maxRetries) {
              const backoffDelay = this.retryBackoffBase * Math.pow(2, attempt - 1);
              this.logger.warn(
                `Server error (attempt ${attempt}/${this.maxRetries}): ` +
                `HTTP ${response.status} - ${errorMsg}. ` +
                `Retrying in ${backoffDelay}ms... [request_id: ${requestId}]`
              );
              await sleep(backoffDelay);
              continue;
            } else {
              this.logger.error(
                `Server error after ${this.maxRetries} attempts: ` +
                `HTTP ${response.status} - ${errorMsg} [request_id: ${requestId}]`
              );
              throw lastError;
            }
          }
        } catch (error) {
          clearTimeout(timeoutId);
          throw error;
        }
      } catch (error: any) {
        // Handle fetch errors (timeout, network, etc.)
        if (error instanceof ClientError) {
          // Client errors are not retriable
          throw error;
        }

        if (error.name === 'AbortError') {
          lastError = new NetworkError('Request timeout');
        } else if (error instanceof ServerError) {
          lastError = error;
        } else {
          lastError = new NetworkError(`Request failed: ${error.message}`);
        }

        if (attempt < this.maxRetries) {
          const backoffDelay = this.retryBackoffBase * Math.pow(2, attempt - 1);
          this.logger.warn(
            `${lastError.message} (attempt ${attempt}/${this.maxRetries}). ` +
            `Retrying in ${backoffDelay}ms... [request_id: ${requestId}]`
          );
          await sleep(backoffDelay);
          continue;
        } else {
          this.logger.error(
            `${lastError.message} after ${this.maxRetries} attempts ` +
            `[request_id: ${requestId}]`
          );
          throw lastError;
        }
      }
    }

    // Should not reach here, but just in case
    if (lastError) {
      throw lastError;
    }
    throw new NetworkError('Request failed for unknown reason');
  }

  /**
   * Check if the IngestKit API is reachable.
   *
   * @returns True if API is healthy, False otherwise
   */
  async healthCheck(): Promise<boolean> {
    try {
      const url = `${this.apiUrl}/health`;
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 5000);

      const response = await fetch(url, {
        signal: controller.signal
      });

      clearTimeout(timeoutId);
      return response.ok;
    } catch (error: any) {
      if (this.debug) {
        this.logger.error(`Health check failed: ${error.message}`);
      }
      return false;
    }
  }
}

// Convenience alias
export const Client = IngestKitClient;
