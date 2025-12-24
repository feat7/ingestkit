# IngestKit Documentation

Detailed guides for development, deployment, and operations.

## Installation

```bash
# Python
pip install ingestkit

# Node.js
npm install -g ingestkit

# Or download from GitHub Releases
```

## Guides

### Getting Started
- **[Development](development.md)** - Development workflows, adding events, schema changes
- **[SDK Generation](sdk-generation.md)** - Python and TypeScript SDK architecture

### Database & Migrations
- **[Automated Migrations](automated-migrations.md)** - Prisma-style migration generation with Atlas
- **[Manual Migrations](migrations.md)** - Manual migration workflow and best practices

### Deployment & Operations
- **[Docker Deployment](docker-deployment.md)** - Zero-downtime container deployments
- **[Load Testing](load-testing.md)** - Performance testing with k6

## Quick Reference

For a concise overview of commands and architecture, see [CLAUDE.md](../CLAUDE.md).

## Contributing

When updating documentation:
1. Keep CLAUDE.md concise - only essentials for quick reference
2. Add detailed guides to this `/docs` folder
3. Update this README when adding new docs
4. Include clear examples and code snippets
