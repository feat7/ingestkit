/**
 * IngestKit TypeScript SDK
 * Auto-generated from schema version 1.0
 */

// Schema version constant
export const SCHEMA_VERSION = "1.0";

export { IngestKitClient } from './client';
export type { IngestKitClientConfig, APIResponse } from './client';
export * as Models from './models';
export type {
  ArticleViewed,
  ArticleShared,
  CommentPosted,
  NewsletterSubscribed,
  ProductViewed,
  SearchPerformed,
  AddedToCart,
  CheckoutStarted,
  OrderCompleted,
  UserSignup,
  Purchase,
  PageView,
} from './models';

// Convenience alias
export { IngestKitClient as Client } from './client';
