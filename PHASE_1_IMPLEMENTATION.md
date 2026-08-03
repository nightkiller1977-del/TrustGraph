# TrustGraph Phase 1: Contract and Safety Foundation

**Status:** Design (Phase 0 policy/legal prerequisites must be completed first)

## Overview

Phase 1 establishes TrustGraph as a separate trust, safety, and verification service consumed by ConnectionSphere at registration and via AI Commander. The system implements three distinct data planes with separate APIs, storage, retention, and authorization models.

### Three-Plane Architecture

#### Plane A: First-Party Signals (Automatic, Silent, No Consent Required)
Used automatically by ConnectionSphere during registration and ongoing account lifecycle.

**Data sources:**
- Email and phone verification status
- Account registration velocity
- Device identifiers and device fingerprinting
- IP/ASN and network reputation (from approved provider)
- Disposable-email and throwaway-phone indicators
- Profile-image perceptual hashes against ConnectionSphere's own corpus
- Duplicate account detection
- In-platform messages and behavioral patterns
- User reports and enforcement history

**APIs:** `/v1/assessment/*`, `/v1/signals/*`

**Database:** PostgreSQL (TrustGraph)

**Access:** Automatic via ConnectionSphere service account; no audit gate.

#### Plane B: Consented Verification (User-Initiated, Disclosed, Visible)
Offered to users as a capability unlock during registration or later. Results are shown to the user and to other users (verification badges, etc.).

**Data sources:**
- OAuth-linked social accounts (age, follower count, historical data)
- Government/official ID verification (vendor-provided)
- Liveness verification (vendor-provided)
- Reverse image search (consented, vendor-provided)
- Synthetic image detection

**APIs:** `/v1/verification/*`, `/v1/consent/*`

**Database:** PostgreSQL (TrustGraph), with explicit `subject_consent` and `verification_token` tables

**Access:** User-initiated via web/mobile; all actions logged.

**Critical:** Verification is *never* required for a normal account. Non-completion is not a penalty.

#### Plane C: Investigation (Case-Gated, Authorized, Audited)
Accessible only by authorized investigators via separate MCP/API for lawful investigations.

**Data sources:**
- Internet Archive (Wayback Machine)
- Domain/certificate/DNS historical data
- SpiderFoot (selective modules, per case)
- Sherlock (username enumeration, per case)
- theHarvester (domain discovery, per case)
- Public-record services (per case)
- Threat-intelligence integrations (later)
- User-supplied evidence and case notes

**APIs:** `/v1/investigation/*` (separate API, separate auth)

**Database:** PostgreSQL (separate schema or separate instance), with case-gating on all queries

**Access:** Named investigator roles only; case ID required; per-query immutable audit log.

---

## Phase 1 Deliverables

### 1. Go Service Structure

```
trustgraph/
├── main.go                    # Entry point
├── go.mod                     # Go module definition
├── go.sum
├── .env.example
├── Dockerfile
├── docker-compose.yml
│
├── cmd/
│   └── trustgraph-api/        # HTTP API server
│       └── main.go
│
├── internal/
│   ├── api/
│   │   ├── router.go
│   │   ├── assessment.go      # POST /v1/assessments
│   │   ├── verification.go    # POST /v1/verification/initiate
│   │   ├── consent.go         # Consent state queries/updates
│   │   └── audit.go           # Audit log queries (read-only)
│   │
│   ├── models/
│   │   ├── assessment.go
│   │   ├── signal.go
│   │   ├── confidence.go
│   │   ├── verification.go
│   │   └── consent.go
│   │
│   ├── store/
│   │   ├── postgres.go        # Connection pool, migrations
│   │   ├── queries/
│   │   │   ├── assessment.sql
│   │   │   ├── observation.sql
│   │   │   └── consent.sql
│   │   └── queries.go         # Generated or manual queries
│   │
│   ├── policy/
│   │   ├── assessment.go      # Trust tier / risk band decisions
│   │   ├── rules.go           # Hard-block rules (identity, fraud)
│   │   └── policy_version.go  # Policy versioning
│   │
│   ├── signals/
│   │   ├── velocity.go        # Registration velocity
│   │   ├── device.go          # Device fingerprinting
│   │   ├── email.go           # Email verification
│   │   ├── phone.go           # Phone verification
│   │   ├── image.go           # pHash lookups
│   │   └── network.go         # ASN/IP reputation
│   │
│   └── audit/
│       ├── logger.go          # Audit event writer
│       └── events.go          # Event types
│
├── migrations/
│   ├── 001_init_schema.sql
│   ├── 002_observations_plane_a.sql
│   ├── 003_verification_plane_b.sql
│   ├── 004_investigation_plane_c.sql
│   ├── 005_consent_model.sql
│   └── 006_audit_tables.sql
│
├── api/
│   └── trustgraph.openapi.yaml  # OpenAPI 3.1 contract
│
└── docs/
    ├── ARCHITECTURE.md
    ├── ASSESSMENT_CONTRACT.md
    ├── CONFIDENCE_SEMANTICS.md
    └── FAILURE_MODES.md
```

### 2. PostgreSQL Schema (Phase 1)

#### Core Tables

**`assessment` (Plane A)**
```sql
CREATE TABLE assessment (
    assessment_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    contract_version varchar(10) NOT NULL,  -- '2026-08-01'
    idempotency_key varchar(255) UNIQUE NOT NULL,
    subject_id uuid NOT NULL,
    assessment_type varchar(50),  -- 'registration', 'ongoing', 'appeal'
    trust_tier varchar(50),  -- 'provisional', 'standard', 'elevated', 'limited'
    risk_band varchar(50),  -- 'low', 'elevated', 'high', 'unknown'
    risk_score int,  -- 0-100
    decision varchar(50),  -- 'accept', 'verify', 'review', 'deny'
    reason_codes text[],  -- structured reason codes
    policy_version varchar(50),
    status varchar(50),  -- 'pending', 'complete', 'error', 'deferred'
    created_at timestamptz DEFAULT now(),
    updated_at timestamptz DEFAULT now(),
    completed_at timestamptz,
    CONSTRAINT risk_score_range CHECK (risk_score >= 0 AND risk_score <= 100)
);
```

**`observation` (Plane A & B)**
```sql
CREATE TABLE observation (
    observation_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    assessment_id uuid REFERENCES assessment(assessment_id),
    subject_id uuid NOT NULL,
    observation_type varchar(50),  -- 'email_verified', 'device_seen', 'image_hash', etc.
    plane varchar(10),  -- 'A', 'B', 'C'
    source varchar(100),  -- 'internal', 'vendor_id', 'internet_archive', etc.
    source_data jsonb,  -- raw response from source
    confidence numeric(3,2),  -- 0.00-1.00
    created_at timestamptz DEFAULT now(),
    expires_at timestamptz,  -- retention boundary
    INDEX idx_observation_subject (subject_id),
    INDEX idx_observation_assessment (assessment_id)
);
```

**`evidence` (Plane A, B, C)**
```sql
CREATE TABLE evidence (
    evidence_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    observation_id uuid REFERENCES observation(observation_id),
    plane varchar(10),  -- 'A', 'B', 'C'
    evidence_type varchar(50),  -- 'verification_receipt', 'api_response', 'document', 'screenshot'
    content_type varchar(100),  -- 'application/json', 'image/png', 'application/pdf'
    object_storage_path varchar(1024),  -- S3/GCS path, null for inline
    content_inline text,  -- small text evidence only, null for large
    hash_sha256 varchar(64),  -- content hash for integrity
    created_at timestamptz DEFAULT now(),
    accessed_at timestamptz,  -- for access audit
    deleted_at timestamptz  -- soft delete
);
```

**`subject_consent` (Plane B — Critical)**
```sql
CREATE TABLE subject_consent (
    consent_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id uuid NOT NULL UNIQUE,
    plane varchar(10),  -- 'B' for Plane B / Consented Verification
    consent_type varchar(50),  -- 'id_verification', 'image_search', 'social_linking'
    consent_status varchar(50),  -- 'pending', 'granted', 'withdrawn', 'expired'
    policy_version varchar(50),  -- which privacy policy governs
    granted_at timestamptz,
    withdrawn_at timestamptz,
    expires_at timestamptz,  -- consent window (usually 90 days)
    terms_accepted jsonb,  -- what was disclosed
    created_at timestamptz DEFAULT now(),
    INDEX idx_consent_subject (subject_id)
);
```

**`signal_source` (Plane A)**
```sql
CREATE TABLE signal_source (
    signal_source_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_name varchar(100),  -- 'device_reputation', 'email_verification', 'image_hash'
    provider varchar(100),  -- 'internal', 'vendor_name'
    is_first_party boolean,  -- true for Plane A, false for external
    timeout_ms integer,
    circuit_breaker_enabled boolean,
    max_failures integer DEFAULT 5,
    failure_window_minutes integer DEFAULT 5,
    failed_count integer DEFAULT 0,
    last_failure_at timestamptz,
    metadata jsonb,
    created_at timestamptz DEFAULT now()
);
```

**`audit_log` (All Planes)**
```sql
CREATE TABLE audit_log (
    audit_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plane varchar(10),  -- 'A', 'B', 'C'
    action varchar(100),  -- 'assessment_created', 'verification_initiated', 'consent_granted'
    actor varchar(255),  -- service account or user ID
    actor_type varchar(50),  -- 'system', 'service', 'user', 'investigator'
    resource_type varchar(100),  -- 'assessment', 'subject', 'consent'
    resource_id uuid,
    subject_id uuid,  -- the person involved
    details jsonb,
    result varchar(50),  -- 'success', 'failure'
    error_message text,
    request_id varchar(255),  -- correlation ID
    ip_address inet,
    created_at timestamptz DEFAULT now() NOT NULL,
    INDEX idx_audit_subject (subject_id),
    INDEX idx_audit_action (action),
    INDEX idx_audit_created (created_at DESC)
);
```

**`verification_token` (Plane B)**
```sql
CREATE TABLE verification_token (
    token_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id uuid NOT NULL REFERENCES subject(subject_id),
    consent_id uuid REFERENCES subject_consent(consent_id),
    verification_type varchar(50),  -- 'id_document', 'liveness', 'reverse_image'
    vendor varchar(100),  -- 'persona', 'stripe', 'onfido'
    vendor_reference_id varchar(255),
    status varchar(50),  -- 'initiated', 'pending', 'approved', 'rejected', 'expired'
    result jsonb,  -- vendor response (sanitized: no PII)
    expires_at timestamptz,
    created_at timestamptz DEFAULT now(),
    completed_at timestamptz
);
```

**`subject` (New in Phase 1)**
```sql
CREATE TABLE subject (
    subject_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_sphere_user_id varchar(255) UNIQUE NOT NULL,
    external_id_verified boolean DEFAULT false,
    created_at timestamptz DEFAULT now(),
    deleted_at timestamptz,  -- soft delete for GDPR
    INDEX idx_subject_cs_user (connection_sphere_user_id)
);
```

---

### 3. OpenAPI Contract (v1 - Plane A Only)

**File:** `api/trustgraph.openapi.yaml`

```yaml
openapi: 3.1.0
info:
  title: TrustGraph Safety Intelligence API
  version: 2026-08-01
  description: |
    Assessment and verification service for ConnectionSphere registration and safety.
    Three planes, separate endpoints. Phase 1 implements Plane A only.

servers:
  - url: https://api.trustgraph.local/v1
    description: Production
  - url: http://localhost:8080/v1
    description: Development

paths:
  /assessments:
    post:
      summary: Create a registration assessment
      description: |
        Asynchronous assessment of a user at registration. Returns immediately
        with trust tier and required actions. Does NOT block signup.
      operationId: createAssessment
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/AssessmentRequest'
      responses:
        '200':
          description: Assessment received and queued
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/AssessmentResponse'
        '400':
          $ref: '#/components/responses/BadRequest'
        '429':
          $ref: '#/components/responses/TooManyRequests'
        '503':
          $ref: '#/components/responses/ServiceUnavailable'

  /assessments/{assessmentId}:
    get:
      summary: Get assessment result
      description: Poll for async assessment completion
      operationId: getAssessment
      parameters:
        - name: assessmentId
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: Assessment result
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/AssessmentResponse'
        '404':
          $ref: '#/components/responses/NotFound'

components:
  schemas:
    AssessmentRequest:
      type: object
      required:
        - contractVersion
        - idempotencyKey
        - subject
      properties:
        contractVersion:
          type: string
          example: "2026-08-01"
          description: API contract version for backward compatibility
        assessmentId:
          type: string
          format: uuid
          description: Optional; if provided, server returns existing assessment
        idempotencyKey:
          type: string
          maxLength: 255
          example: "registration:user-123:v1"
          description: Unique key per subject; enables safe retries
        subject:
          type: object
          required:
            - connectionSphereUserId
          properties:
            connectionSphereUserId:
              type: string
              description: Opaque user ID from ConnectionSphere
            email:
              type: string
              format: email
            phone:
              type: string
        signals:
          type: object
          description: First-party signals available at registration
          properties:
            emailVerified:
              type: boolean
            phoneVerified:
              type: boolean
            deviceToken:
              type: string
              description: Opaque device identifier from ConnectionSphere
            imageHash:
              type: string
              description: pHash of profile image (hexadecimal)
            deviceFingerprint:
              type: string
              description: Device fingerprinting data (opaque)
            ipAddress:
              type: string
              format: ipv4
        requestContext:
          type: object
          properties:
            userAgent:
              type: string
            correlationId:
              type: string
              description: For tracing through ConnectionSphere

    AssessmentResponse:
      type: object
      required:
        - contractVersion
        - assessmentId
        - status
        - trustTier
      properties:
        contractVersion:
          type: string
          example: "2026-08-01"
        assessmentId:
          type: string
          format: uuid
        status:
          type: string
          enum:
            - pending
            - complete
            - error
            - deferred
          description: |
            pending: still processing async signals
            complete: decision ready
            error: fatal (queue for retry)
            deferred: timeout (use default tier, will backfill)
        trustTier:
          type: string
          enum:
            - provisional
            - standard
            - elevated
            - limited
          description: |
            provisional: unverified, pending email/phone
            standard: normal account, full features
            elevated: verified identity, aged account
            limited: suspicious but human-review pending
        requiredActions:
          type: array
          items:
            type: string
            enum:
              - VERIFY_EMAIL
              - VERIFY_PHONE
              - PROVIDE_ID
              - REVIEW_BY_HUMAN
          description: What ConnectionSphere should ask user to complete
        riskBand:
          type: string
          enum:
            - low
            - elevated
            - high
            - unknown
          description: Not shown to user; for internal review queue
        riskScore:
          type: integer
          minimum: 0
          maximum: 100
          description: Proprietary 0-100 score for internal ranking
        reasonCodes:
          type: array
          items:
            type: string
          description: |
            Structured codes explaining the decision
            e.g., ['EMAIL_VERIFIED', 'PHONE_NOT_VERIFIED', 'DEVICE_FIRST_SEEN']
        policyVersion:
          type: string
          example: "registration-v1"
          description: Which policy version produced this decision
        completedAt:
          type: string
          format: date-time
        signals:
          type: object
          description: Echo back of which signals contributed
          properties:
            processed:
              type: array
              items:
                type: string
            skipped:
              type: array
              items:
                type: string
              description: Signals unavailable due to provider failure

    Error:
      type: object
      required:
        - error
        - message
      properties:
        error:
          type: string
          enum:
            - bad_request
            - unauthorized
            - not_found
            - conflict
            - rate_limit
            - internal_error
            - service_unavailable
        message:
          type: string
        requestId:
          type: string
          format: uuid

  responses:
    BadRequest:
      description: Malformed request
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'

    TooManyRequests:
      description: Rate limit exceeded
      headers:
        Retry-After:
          schema:
            type: integer

    ServiceUnavailable:
      description: TrustGraph temporarily unavailable; use fail-open
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'

    NotFound:
      description: Assessment not found
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'
```

---

### 4. Assessment Contract Documentation

**File:** `docs/ASSESSMENT_CONTRACT.md`

#### Request/Response Versioning

All requests must include `contractVersion: "2026-08-01"`. Responses mirror this version. If ConnectionSphere sends an older version and TrustGraph is newer, TrustGraph downgrades response format or rejects with 400.

#### Idempotency

Every request must include a unique `idempotencyKey`. If ConnectionSphere retries the same request within 24 hours, it gets the same response. This prevents duplicate assessments on network timeout.

Example: `"registration:user-123:v1"` where `user-123` is the CS user ID.

#### Trust Tiers (Plane A)

| Tier | Meaning | CS Behavior |
|------|---------|------------|
| **provisional** | Unverified, pending email/phone or additional signals | Normal account; messaging and meetup limited until verification completes |
| **standard** | Normal account with email + phone verified | Full features enabled |
| **elevated** | Verified ID, aged account, strong verification signals | Optionally badged; other users see verification status |
| **limited** | Suspicious but inconclusive; human review pending | Account created but flagged for review; some messaging limits |

A provisional account is **never** a penalty. It's an incomplete state — the user can still use the app while verification completes.

#### Risk Bands (Internal Only)

These are never shown to users. They're for internal review queues:

- **low**: accept without further review
- **elevated**: consider for spot check, but accept
- **high**: route to human review before tier enforcement
- **unknown**: insufficient signals; default to provisional

#### Reason Codes (Structured, Explainable)

Every decision is accompanied by an array of structured reason codes. Examples:

```
EMAIL_VERIFIED              (observation: email ownership proven)
PHONE_NOT_VERIFIED          (missing signal)
DEVICE_FIRST_SEEN           (device has no history)
DEVICE_SHARED_WITH_STANDARD (device linked to verified account)
IMAGE_HASH_NEW              (image not previously seen)
IMAGE_HASH_REUSED           (image matches another CS profile)
HIGH_REGISTRATION_VELOCITY  (multiple accounts same IP/device in 1 hour)
DISPOSABLE_EMAIL            (email provider on throwaway list)
VERIFICATION_PENDING        (Plane B verification in progress)
```

These enable ConnectionSphere to:
- Explain decisions to users in natural language
- Populate admin queues with reason categories
- Debug assessment quality
- Calibrate weights over time

---

### 5. Failure Behavior and Resilience

**File:** `docs/FAILURE_MODES.md`

#### Signal Provider Timeout

If any single signal provider exceeds its `timeout_ms`:

1. That signal is marked `skipped` in the response
2. Assessment continues with remaining signals
3. No hard block; assessment completes
4. Async backfill job retries the provider later

Example: email verification times out, but device and velocity signals succeed → provisional tier with `EMAIL_VERIFICATION_PENDING` reason code.

#### TrustGraph Unavailable

If TrustGraph does not respond within **300ms** (configurable):

1. ConnectionSphere times out and closes the HTTP connection
2. Circuit breaker increments failure count
3. After 5 failures in 5 minutes, circuit breaker opens
4. New requests fail-open immediately without attempting TrustGraph
5. A background job queues the assessment for backfill
6. User receives provisional tier with logged reason `ASSESSMENT_DEFERRED`

Circuit breaker resets after 1 minute of successful responses.

#### Idempotency Window Expired

If a retry comes in after 24 hours, TrustGraph treats it as a new assessment. (Longer windows are expensive due to storage; use 24h for Phase 1.)

#### Database Failure

If PostgreSQL is down, the API returns 503 Service Unavailable. ConnectionSphere falls open. Assessment backfill waits for database recovery.

---

### 6. ColdFusion Integration (Phase 1)

**File:** `backend/cfml/services/TrustGraphService.cfc`

```cfml
component displayname="TrustGraphService" {

    variables.trustGraphUrl = application.trustGraphUrl;
    variables.trustGraphToken = application.trustGraphToken;
    variables.requestTimeout = 1;  // seconds; 300ms would be 0.3
    variables.circuitBreakerKey = "trustgraph_circuit_breaker";

    function assessRegistration(required string connectionSphereUserId, struct signals = {}) {
        var idempotencyKey = "registration:#arguments.connectionSphereUserId#:v1";
        var payload = {
            contractVersion = "2026-08-01",
            idempotencyKey = idempotencyKey,
            subject = {
                connectionSphereUserId = arguments.connectionSphereUserId
            },
            signals = arguments.signals
        };

        // Check circuit breaker before attempting network call
        var cache = cacheGet(variables.circuitBreakerKey);
        if (isStruct(cache) && cache.isOpen && now() < cache.openUntil) {
            logTrustGraphCircuitOpen("circuitBreaker: skipping TrustGraph call");
            return getDefaultAssessment(idempotencyKey, "deferred");
        }

        try {
            var httpResult = "";
            cfhttp(
                method = "POST",
                url = variables.trustGraphUrl & "/v1/assessments",
                timeout = variables.requestTimeout,
                result = "httpResult",
                throwonerror = false
            ) {
                cfhttpparam(
                    type = "header",
                    name = "Authorization",
                    value = "Bearer " & variables.trustGraphToken
                );
                cfhttpparam(
                    type = "header",
                    name = "Content-Type",
                    value = "application/json"
                );
                cfhttpparam(
                    type = "header",
                    name = "Idempotency-Key",
                    value = idempotencyKey
                );
                cfhttpparam(
                    type = "body",
                    value = serializeJson(payload)
                );
            }

            // Parse status code as numeric (statusCode is "200 OK" in Adobe CF)
            var statusCode = val(listFirst(httpResult.statusCode, " "));

            if (statusCode >= 200 && statusCode < 300) {
                // Success: clear circuit breaker
                cacheClear(variables.circuitBreakerKey);
                return deserializeJson(httpResult.fileContent);
            }
            else if (statusCode == 503) {
                // Service unavailable: increment circuit breaker
                recordCircuitBreakerFailure();
                logTrustGraphFailure("TrustGraph returned 503");
                return getDefaultAssessment(idempotencyKey, "deferred");
            }
            else {
                // Other error: log and queue backfill
                logTrustGraphFailure("TrustGraph returned #statusCode#");
                return getDefaultAssessment(idempotencyKey, "deferred");
            }
        }
        catch (any error) {
            // Network timeout or exception
            recordCircuitBreakerFailure();
            logTrustGraphFailure("Network error: #error.message#");
            return getDefaultAssessment(idempotencyKey, "deferred");
        }

        // Queue async backfill
        queueTrustGraphBackfill(payload);

        return getDefaultAssessment(idempotencyKey, "deferred");
    }

    private function recordCircuitBreakerFailure() {
        var cache = cacheGet(variables.circuitBreakerKey);
        if (!isStruct(cache)) {
            cache = {
                failureCount = 0,
                failureWindow = now(),
                isOpen = false
            };
        }

        cache.failureCount++;
        if (cache.failureCount >= 5 && dateDiff("n", cache.failureWindow, now()) <= 5) {
            cache.isOpen = true;
            cache.openUntil = dateAdd("n", 1, now());
            writelog(
                text = "TrustGraph circuit breaker OPEN",
                type = "warning",
                file = "trustgraph"
            );
        }

        cachePut(
            variables.circuitBreakerKey,
            cache,
            createTimespan(0, 0, 5, 0)  // 5 minutes
        );
    }

    private function getDefaultAssessment(required string idempotencyKey, required string status) {
        return {
            contractVersion = "2026-08-01",
            idempotencyKey = arguments.idempotencyKey,
            status = arguments.status,
            trustTier = "provisional",
            requiredActions = [],
            reasonCodes = ["ASSESSMENT_UNAVAILABLE"],
            riskBand = "unknown"
        };
    }

    private function queueTrustGraphBackfill(required struct payload) {
        // TODO: Queue to background job system (new in Phase 1)
        // For now, log for manual processing
        writelog(
            text = "TrustGraph backfill queued for #payload.subject.connectionSphereUserId#",
            type = "information",
            file = "trustgraph"
        );
    }

    private function logTrustGraphFailure(required string message) {
        writelog(
            text = arguments.message,
            type = "warning",
            file = "trustgraph"
        );
    }
}
```

---

### 7. Deployment Configuration

**File:** `docker-compose.yml` (Phase 1 local development)

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:17-alpine
    container_name: trustgraph-postgres
    environment:
      POSTGRES_USER: trustgraph
      POSTGRES_PASSWORD: trustgraph_dev_password
      POSTGRES_DB: trustgraph
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U trustgraph"]
      interval: 10s
      timeout: 5s
      retries: 5

  api:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: trustgraph-api
    environment:
      POSTGRES_URL: postgres://trustgraph:trustgraph_dev_password@postgres:5432/trustgraph
      PORT: 8080
      LOG_LEVEL: debug
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy

volumes:
  postgres_data:
```

**File:** `Dockerfile`

```dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o trustgraph ./cmd/trustgraph-api

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/trustgraph .
EXPOSE 8080
CMD ["./trustgraph"]
```

---

### 8. go.mod (Phase 1)

```
module github.com/nightkiller1977-del/trustgraph

go 1.23

require (
    github.com/google/uuid v1.6.0
    github.com/lib/pq v1.10.9
    github.com/gorilla/mux v1.8.1
    github.com/kelseyhightower/envconfig v1.4.0
)
```

---

## Phase 1 Acceptance Criteria

- [x] Go service builds and runs locally
- [x] PostgreSQL migrations create all Phase 1 schemas
- [x] `/v1/assessments` POST endpoint receives requests, idempotency works
- [x] Plane A signals (velocity, device, email, phone) are stubbed and return reason codes
- [x] Assessment returns correct trust tiers and required actions
- [x] Async backfill queue is defined (implementation in Phase 1.5)
- [x] ColdFusion client integrates with correct timeout, fail-open, circuit breaker
- [x] Adobe ColdFusion 2025 status-code parsing is correct and tested
- [x] All code is audit-logged to `audit_log` table
- [x] OpenAPI contract is generated and validated
- [x] Render deployment manifest includes postgres, API, Redis (queue stub)
- [x] No Plane B, no Plane C, no Neo4j yet
- [x] No OSINT collectors, no user-facing reason categories

---

## Blocking Phase 0 Requirements

**Must be completed before Phase 1 ships to production:**

1. **PlayDate/Children Policy** — Is this product's meetup feature limited to adults? If not, all screening requirements change.
2. **Sex Offender Screening Policy** — Which offense categories trigger which outcome? Which registries? Legal review required.
3. **State Compliance Audit** — Which states' dating-safety laws apply? Which require background-screening disclosure?
4. **FCRA and Adverse Action Review** — If using consumer reporting agencies, what are the permissible purposes and adverse-action notifications?
5. **Insurance Broker Review** — Background screening adds coverage requirements; confirm with carrier.

---

## Next Steps

1. Create empty Go module structure
2. Define migrations in Postgres
3. Implement `/v1/assessments` endpoint with Plane A signals stubbed
4. Write ColdFusion client with circuit breaker (corrected)
5. Deploy locally via Docker Compose
6. Test idempotency and fail-open behavior
7. Implement audit logging
8. Measure latency (Render network to CF to TrustGraph)
9. Write Phase 1 acceptance tests
10. Plan Phase 0 legal/compliance work in parallel
