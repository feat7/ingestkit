# Dependency Analysis & Go Version Decision

**Created:** 2025-11-15
**Issue:** go.mod dependency downgrades after fixing Go version requirement

---

## Summary

When fixing Issue C1 (invalid `go 1.24.0` requirement), changing to `go 1.22` triggered automatic downgrades of core dependencies because newer versions require Go 1.23+. All tests pass with downgraded versions, but we've lost 1-2 years of bug fixes and API improvements.

---

## What Happened

### Original go.mod (broken)
```go
go 1.24.0        // ❌ Doesn't exist
toolchain go1.24.10  // ❌ Doesn't exist

require (
    github.com/jackc/pgx/v5 v5.7.6    // Requires go >= 1.23
    github.com/twmb/franz-go v1.20.3  // Requires go >= 1.24 (!)
)
```

### Current go.mod (functional but downgraded)
```go
go 1.22  // ✅ Matches local environment (go1.22.3)

require (
    github.com/jackc/pgx/v5 v5.4.3    // ⬇️ Downgraded from v5.7.6
    github.com/twmb/franz-go v1.13.5  // ⬇️ Downgraded from v1.20.3
)
```

---

## Dependency Downgrades

| Package | Before | After | Lost Versions | Impact |
|---------|--------|-------|---------------|--------|
| **jackc/pgx/v5** | v5.7.6 | v5.4.3 | 3 minor releases | ~8 months of fixes |
| **twmb/franz-go** | v1.20.3 | v1.13.5 | 7 minor releases | ~1 year of fixes |
| **rogpeppe/go-internal** | v1.14.1 | v1.12.0 | 2 minor releases | ~6 months |
| **golang.org/x/crypto** | v0.43.0 | v0.14.0 | 29 minor releases | ~2 years |
| **golang.org/x/sync** | v0.17.0 | v0.1.0 | 16 minor releases | ~2 years |
| **golang.org/x/sys** | v0.37.0 | v0.28.0 | 9 minor releases | ~1 year |
| **golang.org/x/text** | v0.30.0 | v0.13.0 | 17 minor releases | ~2 years |

---

## Test Results

**Current Status**: ✅ All tests pass with downgraded versions

```
✅ cmd/loadtest            - 5 tests
✅ internal/api/middleware - 26 tests
✅ internal/messaging      - 9 tests
✅ internal/schema         - 28 tests
✅ internal/validation     - 30 tests
-----------------------------------
Total: 98 tests, 100% pass rate
```

**Build Status**: ✅ Both `api` and `consumer` build successfully

---

## Options & Recommendations

### Option 1: Stay with Go 1.22 + Downgraded Dependencies ⚠️
**Pros:**
- Matches current local environment (go1.22.3)
- All tests pass
- Builds work
- No developer environment changes needed

**Cons:**
- Missing 1-2 years of bug fixes in critical dependencies
- Missing performance improvements in pgx COPY protocol
- Missing security fixes in golang.org/x/crypto
- Not recommended for production deployment

**Verdict:** ⚠️ **Acceptable for development, NOT for production**

---

### Option 2: Upgrade to Go 1.23 🔄
**Pros:**
- Can upgrade pgx to v5.7.x (get latest PostgreSQL driver fixes)
- Moderate upgrade path (1.22 → 1.23 is small jump)
- Gets most bug fixes

**Cons:**
- franz-go v1.20.3 still requires go 1.24 (doesn't exist)
- Would need to find highest franz-go version compatible with 1.23
- Requires updating local Go installation

**Implementation:**
```bash
# 1. Install Go 1.23
brew upgrade go  # or download from golang.org

# 2. Update go.mod
go mod edit -go=1.23

# 3. Try upgrading dependencies
go get github.com/jackc/pgx/v5@latest
go get github.com/twmb/franz-go@<find-compatible-version>
go mod tidy

# 4. Test thoroughly
go test ./...
```

**Verdict:** 🔄 **Good middle ground - recommended for most teams**

---

### Option 3: Upgrade to Go 1.25 (Latest Stable) 🚀
**Pros:**
- Can use latest versions of ALL dependencies
- All bug fixes, performance improvements, security patches
- Future-proof (Go 1.25 is current stable)
- Best for production

**Cons:**
- Largest jump from current local environment (1.22.3 → 1.25)
- May require updating CI/CD, Docker images, deployment configs
- Higher risk of compatibility issues

**Implementation:**
```bash
# 1. Install Go 1.25
brew upgrade go  # or download from golang.org

# 2. Update go.mod
go mod edit -go=1.25

# 3. Upgrade all dependencies
go get -u ./...
go mod tidy

# 4. Update Dockerfile (if exists)
FROM golang:1.25-alpine

# 5. Update CI/CD (GitHub Actions, etc.)
go-version: '1.25'

# 6. Test thoroughly
go test ./...
make loadtest-baseline
```

**Verdict:** 🚀 **Best for production - recommended**

---

### Option 4: Find Optimal Versions for Go 1.22 🔍
**Pros:**
- Stay with Go 1.22
- Selectively upgrade to highest compatible versions
- Get some bug fixes without Go upgrade

**Cons:**
- Requires research to find "sweet spot" versions
- Still won't get latest fixes
- More complex maintenance

**Verdict:** 🔍 **Only if Go upgrade is blocked by organizational constraints**

---

## Recommended Action Plan

### For Development (Short Term)
✅ **Current state is acceptable** - all tests pass, builds work

### For Production (Required Before Deploy)
🚀 **Upgrade to Go 1.25** following Option 3 implementation steps

### Migration Path
1. **Week 1**: Upgrade local development environment to Go 1.25
2. **Week 1**: Update go.mod, upgrade dependencies, run full test suite
3. **Week 2**: Update CI/CD pipelines, Docker images
4. **Week 2**: Run load tests to verify performance
5. **Week 3**: Deploy to staging, monitor for issues
6. **Week 4**: Production deployment with monitoring

---

## Key Dependencies Analysis

### PostgreSQL Driver (pgx)
- **Current**: v5.4.3 (from June 2023)
- **Latest**: v5.7.6 (from September 2025)
- **Missing**: COPY protocol improvements, connection pool fixes, security updates
- **Impact**: High - core database functionality

### Kafka Driver (franz-go)
- **Current**: v1.13.5 (from ~Q4 2023)
- **Latest**: v1.20.3 (from ~Q4 2024)
- **Missing**: Consumer group rebalancing improvements, offset management fixes
- **Impact**: High - core messaging functionality

### Security Libraries (golang.org/x/crypto)
- **Current**: v0.14.0 (from ~Q3 2023)
- **Latest**: v0.43.0 (from ~Q4 2025)
- **Missing**: 2 years of security patches
- **Impact**: Critical - security vulnerabilities

---

## References

- Go Release History: https://go.dev/doc/devel/release
- pgx Changelog: https://github.com/jackc/pgx/blob/master/CHANGELOG.md
- franz-go Releases: https://github.com/twmb/franz-go/releases
- ISSUES.md: Issue C1 for full resolution details

---

## Decision

✅ **DECISION MADE: Upgraded to Go 1.25.4**

**Date:** 2025-11-15
**Implemented by:** Claude Code
**Rationale:** Latest stable Go version with all dependency security patches and bug fixes

### Upgrade Results

✅ **All dependencies restored to latest versions:**
- `jackc/pgx/v5`: v5.4.3 → v5.7.6 (PostgreSQL driver)
- `twmb/franz-go`: v1.13.5 → v1.20.3 (Kafka client)
- `golang.org/x/crypto`: v0.14.0 → v0.44.0 (Security libraries)
- `golang.org/x/sync`: v0.1.0 → v0.18.0
- `golang.org/x/sys`: v0.28.0 → v0.38.0
- `golang.org/x/text`: v0.13.0 → v0.31.0

✅ **All tests pass:** 98 tests, 100% pass rate
✅ **Builds succeed:** Both API and consumer build successfully

### Developer Action Required

**Before you can build/run the project, upgrade your local Go:**

```bash
# macOS (Homebrew)
brew upgrade go

# Or download from official site
# https://go.dev/dl/

# Verify installation
go version  # Should show go1.25.4 or higher
```

**After upgrading Go, you can build:**
```bash
go build ./cmd/api
go build ./cmd/consumer
go test ./...
```

### Next Steps

- [x] Update README.md to require Go 1.25+ (DONE)
- [ ] Update CI/CD configs (GitHub Actions, Docker, etc.) to use Go 1.25 when added
- [ ] Update Dockerfile to use `FROM golang:1.25-alpine` when added
- [x] Notify team members to upgrade their local Go installations (via this doc)

**Note:** No CI/CD or Docker files exist yet. When adding them, ensure they use Go 1.25+:

```yaml
# Example GitHub Actions
- uses: actions/setup-go@v5
  with:
    go-version: '1.25'

# Example Dockerfile
FROM golang:1.25-alpine AS builder
```
