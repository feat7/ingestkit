#!/usr/bin/env python3
"""
IngestKit Python SDK - Quick Start Example

This example shows how to:
1. Initialize the IngestKit client
2. Send single events
3. Send batch events
4. Handle errors

Prerequisites:
- pip install requests pydantic
- IngestKit running on http://localhost:8080
"""

import sys
import os

# Add generated SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'generated', 'sdk'))

from python.client import IngestKitClient
from python.models import UserSignup, Purchase, PageView

def main():
    print("🚀 IngestKit Python SDK - Quick Start Example\n")

    # Initialize client
    client = IngestKitClient(
        api_url="http://localhost:8080",
        api_key="dev_key_1234567890"  # Default dev key from .env
    )
    print("✓ Client initialized\n")

    # Example 1: Send a user signup event
    print("📝 Example 1: Send user signup event")
    signup = UserSignup(
        tenant_id="quickstart-demo",
        user_id="alice_123",
        email="alice@example.com",
        signup_source="web",
        utm_campaign="demo_2024"
    )

    try:
        response = client.send_user_signup(signup)
        print(f"   ✓ User signup tracked: {response}\n")
    except Exception as e:
        print(f"   ✗ Error: {e}\n")
        return

    # Example 2: Send a purchase event
    print("💳 Example 2: Send purchase event")
    purchase = Purchase(
        tenant_id="quickstart-demo",
        user_id="alice_123",
        order_id="order_456",
        amount=149.99,
        currency="USD",
        payment_method="stripe",
        items={"products": [{"sku": "BOOK-001", "qty": 2}]}
    )

    try:
        response = client.send_purchase(purchase)
        print(f"   ✓ Purchase tracked: {response}\n")
    except Exception as e:
        print(f"   ✗ Error: {e}\n")
        return

    # Example 3: Send a page view event
    print("👁️  Example 3: Send page view event")
    page_view = PageView(
        tenant_id="quickstart-demo",
        session_id="session_789",
        page_url="https://example.com/products",
        page_title="Products - Example Shop",
        referrer="https://google.com",
        duration_ms=15000
    )

    try:
        response = client.send_page_view(page_view)
        print(f"   ✓ Page view tracked: {response}\n")
    except Exception as e:
        print(f"   ✗ Error: {e}\n")
        return

    # Example 4: Send batch of events
    print("📦 Example 4: Send batch of signups")
    batch_signups = [
        UserSignup(
            tenant_id="quickstart-demo",
            user_id=f"user_{i}",
            email=f"user{i}@example.com",
            signup_source="api"
        )
        for i in range(10)
    ]

    try:
        response = client.send_user_signup_batch(batch_signups)
        print(f"   ✓ Batch tracked: {response}\n")
    except Exception as e:
        print(f"   ✗ Error: {e}\n")
        return

    # Success!
    print("=" * 50)
    print("✅ All events sent successfully!")
    print("\nNext steps:")
    print("1. Query your data: make db-connect")
    print("2. View consumer metrics: make metrics")
    print("3. Check this query:")
    print("\n   SELECT * FROM events_user_signup")
    print("   WHERE tenant_id = 'quickstart-demo'")
    print("   ORDER BY timestamp DESC LIMIT 10;")
    print("=" * 50)

if __name__ == "__main__":
    main()
