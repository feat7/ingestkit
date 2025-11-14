#!/bin/bash

# Complete blog user journey demonstration

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Generate unique user ID
USER_ID="user_$(date +%s)"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}IngestKit Blog Analytics Example${NC}"
echo -e "${BLUE}========================================${NC}"
echo
echo "User: $USER_ID"
echo

# 1. Search for Python articles
echo -e "${GREEN}1. Searching for 'python' articles...${NC}"
SEARCH_RESULTS=$(curl -s "http://localhost:5000/search?q=python" -H "X-User-Id: $USER_ID")
echo "$SEARCH_RESULTS" | python -m json.tool | grep -A 2 '"results"' | head -10
RESULTS_COUNT=$(echo "$SEARCH_RESULTS" | python -c "import sys, json; print(json.load(sys.stdin)['results_count'])")
echo "Found $RESULTS_COUNT article(s)"
echo

# 2. View article from search results
echo -e "${GREEN}2. Viewing 'Getting Started with Python' article...${NC}"
ARTICLE=$(curl -s http://localhost:5000/articles/getting-started-with-python \
  -H "X-User-Id: $USER_ID")
TITLE=$(echo "$ARTICLE" | python -c "import sys, json; print(json.load(sys.stdin)['article']['title'])")
AUTHOR=$(echo "$ARTICLE" | python -c "import sys, json; print(json.load(sys.stdin)['article']['author'])")
echo "Title: $TITLE"
echo "Author: $AUTHOR"
echo

# 3. Share article on Twitter
echo -e "${GREEN}3. Sharing article on Twitter...${NC}"
SHARE_RESPONSE=$(curl -s -X POST http://localhost:5000/articles/getting-started-with-python/share \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d '{"platform":"twitter"}')
echo "$SHARE_RESPONSE" | python -m json.tool
echo

# 4. Post a comment
echo -e "${GREEN}4. Posting a comment...${NC}"
COMMENT_RESPONSE=$(curl -s -X POST \
  http://localhost:5000/articles/getting-started-with-python/comments \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d '{"text":"This is exactly what I was looking for! Very helpful tutorial."}')
COMMENT_ID=$(echo "$COMMENT_RESPONSE" | python -c "import sys, json; print(json.load(sys.stdin)['comment_id'])")
echo "Comment posted: $COMMENT_ID"
echo

# 5. Reply to comment
echo -e "${GREEN}5. Posting a reply to comment...${NC}"
REPLY_RESPONSE=$(curl -s -X POST \
  http://localhost:5000/articles/getting-started-with-python/comments \
  -H "Content-Type: application/json" \
  -H "X-User-Id: author_jane" \
  -d "{\"text\":\"Thank you! Glad it helped.\",\"parent_comment_id\":\"$COMMENT_ID\"}")
REPLY_ID=$(echo "$REPLY_RESPONSE" | python -c "import sys, json; print(json.load(sys.stdin)['comment_id'])")
echo "Reply posted: $REPLY_ID"
echo

# 6. Subscribe to newsletter
echo -e "${GREEN}6. Subscribing to newsletter...${NC}"
SUBSCRIBE_RESPONSE=$(curl -s -X POST http://localhost:5000/subscribe \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d "{
    \"email\": \"reader_${USER_ID}@example.com\",
    \"type\": \"weekly\",
    \"source\": \"article\"
  }")
echo "$SUBSCRIBE_RESPONSE" | python -m json.tool
echo

# 7. View another article
echo -e "${GREEN}7. Viewing 'Machine Learning' article...${NC}"
ML_ARTICLE=$(curl -s http://localhost:5000/articles/intro-to-machine-learning \
  -H "X-User-Id: $USER_ID" \
  -H "X-Session-Id: session_123")
ML_TITLE=$(echo "$ML_ARTICLE" | python -c "import sys, json; print(json.load(sys.stdin)['article']['title'])")
echo "Title: $ML_TITLE"
echo

# 8. Share on LinkedIn
echo -e "${GREEN}8. Sharing on LinkedIn...${NC}"
curl -s -X POST http://localhost:5000/articles/intro-to-machine-learning/share \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d '{"platform":"linkedin"}' | python -m json.tool
echo

echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}✅ Complete user journey finished!${NC}"
echo -e "${BLUE}========================================${NC}"
echo
echo "Events tracked:"
echo "  - 1x search_performed"
echo "  - 2x article_viewed"
echo "  - 2x article_shared (Twitter, LinkedIn)"
echo "  - 2x comment_posted (comment + reply)"
echo "  - 1x newsletter_subscribed"
echo
echo "Query events in database with:"
echo "  make db-connect"
echo "  SELECT event_type, COUNT(*) FROM ("
echo "    SELECT 'article_viewed' as event_type FROM events_article_viewed WHERE tenant_id='blog-demo'"
echo "    UNION ALL SELECT 'article_shared' FROM events_article_shared WHERE tenant_id='blog-demo'"
echo "    UNION ALL SELECT 'comment_posted' FROM events_comment_posted WHERE tenant_id='blog-demo'"
echo "    UNION ALL SELECT 'newsletter_subscribed' FROM events_newsletter_subscribed WHERE tenant_id='blog-demo'"
echo "    UNION ALL SELECT 'search_performed' FROM events_search_performed WHERE tenant_id='blog-demo'"
echo "  ) t GROUP BY event_type;"
