# IngestKit npm Package

High-performance event ingestion with type-safe TypeScript SDKs.

## Installation

```bash
npm install -D ingestkit
# or
yarn add -D ingestkit
```

This will automatically download the IngestKit CLI binary for your platform.

## Quick Start

```bash
# 1. Initialize in your project
npx ingestkit init

# 2. Define events in ingestkit/schema.yaml
# (Already has a user_signup example!)

# 3. Generate type-safe client
npx ingestkit generate

# 4. Use in your code
```

```typescript
import { Client } from './ingestkit'

const client = new Client()  // Reads config automatically

// Send events with full type safety
await client.userSignup.send({
    userId: "123",
    email: "user@example.com",
    signupSource: "web"
})
```

## What You Get

✅ **Type-safe client** - Auto-generated from your schema
✅ **TypeScript interfaces** - Full IDE autocomplete
✅ **Zero configuration** - Just `npx ingestkit init` and go
✅ **Fast binary** - Go-powered CLI, no overhead

## Project Structure

After `npx ingestkit init`:

```
my-app/
├── ingestkit/
│   ├── schema.yaml          # YOU EDIT - Define events
│   ├── client.ts           # GENERATED - Type-safe client
│   ├── models.ts           # GENERATED - TypeScript interfaces
│   └── index.ts            # GENERATED - Exports
└── ingestkit.config.json   # Configuration
```

## Requirements

- Node.js 14+
- Internet connection for initial binary download

## Examples

### Express App

```typescript
import express from 'express'
import { Client } from './ingestkit'

const app = express()
const events = new Client()

app.post('/api/signup', async (req, res) => {
    const { userId, email } = req.body

    // Track signup with IngestKit
    await events.userSignup.send({
        userId,
        email,
        signupSource: 'api'
    })

    res.json({ success: true })
})

app.listen(3000)
```

### Next.js API Route

```typescript
import type { NextApiRequest, NextApiResponse } from 'next'
import { Client } from '../../../ingestkit'

const events = new Client()

export default async function handler(
    req: NextApiRequest,
    res: NextApiResponse
) {
    if (req.method === 'POST') {
        // Track purchase
        await events.purchase.send({
            userId: req.body.userId,
            orderId: req.body.orderId,
            amount: req.body.amount,
            paymentMethod: req.body.paymentMethod
        })

        res.status(200).json({ success: true })
    }
}
```

## Commands

```bash
# Initialize project
npx ingestkit init [--python|--typescript|--go]

# Generate client from schema
npx ingestkit generate

# Show help
npx ingestkit --help
```

## Configuration

`ingestkit.config.json`:

```json
{
  "apiUrl": "http://localhost:8080",
  "apiKey": "${INGESTKIT_API_KEY}",
  "tenantId": "my-app",
  "generator": {
    "language": "typescript",
    "output": "./ingestkit"
  }
}
```

Set `INGESTKIT_API_KEY` environment variable or hardcode for development.

## Documentation

- **Quick Start**: See above
- **Full Guide**: https://github.com/feat7/ingestkit/blob/main/PRISMA_STYLE_GUIDE.md
- **Server Setup**: https://github.com/feat7/ingestkit

## Troubleshooting

### Binary download fails

Visit https://github.com/feat7/ingestkit/releases/latest and manually download the binary for your platform, then place it in `node_modules/ingestkit/bin/`.

### "Client not generated yet" error

Run `npx ingestkit generate` in your project directory first.

### Import errors

Make sure you're importing from your project's `ingestkit` directory:
```typescript
import { Client } from './ingestkit'  // ✅ Correct (generated)
import { Client } from 'ingestkit'     // ❌ Wrong (npm package)
```

## License

MIT

## Support

- GitHub Issues: https://github.com/feat7/ingestkit/issues
- Documentation: https://github.com/feat7/ingestkit
