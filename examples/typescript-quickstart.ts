#!/usr/bin/env ts-node
/**
 * IngestKit TypeScript SDK - Quick Start Example
 *
 * This example shows how to:
 * 1. Initialize the IngestKit client
 * 2. Send single events
 * 3. Send batch events
 * 4. Handle errors
 *
 * Prerequisites:
 * - npm install (or copy generated SDK to your project)
 * - IngestKit running on http://localhost:8080
 *
 * Run: npx ts-node examples/typescript-quickstart.ts
 */

import * as path from 'path';
import * as fs from 'fs';

// Import generated SDK
// In a real app, you'd copy these files to your src/ directory
const sdkPath = path.join(__dirname, '..', 'generated', 'sdk', 'typescript');

// Dynamic import (in real app, use: import { IngestKitClient, Models } from './sdk')
import type { IngestKitClient as Client } from '../generated/sdk/typescript/client';
import type * as Models from '../generated/sdk/typescript/models';

async function main() {
    console.log("🚀 IngestKit TypeScript SDK - Quick Start Example\n");

    // Import SDK dynamically
    const { IngestKitClient } = await import(path.join(sdkPath, 'client'));

    // Initialize client
    const client = new IngestKitClient({
        apiUrl: "http://localhost:8080",
        apiKey: "dev_key_1234567890"  // Default dev key from .env
    });
    console.log("✓ Client initialized\n");

    // Example 1: Send a user signup event
    console.log("📝 Example 1: Send user signup event");
    const signup: Models.UserSignup = {
        tenant_id: "quickstart-demo",
        user_id: "alice_123",
        email: "alice@example.com",
        signup_source: "web",
        utm_campaign: "demo_2024"
    };

    try {
        const response = await client.sendUserSignup(signup);
        console.log(`   ✓ User signup tracked:`, response, "\n");
    } catch (error: any) {
        console.error(`   ✗ Error: ${error.message}\n`);
        return;
    }

    // Example 2: Send a purchase event
    console.log("💳 Example 2: Send purchase event");
    const purchase: Models.Purchase = {
        tenant_id: "quickstart-demo",
        user_id: "alice_123",
        order_id: "order_456",
        amount: 149.99,
        currency: "USD",
        payment_method: "stripe",
        items: { products: [{ sku: "BOOK-001", qty: 2 }] }
    };

    try {
        const response = await client.sendPurchase(purchase);
        console.log(`   ✓ Purchase tracked:`, response, "\n");
    } catch (error: any) {
        console.error(`   ✗ Error: ${error.message}\n`);
        return;
    }

    // Example 3: Send a page view event
    console.log("👁️  Example 3: Send page view event");
    const pageView: Models.PageView = {
        tenant_id: "quickstart-demo",
        session_id: "session_789",
        page_url: "https://example.com/products",
        page_title: "Products - Example Shop",
        referrer: "https://google.com",
        duration_ms: 15000
    };

    try {
        const response = await client.sendPageView(pageView);
        console.log(`   ✓ Page view tracked:`, response, "\n");
    } catch (error: any) {
        console.error(`   ✗ Error: ${error.message}\n`);
        return;
    }

    // Example 4: Send batch of events
    console.log("📦 Example 4: Send batch of signups");
    const batchSignups: Models.UserSignup[] = Array.from({ length: 10 }, (_, i) => ({
        tenant_id: "quickstart-demo",
        user_id: `user_${i}`,
        email: `user${i}@example.com`,
        signup_source: "api"
    }));

    try {
        const response = await client.sendUserSignupBatch(batchSignups);
        console.log(`   ✓ Batch tracked:`, response, "\n");
    } catch (error: any) {
        console.error(`   ✗ Error: ${error.message}\n`);
        return;
    }

    // Success!
    console.log("=".repeat(50));
    console.log("✅ All events sent successfully!");
    console.log("\nNext steps:");
    console.log("1. Query your data: make db-connect");
    console.log("2. View consumer metrics: make metrics");
    console.log("3. Check this query:");
    console.log("\n   SELECT * FROM events_user_signup");
    console.log("   WHERE tenant_id = 'quickstart-demo'");
    console.log("   ORDER BY timestamp DESC LIMIT 10;");
    console.log("=".repeat(50));
}

main().catch(console.error);
