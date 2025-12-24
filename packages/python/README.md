# IngestKit Python Package

High-performance event ingestion with type-safe Python SDKs.

## Installation

```bash
pip install ingestkit
```

This will automatically download the IngestKit CLI binary for your platform.

## Quick Start

```bash
# 1. Initialize in your project
cd my-django-app
ingestkit init

# 2. Define events in ingestkit/schema.yaml
# (Already has a user_signup example!)

# 3. Generate type-safe client
ingestkit generate

# 4. Use in your code
```

```python
from ingestkit import Client

client = Client()  # Reads config automatically

# Send events with full type safety
client.user_signup.send(
    user_id="123",
    email="user@example.com",
    signup_source="web"
)
```

## What You Get

- **Type-safe client** - Auto-generated from your schema
- **Pydantic validation** - Catch errors before sending
- **Zero configuration** - Just `ingestkit init` and go
- **Fast binary** - Go-powered CLI, no overhead

## Project Structure

After `ingestkit init`:

```
my-app/
├── ingestkit/
│   ├── schema.yaml          # YOU EDIT - Define events
│   ├── client.py           # GENERATED - Type-safe client
│   ├── models.py           # GENERATED - Pydantic models
│   └── __init__.py         # GENERATED - Exports
└── ingestkit.config.json   # Configuration
```

## Requirements

- Python 3.7+
- `requests` and `pydantic` (auto-installed)
- Internet connection for initial binary download

## Examples

### Flask App

```python
from flask import Flask, request
from ingestkit import Client

app = Flask(__name__)
events = Client()

@app.route('/signup', methods=['POST'])
def signup():
    data = request.json

    # Track signup with IngestKit
    events.user_signup.send(
        user_id=data['user_id'],
        email=data['email'],
        signup_source='api'
    )

    return {'success': True}
```

### Django View

```python
from django.views.decorators.http import require_POST
from ingestkit import Client

events = Client()

@require_POST
def checkout(request):
    order = process_order(request.POST)

    # Track purchase
    events.purchase.send(
        user_id=request.user.id,
        order_id=order.id,
        amount=float(order.total),
        payment_method=request.POST['method']
    )

    return HttpResponse('OK')
```

## Commands

```bash
# Initialize project
ingestkit init [--python|--typescript|--go]

# Generate client from schema
ingestkit generate

# Show help
ingestkit --help
```

## Configuration

`ingestkit.config.json`:

```json
{
  "apiUrl": "http://localhost:8080",
  "apiKey": "${INGESTKIT_API_KEY}",
  "tenantId": "my-app",
  "generator": {
    "language": "python",
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

Visit https://github.com/feat7/ingestkit/releases/latest and manually download the binary for your platform.

### "Client not generated yet" error

Run `ingestkit generate` in your project directory first.

### Import errors

Make sure you're importing from your project's `ingestkit` directory:
```python
from ingestkit import Client  # ✅ Correct (generated)
from ingestkit.client import IngestKitClient  # ❌ Wrong
```

## License

Apache 2.0

## Support

- GitHub Issues: https://github.com/feat7/ingestkit/issues
- Documentation: https://github.com/feat7/ingestkit
