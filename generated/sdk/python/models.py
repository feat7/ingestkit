"""
IngestKit Python SDK - Event Models
Auto-generated from schema version 1.0
DO NOT EDIT MANUALLY
"""

from typing import Optional, Any
from datetime import datetime
from decimal import Decimal
from pydantic import BaseModel, Field


class ArticleViewed(BaseModel):
    """Fired when a user views a blog article"""

    tenant_id: str = Field(..., description="Tenant identifier")
    event_id: Optional[int] = Field(None, description="Event ID (auto-generated)")
    timestamp: Optional[datetime] = Field(None, description="Event timestamp (auto-generated)")

    user_id: Optional[str] = None
    session_id: str = Field(...)
    category: str = Field(...)
    tags: Optional[dict] = None
    read_time_seconds: Optional[int] = None
    source: Optional[str] = None
    referrer: Optional[str] = None
    article_id: str = Field(...)
    article_title: str = Field(...)
    author: str = Field(...)
    metadata: Optional[dict] = None

    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat(),
            Decimal: lambda v: float(v)
        }

class ArticleShared(BaseModel):
    """Fired when a user shares an article"""

    tenant_id: str = Field(..., description="Tenant identifier")
    event_id: Optional[int] = Field(None, description="Event ID (auto-generated)")
    timestamp: Optional[datetime] = Field(None, description="Event timestamp (auto-generated)")

    platform: str = Field(...)
    metadata: Optional[dict] = None
    user_id: Optional[str] = None
    session_id: str = Field(...)
    article_id: str = Field(...)
    article_title: str = Field(...)

    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat(),
            Decimal: lambda v: float(v)
        }

class CommentPosted(BaseModel):
    """Fired when a user posts a comment"""

    tenant_id: str = Field(..., description="Tenant identifier")
    event_id: Optional[int] = Field(None, description="Event ID (auto-generated)")
    timestamp: Optional[datetime] = Field(None, description="Event timestamp (auto-generated)")

    parent_comment_id: Optional[str] = None
    comment_length: int = Field(...)
    metadata: Optional[dict] = None
    user_id: str = Field(...)
    session_id: str = Field(...)
    article_id: str = Field(...)
    comment_id: str = Field(...)

    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat(),
            Decimal: lambda v: float(v)
        }

class NewsletterSubscribed(BaseModel):
    """Fired when a user subscribes to newsletter"""

    tenant_id: str = Field(..., description="Tenant identifier")
    event_id: Optional[int] = Field(None, description="Event ID (auto-generated)")
    timestamp: Optional[datetime] = Field(None, description="Event timestamp (auto-generated)")

    user_id: Optional[str] = None
    session_id: str = Field(...)
    email: str = Field(...)
    subscription_type: str = Field(...)
    source: Optional[str] = None
    metadata: Optional[dict] = None

    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat(),
            Decimal: lambda v: float(v)
        }

class SearchPerformed(BaseModel):
    """Fired when a user searches for content"""

    tenant_id: str = Field(..., description="Tenant identifier")
    event_id: Optional[int] = Field(None, description="Event ID (auto-generated)")
    timestamp: Optional[datetime] = Field(None, description="Event timestamp (auto-generated)")

    metadata: Optional[dict] = None
    user_id: Optional[str] = None
    session_id: str = Field(...)
    query: str = Field(...)
    results_count: int = Field(...)
    clicked_result_id: Optional[str] = None

    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat(),
            Decimal: lambda v: float(v)
        }

class CheckoutStarted(BaseModel):
    """Fired when a user begins checkout process"""

    tenant_id: str = Field(..., description="Tenant identifier")
    event_id: Optional[int] = Field(None, description="Event ID (auto-generated)")
    timestamp: Optional[datetime] = Field(None, description="Event timestamp (auto-generated)")

    cart_id: str = Field(...)
    num_items: int = Field(...)
    cart_total: Decimal = Field(...)
    currency: str = Field(...)
    items: dict = Field(...)
    metadata: Optional[dict] = None
    user_id: str = Field(...)
    session_id: str = Field(...)

    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat(),
            Decimal: lambda v: float(v)
        }

class PageView(BaseModel):
    """Fired when a user views a page"""

    tenant_id: str = Field(..., description="Tenant identifier")
    event_id: Optional[int] = Field(None, description="Event ID (auto-generated)")
    timestamp: Optional[datetime] = Field(None, description="Event timestamp (auto-generated)")

    duration_ms: Optional[int] = None  # Time spent on page (milliseconds)
    metadata: Optional[dict] = None  # Additional page metadata
    user_id: Optional[str] = None  # User ID (if logged in)
    session_id: str = Field(...)  # Session identifier
    page_url: str = Field(...)  # Full page URL
    page_title: Optional[str] = None  # Page title
    referrer: Optional[str] = None  # Referrer URL

    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat(),
            Decimal: lambda v: float(v)
        }

class ProductViewed(BaseModel):
    """Fired when a user views a product page"""

    tenant_id: str = Field(..., description="Tenant identifier")
    event_id: Optional[int] = Field(None, description="Event ID (auto-generated)")
    timestamp: Optional[datetime] = Field(None, description="Event timestamp (auto-generated)")

    session_id: str = Field(...)
    product_name: str = Field(...)
    product_category: str = Field(...)
    price: Decimal = Field(...)
    metadata: Optional[dict] = None
    user_id: str = Field(...)
    product_id: str = Field(...)
    currency: str = Field(...)
    source: Optional[str] = None

    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat(),
            Decimal: lambda v: float(v)
        }

class AddedToCart(BaseModel):
    """Fired when a user adds an item to cart"""

    tenant_id: str = Field(..., description="Tenant identifier")
    event_id: Optional[int] = Field(None, description="Event ID (auto-generated)")
    timestamp: Optional[datetime] = Field(None, description="Event timestamp (auto-generated)")

    price: Decimal = Field(...)
    metadata: Optional[dict] = None
    session_id: str = Field(...)
    product_id: str = Field(...)
    product_name: str = Field(...)
    quantity: int = Field(...)
    currency: str = Field(...)
    cart_total: Optional[Decimal] = None
    user_id: str = Field(...)

    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat(),
            Decimal: lambda v: float(v)
        }

class OrderCompleted(BaseModel):
    """Fired when an order is successfully placed"""

    tenant_id: str = Field(..., description="Tenant identifier")
    event_id: Optional[int] = Field(None, description="Event ID (auto-generated)")
    timestamp: Optional[datetime] = Field(None, description="Event timestamp (auto-generated)")

    payment_method: str = Field(...)
    num_items: int = Field(...)
    metadata: Optional[dict] = None
    order_id: str = Field(...)
    cart_id: Optional[str] = None
    currency: str = Field(...)
    shipping_address: dict = Field(...)
    items: dict = Field(...)
    discount_code: Optional[str] = None
    discount_amount: Optional[Decimal] = None
    user_id: str = Field(...)
    session_id: str = Field(...)
    total_amount: Decimal = Field(...)

    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat(),
            Decimal: lambda v: float(v)
        }

class UserSignup(BaseModel):
    """Fired when a new user signs up"""

    tenant_id: str = Field(..., description="Tenant identifier")
    event_id: Optional[int] = Field(None, description="Event ID (auto-generated)")
    timestamp: Optional[datetime] = Field(None, description="Event timestamp (auto-generated)")

    user_id: str = Field(...)  # Unique user identifier
    email: str = Field(...)  # User email address
    signup_source: Optional[str] = None  # Where the signup originated
    utm_campaign: Optional[str] = None  # Marketing campaign identifier
    metadata: Optional[dict] = None  # Additional flexible metadata

    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat(),
            Decimal: lambda v: float(v)
        }

class Purchase(BaseModel):
    """Fired when a user makes a purchase"""

    tenant_id: str = Field(..., description="Tenant identifier")
    event_id: Optional[int] = Field(None, description="Event ID (auto-generated)")
    timestamp: Optional[datetime] = Field(None, description="Event timestamp (auto-generated)")

    payment_method: Optional[str] = None  # Payment method used
    items: Optional[dict] = None  # Array of purchased items
    user_id: str = Field(...)  # User who made the purchase
    order_id: str = Field(...)  # Unique order identifier
    amount: Decimal = Field(...)  # Purchase amount
    currency: Optional[str] = None  # Currency code

    class Config:
        json_encoders = {
            datetime: lambda v: v.isoformat(),
            Decimal: lambda v: float(v)
        }

