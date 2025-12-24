"""
IngestKit Python SDK
Auto-generated from schema version 1.0
"""

from .client import IngestKitClient
from .models import (
    ArticleViewed,
    ArticleShared,
    CommentPosted,
    NewsletterSubscribed,
    SearchPerformed,
    CheckoutStarted,
    PageView,
    ProductViewed,
    AddedToCart,
    OrderCompleted,
    UserSignup,
    Purchase,
)

# Convenience alias
Client = IngestKitClient

__version__ = "1.0"
__all__ = [
    "IngestKitClient",
    "Client",
    "ArticleViewed",
    "ArticleShared",
    "CommentPosted",
    "NewsletterSubscribed",
    "SearchPerformed",
    "CheckoutStarted",
    "PageView",
    "ProductViewed",
    "AddedToCart",
    "OrderCompleted",
    "UserSignup",
    "Purchase",
]
