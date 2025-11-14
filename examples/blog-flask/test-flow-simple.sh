#!/bin/bash

# Simple blog user journey demonstration (no external dependencies)

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
curl -s "http://localhost:5002/search?q=python" -H "X-User-Id: $USER_ID"
echo -e "\n"

# 2. View article from search results
echo -e "${GREEN}2. Viewing 'Getting Started with Python' article...${NC}"
curl -s http://localhost:5002/articles/getting-started-with-python \
  -H "X-User-Id: $USER_ID"
echo -e "\n"

# 3. Share article on Twitter
echo -e "${GREEN}3. Sharing article on Twitter...${NC}"
curl -s -X POST http://localhost:5002/articles/getting-started-with-python/share \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d '{"platform":"twitter"}'
echo -e "\n"

# 4. Post a comment
echo -e "${GREEN}4. Posting a comment...${NC}"
curl -s -X POST \
  http://localhost:5002/articles/getting-started-with-python/comments \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d '{"text":"This is exactly what I was looking for! Very helpful tutorial."}'
echo -e "\n"

# 5. Subscribe to newsletter
echo -e "${GREEN}5. Subscribing to newsletter...${NC}"
curl -s -X POST http://localhost:5002/subscribe \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d "{
    \"email\": \"reader_${USER_ID}@example.com\",
    \"type\": \"weekly\",
    \"source\": \"article\"
  }"
echo -e "\n"

# 6. View another article
echo -e "${GREEN}6. Viewing 'Machine Learning' article...${NC}"
curl -s http://localhost:5002/articles/intro-to-machine-learning \
  -H "X-User-Id: $USER_ID"
echo -e "\n"

# 7. Share on LinkedIn
echo -e "${GREEN}7. Sharing on LinkedIn...${NC}"
curl -s -X POST http://localhost:5002/articles/intro-to-machine-learning/share \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d '{"platform":"linkedin"}'
echo -e "\n"

echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}✅ Complete user journey finished!${NC}"
echo -e "${BLUE}========================================${NC}"
echo
echo "Events tracked:"
echo "  - 1x search_performed"
echo "  - 2x article_viewed"
echo "  - 2x article_shared (Twitter, LinkedIn)"
echo "  - 1x comment_posted"
echo "  - 1x newsletter_subscribed"
echo
echo "Check the Flask app logs to see tracked events!"
