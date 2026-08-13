# TrustGraph Developer Guide (Phase 1)

## Overview

TrustGraph is a Go trust and safety service for ConnectionSphere. Phase 1 implements Plane A (first-party signals): assessment endpoint, signal providers, policy engine, and audit logging.

## Project Structure

```
trustgraph/
├── cmd/trustgraph-api/
│   └── main.go                    # Entry point
├── internal/
│   ├── api/
│   │   ├── router.go              # HTTP router + middleware
│   │   └── assessment_handler.go  # POST/GET /v1/assessments
│   ├── audit/
│   │   ├── events.go              # Audit event types + constants
│   │   └── logger.go              # Database-backed audit logger
│   ├── config/
│   │   └── config.go              # Environment config (envconfig)
│   ├── models/
│   │   └── assessment.go          # Request/response types, constants
│   ├── policy/
│   │   ├── engine.go              # Weighted-score policy engine
│   │   ├── rules.go               # Hard-block rules
│   │   └── version.go             # Policy versioning
│   ├── signals/
│   │   ├── provider.go            # Provider interface + types
│   │   ├── email.go               # Email verification + disposable detection
│   │   ├── phone.go               # Phone verification
│   │   ├── device.go              # Device fingerprint sharing
│   │   ├── velocity.go            # Registration velocity
│   │   ├── image.go               # Image hash reuse
│   │   └── evaluator.go           # Runs all providers
│   └── store/
│       ├── postgres.go            # Connection pool
│       ├── migrations.go          # Migration runner
│       ├── assessment_repo.go     # Assessment CRUD
│       ├── subject_repo.go        # Subject upsert + queries
│       └── observation_repo.go    # Observation recording + lookups
├── migrations/
│   └── 001_init_schema.sql        # All Phase 1 tables
├── api/
│   └── trustgraph.openapi.yaml    # OpenAPI 3.1 spec
├── docs/
│   └── COLDFUSION_INTEGRATION.md  # ConnectionSphere integration guide
├── go.mod, go.sum
├── Dockerfile                     # Multi-stage Go build
├── docker-compose.yml             # Uses ConnectionSphere's Postgres
├── render.yaml                    # Render deployment blueprint
├── Makefile
└── .env.example
```

## Quick Start

### Prerequisites

- Docker (ConnectionSphere stack must be running for shared Postgres)
- Go 1.23+

### Shared Infrastructure

TrustGraph reuses ConnectionSphere's PostgreSQL 17 container (`connectsphere-postgres`). It creates a separate `trustgraph` database inside the same instance.

```bash
# 1. Start ConnectionSphere's Postgres first
cd ~/Dev/Projects/ConnectionSphere
docker compose up -d postgres

# 2. Back in TrustGraph, create the database and start
cd ~/Dev/Projects/TrustGraph
make db-init   # Creates trustgraph database in ConnectionSphere's Postgres
make up        # Starts TrustGraph API on port 8081
```

### Test the API

```bash
curl -X POST http://localhost:8081/v1/assessments \
  -H "Content-Type: application/json" \
  -d '{
    "contractVersion": "2026-08-01",
    "idempotencyKey": "reg:user-123:v1",
    "subject": {
      "connectionSphereUserId": "user-123",
      "email": "user@example.com"
    },
    "signals": {
      "emailVerified": true,
      "phoneVerified": false,
      "ipAddress": "192.168.1.100"
    }
  }'
```

### Local Development (no Docker for the API)

```bash
# ConnectionSphere Postgres must be running on port 5432
make deps
make dev    # Runs go run ./cmd/trustgraph-api
```

## Architecture

### Assessment Flow

```
ConnectionSphere (ColdFusion)
  │
  │ POST /v1/assessments (300ms timeout, fail-open)
  ▼
TrustGraph API
  ├── Idempotency check (cached? return immediately)
  ├── Find/create subject
  ├── Run signal providers (email, phone, device, velocity, image)
  ├── Record observations (for future velocity/device/image lookups)
  ├── Policy engine (weighted score + hard-block rules → tier + decision)
  ├── Persist assessment
  ├── Audit log
  └── Return: {trustTier, riskScore, riskBand, decision, reasonCodes}
```

### Signal Providers

| Provider | What it checks | Score range | Key reason codes |
|----------|---------------|-------------|-----------------|
| Email | Verification + disposable domain | 0-40 | EMAIL_VERIFIED, DISPOSABLE_EMAIL |
| Phone | Verification status | 0-10 | PHONE_VERIFIED, PHONE_NOT_VERIFIED |
| Device | Fingerprint sharing with other accounts | 0-30 | DEVICE_FIRST_SEEN, DEVICE_SHARED_WITH_ENFORCED |
| Velocity | Registrations from same IP in 1 hour | 0-35 | HIGH_REGISTRATION_VELOCITY |
| Image | Profile image hash reuse | 0-20 | IMAGE_HASH_REUSED |

### Policy Engine

1. Compute weighted risk score: `sum(score * confidence) / sum(confidence)`
2. Check hard-block rules (first match wins):
   - DISPOSABLE_EMAIL + HIGH_VELOCITY → deny
   - DEVICE_SHARED_WITH_ENFORCED → review
   - IMAGE_REUSED + HIGH_VELOCITY → review
3. Map score to tier: 0-20 standard, 21-40 provisional/low, 41-60 provisional/elevated, 61-80 provisional/high, 81+ limited
4. Attach required actions (VERIFY_EMAIL, VERIFY_PHONE, REVIEW_BY_HUMAN)

## Configuration

```bash
TRUSTGRAPH_PORT=8080                           # HTTP port
TRUSTGRAPH_ENVIRONMENT=development             # development | production
TRUSTGRAPH_DATABASE_URL=postgres://...         # ConnectionSphere Postgres → trustgraph db
TRUSTGRAPH_ASSESSMENT_TIMEOUT_MS=300           # Assessment budget (ms)
TRUSTGRAPH_CIRCUIT_BREAKER_MAX_FAILURES=5      # Circuit breaker threshold
TRUSTGRAPH_CIRCUIT_BREAKER_WINDOW_MINS=5       # Circuit breaker window
TRUSTGRAPH_LOG_LEVEL=debug                     # zap log level
```

## Testing

```bash
make test                          # All tests
go test -v ./internal/policy/...   # Policy engine only
go test -v ./internal/signals/...  # Signal providers only
go test -cover ./...               # Coverage report
```

## Database

```bash
# Connect to the shared Postgres
docker exec -it connectsphere-postgres psql -U connectsphere -d trustgraph

# Useful queries
SELECT * FROM assessment ORDER BY created_at DESC LIMIT 5;
SELECT * FROM audit_log ORDER BY created_at DESC LIMIT 10;
SELECT * FROM observation WHERE observation_type = 'registration' ORDER BY created_at DESC;
SELECT * FROM schema_migrations;
```

## Deployment

### Render (Production)

The `render.yaml` blueprint provisions:
- `trustgraph-api` — web service (Docker, starter plan)
- `trustgraph-db` — PostgreSQL 17 (starter plan, Oregon)

```bash
# Deploy via Render dashboard or CLI
render blueprint apply
```

### Docker (Local)

```bash
make build   # Build image
make up      # Start (joins ConnectionSphere network)
make logs    # Stream logs
make down    # Stop
```

## Adding a Signal Provider

1. Create `internal/signals/my_signal.go` implementing the `Provider` interface
2. Register it in `evaluator.go` → `NewEvaluator()`
3. Add reason code constants to `internal/models/assessment.go`
4. Add hard-block rules to `internal/policy/rules.go` if needed
5. Write tests in `internal/signals/my_signal_test.go`

## References

- [PHASE_1_IMPLEMENTATION.md](./PHASE_1_IMPLEMENTATION.md) — Full technical design
- [OpenAPI spec](./api/trustgraph.openapi.yaml) — API contract
- [ColdFusion Integration](./docs/COLDFUSION_INTEGRATION.md) — ConnectionSphere caller guide

## Security Guidelines

### Before Contributing

1. **Never commit secrets.** All credentials (passwords, tokens, API keys) must come from environment variables or `.env` files (which are gitignored).
2. **Use `.env.example` as a template only.** All values should be placeholders like `your_password`, not real credentials.
3. **Always use parameterized queries.** Never concatenate user input into SQL.
4. **Validate all inputs.** All external data (API requests, form inputs) must be validated before processing.
5. **Log carefully.** Never log sensitive data (passwords, tokens, PII). Use audit logging for compliance.

### Testing Security Changes

```bash
# Check for hardcoded credentials (should return nothing)
grep -r "password\|secret\|token" --include="*.go" . | grep -v "Password string\|TOKEN\|Token"

# Run tests with race detector
make test-race

# Check dependency vulnerabilities
go mod audit
```

### Database Security

- All database connections use parameterized queries via `lib/pq`.
- Connection pooling prevents resource exhaustion.
- Audit logging captures all data access.
- Production deployments must use `sslmode=require` for PostgreSQL.

### Audit Logging

Every assessment, consent change, and investigator action is immutably logged:

```go
audit.LogEvent(ctx, &audit.Event{
  Plane: "A",
  Action: "assessment_created",
  Actor: "service:trustgraph",
  ResourceType: "assessment",
  ResourceID: assessmentID,
  SubjectID: subjectID,
  Result: "success",
})
```

See [`internal/audit/events.go`](./internal/audit/events.go) for all event types.

### Reporting Security Issues

Do **not** open a public GitHub issue for security vulnerabilities. Instead, email security details to the maintainers. See [SECURITY.md](./SECURITY.md) for the full disclosure policy.
