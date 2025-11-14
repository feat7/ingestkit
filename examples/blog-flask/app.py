"""
Blog Flask Example - IngestKit Integration

A simple blog application demonstrating IngestKit event tracking for:
- Article views
- Article shares
- Comment posting
- Newsletter subscriptions
- Search queries
"""

import os
import uuid
from datetime import datetime
from typing import Optional
from flask import Flask, request, jsonify, session
from dotenv import load_dotenv
from ingestkit import Client

# Load environment variables
load_dotenv()

app = Flask(__name__)
app.secret_key = os.getenv("SECRET_KEY", "dev-secret-key-change-in-production")

# Initialize IngestKit client
analytics = Client()

# In-memory data (for demo purposes)
articles = [
    {
        "id": "getting-started-with-python",
        "title": "Getting Started with Python in 2024",
        "author": "Jane Doe",
        "category": "Programming",
        "tags": ["python", "tutorial", "beginners"],
        "read_time_seconds": 300,
        "content": "Python is a versatile programming language...",
    },
    {
        "id": "intro-to-machine-learning",
        "title": "Introduction to Machine Learning",
        "author": "John Smith",
        "category": "AI/ML",
        "tags": ["machine-learning", "ai", "tutorial"],
        "read_time_seconds": 600,
        "content": "Machine learning is transforming industries...",
    },
    {
        "id": "web-development-best-practices",
        "title": "Web Development Best Practices",
        "author": "Alice Johnson",
        "category": "Web Development",
        "tags": ["web", "best-practices", "javascript"],
        "read_time_seconds": 420,
        "content": "Building great web applications requires...",
    },
    {
        "id": "database-optimization-tips",
        "title": "Database Optimization Tips",
        "author": "Bob Wilson",
        "category": "Database",
        "tags": ["database", "optimization", "sql"],
        "read_time_seconds": 480,
        "content": "Optimizing database queries is crucial...",
    },
]

comments = {}
subscribers = []


def get_session_id() -> str:
    """Get or create session ID"""
    if "session_id" not in session:
        session["session_id"] = str(uuid.uuid4())
    return session["session_id"]


def get_user_id() -> Optional[str]:
    """Get user ID if logged in"""
    # For demo, using header. In production, use real authentication
    return request.headers.get("X-User-Id")


@app.route("/")
def index():
    """Homepage - list all articles"""
    return jsonify(
        {
            "message": "Blog API",
            "articles": len(articles),
            "endpoints": {
                "GET /articles": "List all articles",
                "GET /articles/<id>": "View article",
                "POST /articles/<id>/share": "Share article",
                "POST /articles/<id>/comments": "Post comment",
                "POST /subscribe": "Subscribe to newsletter",
                "GET /search?q=<query>": "Search articles",
            },
        }
    )


@app.route("/articles")
def list_articles():
    """List all articles"""
    return jsonify(
        {
            "articles": [
                {
                    "id": a["id"],
                    "title": a["title"],
                    "author": a["author"],
                    "category": a["category"],
                    "tags": a["tags"],
                    "read_time_seconds": a["read_time_seconds"],
                }
                for a in articles
            ]
        }
    )


@app.route("/articles/<article_id>")
def view_article(article_id: str):
    """View an article (tracks article_viewed event)"""
    article = next((a for a in articles if a["id"] == article_id), None)

    if not article:
        return jsonify({"error": "Article not found"}), 404

    user_id = get_user_id()
    session_id = get_session_id()

    # Track article view event
    try:
        analytics.send_article_viewed({
            "user_id": user_id,
            "session_id": session_id,
            "article_id": article["id"],
            "article_title": article["title"],
            "author": article["author"],
            "category": article["category"],
            "tags": article["tags"],
            "read_time_seconds": article["read_time_seconds"],
            "source": request.args.get("source", "direct"),
            "referrer": request.referrer,
            "metadata": {
                "user_agent": request.headers.get("User-Agent"),
                "ip": request.remote_addr,
            },
        })

        print(
            f"✓ Tracked article view: {article['title']} (user: {user_id or 'anonymous'})"
        )
    except Exception as e:
        print(f"Failed to track article view: {e}")

    return jsonify({"article": article})


@app.route("/articles/<article_id>/share", methods=["POST"])
def share_article(article_id: str):
    """Share an article (tracks article_shared event)"""
    article = next((a for a in articles if a["id"] == article_id), None)

    if not article:
        return jsonify({"error": "Article not found"}), 404

    data = request.get_json()
    platform = data.get("platform")  # twitter, facebook, linkedin, email

    if not platform:
        return jsonify({"error": "Platform is required"}), 400

    user_id = get_user_id()
    session_id = get_session_id()

    # Track article share event
    try:
        analytics.send_article_shared({
            "user_id": user_id,
            "session_id": session_id,
            "article_id": article["id"],
            "article_title": article["title"],
            "platform": platform,
            "metadata": {
                "article_author": article["author"],
                "article_category": article["category"],
            },
        })

        print(
            f"✓ Tracked article share: {article['title']} on {platform} (user: {user_id or 'anonymous'})"
        )
    except Exception as e:
        print(f"Failed to track article share: {e}")

    return jsonify({"success": True, "message": f"Article shared on {platform}"})


@app.route("/articles/<article_id>/comments", methods=["POST"])
def post_comment(article_id: str):
    """Post a comment (tracks comment_posted event)"""
    article = next((a for a in articles if a["id"] == article_id), None)

    if not article:
        return jsonify({"error": "Article not found"}), 404

    data = request.get_json()
    comment_text = data.get("text")
    parent_comment_id = data.get("parent_comment_id")

    if not comment_text:
        return jsonify({"error": "Comment text is required"}), 400

    user_id = get_user_id()
    if not user_id:
        return jsonify({"error": "Must be logged in to comment"}), 401

    session_id = get_session_id()
    comment_id = str(uuid.uuid4())

    # Save comment
    if article_id not in comments:
        comments[article_id] = []

    comments[article_id].append(
        {
            "id": comment_id,
            "user_id": user_id,
            "text": comment_text,
            "parent_id": parent_comment_id,
            "timestamp": datetime.utcnow().isoformat(),
        }
    )

    # Track comment posted event
    try:
        analytics.send_comment_posted({
            "user_id": user_id,
            "session_id": session_id,
            "article_id": article_id,
            "comment_id": comment_id,
            "parent_comment_id": parent_comment_id,
            "comment_length": len(comment_text),
            "metadata": {
                "article_title": article["title"],
                "is_reply": parent_comment_id is not None,
            },
        })

        print(
            f"✓ Tracked comment: {comment_id} on {article['title']} (user: {user_id})"
        )
    except Exception as e:
        print(f"Failed to track comment: {e}")

    return jsonify({"success": True, "comment_id": comment_id})


@app.route("/subscribe", methods=["POST"])
def subscribe():
    """Subscribe to newsletter (tracks newsletter_subscribed event)"""
    data = request.get_json()
    email = data.get("email")
    subscription_type = data.get("type", "weekly")  # weekly, daily, instant
    source = data.get("source", "homepage")  # article, homepage, popup

    if not email:
        return jsonify({"error": "Email is required"}), 400

    user_id = get_user_id()
    session_id = get_session_id()

    # Save subscription
    subscribers.append(
        {
            "email": email,
            "type": subscription_type,
            "timestamp": datetime.utcnow().isoformat(),
        }
    )

    # Track newsletter subscription event
    try:
        analytics.send_newsletter_subscribed({
            "user_id": user_id,
            "session_id": session_id,
            "email": email,
            "subscription_type": subscription_type,
            "source": source,
            "metadata": {
                "referrer": request.referrer,
            },
        })

        print(f"✓ Tracked newsletter subscription: {email} ({subscription_type})")
    except Exception as e:
        print(f"Failed to track newsletter subscription: {e}")

    return jsonify({"success": True, "message": "Subscribed successfully"})


@app.route("/search")
def search():
    """Search articles (tracks search_performed event)"""
    query = request.args.get("q", "").lower()

    if not query:
        return jsonify({"error": 'Query parameter "q" is required'}), 400

    # Simple search implementation
    results = [
        a
        for a in articles
        if query in a["title"].lower()
        or query in a["content"].lower()
        or any(query in tag for tag in a["tags"])
    ]

    user_id = get_user_id()
    session_id = get_session_id()
    clicked_result_id = request.args.get("clicked")

    # Track search performed event
    try:
        analytics.send_search_performed({
            "user_id": user_id,
            "session_id": session_id,
            "query": query,
            "results_count": len(results),
            "clicked_result_id": clicked_result_id,
            "metadata": {
                "search_type": "full_text",
            },
        })

        print(
            f"✓ Tracked search: '{query}' ({len(results)} results, user: {user_id or 'anonymous'})"
        )
    except Exception as e:
        print(f"Failed to track search: {e}")

    return jsonify(
        {
            "query": query,
            "results_count": len(results),
            "results": [
                {
                    "id": a["id"],
                    "title": a["title"],
                    "author": a["author"],
                    "category": a["category"],
                }
                for a in results
            ],
        }
    )


@app.route("/health")
def health():
    """Health check"""
    return jsonify({"status": "ok"})


if __name__ == "__main__":
    port = int(os.getenv("PORT", 5000))
    print(f"🚀 Blog API running on http://localhost:{port}")
    print(f"📊 IngestKit analytics enabled")
    print()
    print("Example requests:")
    print(f"  curl http://localhost:{port}/articles")
    print(f"  curl http://localhost:{port}/articles/getting-started-with-python")
    print(f"  curl http://localhost:{port}/search?q=python")
    print()
    app.run(debug=True, port=port)
