# Blog Flask Example

A complete example showing how to integrate IngestKit into a Flask blog application for content analytics and user engagement tracking.

## What This Example Shows

This example demonstrates a blog platform that tracks:

- **Article Views** - When users read articles with source attribution
- **Article Shares** - When users share content on social platforms
- **Comments Posted** - When users engage with comments (including replies)
- **Newsletter Subscriptions** - When users subscribe to newsletters
- **Search Queries** - What users search for and click on

## Architecture

```
Flask API
    ↓
IngestKit Python Client (auto-generated with Pydantic)
    ↓
IngestKit API Server
    ↓
PostgreSQL (for analytics)
```

## Setup Instructions

### 1. Prerequisites

Make sure IngestKit server is running:

```bash
# From the IngestKit root directory
cd ../..
make up              # Start PostgreSQL and Redpanda
make run-api         # Start API server (port 8080)
make run-consumer    # Start consumer (port 8081)
```

### 2. Initialize IngestKit (Already Done)

This example already has IngestKit initialized with:
- Custom schema for blog events (`ingestkit/schema.yaml`)
- Configuration file (`ingestkit.config.json`)

To see how this was created:
```bash
# This is what was run (you don't need to run this)
# pip install ingestkit
# ingestkit init --python
# Then schema.yaml was customized for blog events
```

### 3. Generate IngestKit Client

Generate the type-safe Python client from the schema:

```bash
# Make sure you're in the example directory
cd examples/blog-flask

# Generate the client
../../bin/ingestkit generate
```

This creates:
- `ingestkit/client.py` - Type-safe client with methods for each event
- `ingestkit/models.py` - Pydantic models for validation
- `ingestkit/__init__.py` - Package exports

### 4. Install Dependencies

```bash
# Create virtual environment (recommended)
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt
```

### 5. Configure Environment

```bash
cp .env.example .env
```

The default API key (`dev_key_1234567890`) should work with the default IngestKit setup.

### 6. Run the Server

**Option A: Quick Start (Recommended)**

```bash
./run.sh
```

This script will:
- Check if IngestKit server is running
- Generate the client if needed
- Create virtual environment if needed
- Install dependencies
- Start the Flask app

**Option B: Manual Start**

```bash
# Activate virtual environment
source venv/bin/activate

# Run the app
python app.py
```

The server will start on `http://localhost:5000`.

## Usage Examples

### 1. List Articles

```bash
curl http://localhost:5000/articles
```

### 2. View Article (Tracks `article_viewed` event)

```bash
curl http://localhost:5000/articles/getting-started-with-python \
  -H "X-User-Id: user_123"
```

### 3. Share Article (Tracks `article_shared` event)

```bash
curl -X POST http://localhost:5000/articles/getting-started-with-python/share \
  -H "Content-Type: application/json" \
  -H "X-User-Id: user_123" \
  -d '{
    "platform": "twitter"
  }'
```

### 4. Post Comment (Tracks `comment_posted` event)

```bash
curl -X POST http://localhost:5000/articles/getting-started-with-python/comments \
  -H "Content-Type: application/json" \
  -H "X-User-Id: user_123" \
  -d '{
    "text": "Great article! Really helpful for beginners."
  }'
```

### 5. Subscribe to Newsletter (Tracks `newsletter_subscribed` event)

```bash
curl -X POST http://localhost:5000/subscribe \
  -H "Content-Type: application/json" \
  -d '{
    "email": "reader@example.com",
    "type": "weekly",
    "source": "article"
  }'
```

### 6. Search Articles (Tracks `search_performed` event)

```bash
curl http://localhost:5000/search?q=python
```

## Complete User Journey

**Quick Test (Recommended):**

```bash
./test-flow-simple.sh
```

This runs a complete user journey with all event types.

**Detailed Test (Requires `jq` and Python):**

```bash
# Set user ID
USER_ID="user_$(date +%s)"
echo "User: $USER_ID"
echo

# 1. Search for Python articles
echo "1. Searching for 'python'..."
curl -s "http://localhost:5000/search?q=python" | \
  python -m json.tool | grep -A 3 '"title"'
echo

# 2. View article from search results
echo "2. Viewing article..."
curl -s http://localhost:5000/articles/getting-started-with-python \
  -H "X-User-Id: $USER_ID" | \
  python -m json.tool | grep '"title"'
echo

# 3. Share article on Twitter
echo "3. Sharing article on Twitter..."
curl -s -X POST http://localhost:5000/articles/getting-started-with-python/share \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d '{"platform":"twitter"}' | \
  python -m json.tool
echo

# 4. Post a comment
echo "4. Posting a comment..."
COMMENT_RESPONSE=$(curl -s -X POST \
  http://localhost:5000/articles/getting-started-with-python/comments \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d '{"text":"This is exactly what I was looking for! Thanks!"}')
COMMENT_ID=$(echo $COMMENT_RESPONSE | python -c "import sys, json; print(json.load(sys.stdin)['comment_id'])")
echo "Comment ID: $COMMENT_ID"
echo

# 5. Subscribe to newsletter
echo "5. Subscribing to newsletter..."
curl -s -X POST http://localhost:5000/subscribe \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d '{
    "email": "reader@example.com",
    "type": "weekly",
    "source": "article"
  }' | python -m json.tool
echo

echo "Complete user journey finished!"
echo
echo "Events tracked:"
echo "  - 1x search_performed"
echo "  - 1x article_viewed"
echo "  - 1x article_shared"
echo "  - 1x comment_posted"
echo "  - 1x newsletter_subscribed"
```

Save this as `test-flow.sh` and run with `bash test-flow.sh`.

## Verify Events in Database

Connect to the database and query your events:

```bash
# From IngestKit root
make db-connect
```

Then run these queries:

```sql
-- View all article views
SELECT
  article_title,
  author,
  category,
  source,
  COUNT(*) as views,
  COUNT(DISTINCT session_id) as unique_sessions
FROM events_article_viewed
WHERE tenant_id = 'blog-demo'
GROUP BY article_title, author, category, source
ORDER BY views DESC;

-- View all shares by platform
SELECT
  platform,
  COUNT(*) as share_count,
  COUNT(DISTINCT article_id) as articles_shared
FROM events_article_shared
WHERE tenant_id = 'blog-demo'
GROUP BY platform
ORDER BY share_count DESC;

-- View comment activity
SELECT
  article_id,
  COUNT(*) as total_comments,
  COUNT(DISTINCT user_id) as unique_commenters,
  AVG(comment_length) as avg_comment_length
FROM events_comment_posted
WHERE tenant_id = 'blog-demo'
GROUP BY article_id
ORDER BY total_comments DESC;

-- Newsletter subscriptions by type
SELECT
  subscription_type,
  source,
  COUNT(*) as subscribers
FROM events_newsletter_subscribed
WHERE tenant_id = 'blog-demo'
GROUP BY subscription_type, source
ORDER BY subscribers DESC;

-- Popular search queries
SELECT
  query,
  COUNT(*) as search_count,
  AVG(results_count) as avg_results,
  COUNT(DISTINCT session_id) as unique_searchers
FROM events_search_performed
WHERE tenant_id = 'blog-demo'
GROUP BY query
ORDER BY search_count DESC
LIMIT 20;

-- Content engagement funnel
SELECT
  'Article Views' as stage,
  COUNT(*) as count,
  COUNT(DISTINCT session_id) as unique_users
FROM events_article_viewed
WHERE tenant_id = 'blog-demo'
UNION ALL
SELECT
  'Shares',
  COUNT(*),
  COUNT(DISTINCT session_id)
FROM events_article_shared
WHERE tenant_id = 'blog-demo'
UNION ALL
SELECT
  'Comments',
  COUNT(*),
  COUNT(DISTINCT session_id)
FROM events_comment_posted
WHERE tenant_id = 'blog-demo'
UNION ALL
SELECT
  'Subscriptions',
  COUNT(*),
  COUNT(DISTINCT session_id)
FROM events_newsletter_subscribed
WHERE tenant_id = 'blog-demo';
```

## Code Highlights

### How Events Are Tracked

In `app.py`, events are tracked using the auto-generated Pydantic client:

```python
from ingestkit import Client

analytics = Client()

# Track article view
analytics.article_viewed.send({
    'user_id': user_id,
    'session_id': session_id,
    'article_id': article['id'],
    'article_title': article['title'],
    'author': article['author'],
    'category': article['category'],
    'tags': article['tags'],
    'read_time_seconds': article['read_time_seconds'],
    'source': request.args.get('source', 'direct'),
    'referrer': request.referrer,
})

# Track comment
analytics.comment_posted.send({
    'user_id': user_id,
    'session_id': session_id,
    'article_id': article_id,
    'comment_id': comment_id,
    'parent_comment_id': parent_comment_id,
    'comment_length': len(comment_text),
})
```

### Pydantic Validation

The generated client uses Pydantic for type safety:

```python
# This will raise validation error if fields are missing or wrong type
analytics.article_viewed.send({
    'user_id': 123,  # Wrong - should be string
    'article_title': None,  # Wrong - required field
})
```

### Custom Event Schema

The schema in `ingestkit/schema.yaml` defines blog-specific events:

- **article_viewed** - Tracks article reads with traffic source
- **article_shared** - Tracks social sharing by platform
- **comment_posted** - Tracks comments including replies
- **newsletter_subscribed** - Tracks subscriptions by type and source
- **search_performed** - Tracks queries and result clicks

## What You Can Build

With this pattern, you can track and analyze:

- **Content Performance** - Which articles get the most views, shares, comments?
- **Traffic Sources** - Where do your readers come from?
- **Search Insights** - What are users looking for? Are they finding it?
- **Engagement Metrics** - Read time, comment rate, share rate
- **Conversion Funnels** - How many readers → subscribers?
- **User Behavior** - Session analysis, return visits, content preferences
- **Author Performance** - Which authors drive the most engagement?

## Next Steps

1. **Customize Events** - Add more fields like reading progress, time on page
2. **Add More Features** - Track bookmarks, likes, author follows
3. **Build Dashboards** - Create SQL views or use BI tools
4. **Personalization** - Use event data to recommend content
5. **A/B Testing** - Track experiment variants in metadata

## Troubleshooting

### "Client not generated yet" error

Run `../../bin/ingestkit generate` from this directory.

### ImportError for ingestkit

Make sure you've installed dependencies: `pip install -r requirements.txt`

### Events not appearing in database

1. Check consumer is running: `curl http://localhost:8081/health`
2. Check metrics: `make metrics` (from root)
3. Check DLQ: `make db-dlq-check`

### Pydantic validation errors

Check your event data matches the schema types in `ingestkit/schema.yaml`.

## Learn More

- [IngestKit README](../../README.md) - Full documentation
- [SDK Generation Guide](../../docs/sdk-generation.md) - CLI workflow guide
- [E-Commerce Example](../ecommerce-express/) - TypeScript/Express example
