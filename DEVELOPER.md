# TrustGraph Developer Guide (Phase 1)

## Overview

TrustGraph is a Go-based trust and safety service for ConnectionSphere. Phase 1 implements the core assessment endpoint with basic first-party signal evaluation.

## Project Structure

```
trustgraph/
├── cmd/
│   └── trustgraph-api/
│       └── main.go              # Entry point
├── internal/
│   ├── api/
│   │   ├── router.go            # HTTP router and middleware
│   │   └── assessment_handler.go # Assessment endpoint handlers
│   ├── config/
│   │   └── config.go            # Configuration management
│   ├── models/
│   │   └── assessment.go        # Data models and constants
│   ├── store/
│   │   ├── postgres.go          # Database connection
│   │   ├── migrations.go        # Migration runner
│   │   └── assessment_repo.go   # Assessment repository (queries)
│   └── policy/
│       └── [future: policy engine]
├── migrations/
│   └── 001_init_schema.sql      # Initial database schema
├── go.mod, go.sum              # Go module dependencies
├── Dockerfile                  # Docker image build
├── docker-compose.yml          # Local development environment
├── Makefile                    # Convenience commands
├── .env.example                # Environment variable template
└── DEVELOPER.md                # This file
```

## Quick Start

### Prerequisites

- Docker and Docker Compose (for containerized development)
- Or: Go 1.23+ and PostgreSQL 17+ (for local development)

### Option 1: Docker (Recommended)

```bash
# Start services
make up

# Check logs
make logs

# Test the API
curl -X POST http://localhost:8080/v1/assessments \
  -H "Content-Type: application/json" \
  -d '{
    "contractVersion": "2026-08-01",
    "idempotencyKey": "registration:user-123:v1",
    "subject": {
      "connectionSphereUserId": "user-123"
    },
    "signals": {
      "emailVerified": true,
      "phoneVerified": false
    }
  }'

# Stop services
make down
```

### Option 2: Local Development

```bash
# Install dependencies
go mod download

# Start PostgreSQL (requires local installation or Docker)
# If using Docker just for Postgres:
docker run -d \
  --name trustgraph-postgres \
  -e POSTGRES_USER=trustgraph \
  -e POSTGRES_PASSWORD=trustgraph_dev_password \
  -e POSTGRES_DB=trustgraph \
  -p 5432:5432 \
  postgres:17-alpine

# Run migrations and start server
make dev

# Server runs on http://localhost:8080
```

## API Usage (Phase 1)

### Create Assessment

**Endpoint:** `POST /v1/assessments`

**Request:**
```json
{
  "contractVersion": "2026-08-01",
  "idempotencyKey": "registration:user-123:v1",
  "subject": {
    "connectionSphereUserId": "user-123",
    "email": "user@example.com",
    "phone": "+1-555-0123"
  },
  "signals": {
    "emailVerified": true,
    "phoneVerified": false,
    "deviceToken": "device-abc-123",
    "imageHash": "hash-xyz",
    "deviceFingerprint": "fingerprint-data",
    "ipAddress": "192.0.2.1"
  }
}
```

**Response:**
```json
{
  "contractVersion": "2026-08-01",
  "assessmentId": "550e8400-e29b-41d4-a716-446655440000",
  "status": "complete",
  "trustTier": "provisional",
  "requiredActions": ["VERIFY_EMAIL", "VERIFY_PHONE"],
  "riskBand": "unknown",
  "riskScore": 50,
  "reasonCodes": ["EMAIL_VERIFIED", "PHONE_NOT_VERIFIED", "DEVICE_FIRST_SEEN"],
  "policyVersion": "registration-v1",
  "completedAt": "2026-08-03T12:34:56Z"
}
```

### Get Assessment

**Endpoint:** `GET /v1/assessments/{assessmentId}`

**Response:** Same as create assessment response

### Health Check

**Endpoint:** `GET /health`

**Response:**
```json
{
  "status": "ok"
}
```

## Development Workflow

### Adding a Signal Provider (Phase 2)

1. Create new file in `internal/signals/`: `internal/signals/my_signal.go`
2. Implement signal evaluation function:
   ```go
   func EvaluateMySignal(ctx context.Context, signals models.SignalsData) ([]string, error) {
       // Evaluate signal
       // Return reason codes
   }
   ```
3. Call from `assessment_handler.go` in `evaluateSignals()`
4. Update `decideOutcome()` if new signal affects tier decision
5. Write unit tests in `internal/signals/my_signal_test.go`

### Adding an API Endpoint

1. Create handler method in `assessment_handler.go` or new file
2. Register route in `internal/api/router.go`
3. Add OpenAPI definition to docs
4. Test via curl or Postman

### Database Changes

1. Create migration file: `migrations/NNN_description.sql`
2. Number sequentially (001, 002, etc.)
3. Run migrations automatically on startup
4. Update models in `internal/models/` as needed

## Configuration

Environment variables (see `.env.example`):

```bash
TRUSTGRAPH_PORT=8080                           # HTTP port
TRUSTGRAPH_ENVIRONMENT=development             # development or production
TRUSTGRAPH_DATABASE_URL=postgres://...         # PostgreSQL connection string
TRUSTGRAPH_ASSESSMENT_TIMEOUT_MS=300           # Assessment budget (ms)
TRUSTGRAPH_CIRCUIT_BREAKER_MAX_FAILURES=5      # Circuit breaker threshold
TRUSTGRAPH_CIRCUIT_BREAKER_WINDOW_MINS=5       # Circuit breaker window
TRUSTGRAPH_LOG_LEVEL=debug                     # Logger level
```

## Testing

### Unit Tests

```bash
# Run all tests
make test

# Run specific package
go test -v ./internal/models/...

# Run with coverage
go test -cover ./...
```

### Integration Tests (Phase 1.5)

Create a `*_integration_test.go` file:

```go
func TestAssessmentFlow(t *testing.T) {
    // Requires test database (docker-compose test?)
    // Test full request → response cycle
}
```

### Manual Testing

Use curl or Postman:

```bash
# Create assessment
curl -X POST http://localhost:8080/v1/assessments \
  -H "Content-Type: application/json" \
  -d @test-assessment.json

# Get assessment (copy assessmentId from response)
curl http://localhost:8080/v1/assessments/{assessmentId}

# Health check
curl http://localhost:8080/health
```

## Debugging

### Logs

```bash
# View container logs
make logs

# View specific service
docker-compose logs -f api
docker-compose logs -f postgres
```

### Database Inspection

```bash
# Connect to PostgreSQL
docker-compose exec postgres psql -U trustgraph -d trustgraph

# View assessments
SELECT * FROM assessment;

# View audit logs
SELECT * FROM audit_log ORDER BY created_at DESC LIMIT 10;
```

### Code Debugging

Add debug logging:

```go
logger.Debug("Debug message", zap.String("key", "value"))
```

Set `LOG_LEVEL=debug` in environment.

## Build and Deploy

### Docker Build

```bash
# Build image (also done by docker-compose build)
docker build -t trustgraph:latest .

# Run standalone
docker run -e TRUSTGRAPH_DATABASE_URL=... -p 8080:8080 trustgraph:latest
```

### Local Binary

```bash
go build -o trustgraph ./cmd/trustgraph-api
./trustgraph
```

### Render Deployment (Phase 1)

1. Connect GitHub repository to Render
2. Create Render Postgres service
3. Create Render Web Service with:
   - Build command: `go build -o trustgraph ./cmd/trustgraph-api`
   - Start command: `./trustgraph`
   - Environment variables: copy from `.env.example`
4. Set `TRUSTGRAPH_DATABASE_URL` to Render Postgres URL

## Common Issues

### "connection refused" to PostgreSQL

Make sure PostgreSQL is running:
```bash
# Docker
docker-compose ps
docker-compose up -d postgres

# Local
psql -U trustgraph -d trustgraph -c "SELECT 1"
```

### "migration already applied"

This is expected. Migrations track applied versions in `schema_migrations` table.

### Port 8080 already in use

Change port:
```bash
docker-compose down
TRUSTGRAPH_PORT=8081 make up
```

Or locally:
```bash
TRUSTGRAPH_PORT=8081 go run ./cmd/trustgraph-api/main.go
```

## Next Steps (Phase 1.5+)

1. **Real signal providers**: Implement device fingerprinting, velocity, email/phone verification
2. **Policy engine**: Move decision logic from handler to dedicated policy module
3. **Circuit breaker**: Implement actual circuit breaker for signal providers
4. **Audit logging**: Add comprehensive audit events
5. **Async assessment**: Queue slow signals, return provisional tier immediately
6. **Tests**: Unit and integration test coverage

## References

- [PHASE_1_IMPLEMENTATION.md](./PHASE_1_IMPLEMENTATION.md) — Technical design
- [README.md](./README.md) — Architecture overview
- [ConnectionSphere integration](../connectionsphere/docs/TRUSTGRAPH_INTEGRATION_PHASE_1.md) — How CS calls TrustGraph

## Contributing

1. Create branch from `main`
2. Make changes
3. Run tests: `make test`
4. Run linter: `make lint`
5. Create PR with reference to phase/issue
6. Get review from team lead

---

Questions? Check the docs or ask in the team channel.
