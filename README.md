# TrustGraph

TrustGraph is a **trust-assessment, policy, calibration, and capability-gating service** for ConnectSphere. It records trust-related signals and assessment evidence, produces explainable trust decisions/tiers, supports human review and calibration, and lets the application gate selected capabilities without embedding trust logic throughout the product codebase.

## Current status — September 9, 2026

**State: Phase 1/1.5 implementation integrated with ConnectSphere; conservative/shadow-oriented rollout and calibration continue.**

The old README's “Phase 1 Planning” checklist is stale. `main` now contains working service code and integration foundations, including:

- Go API/service implementation with PostgreSQL persistence.
- Database migrations for the current trust data model.
- Trust assessment and decision-record workflows.
- Idempotent assessment behavior, including database-backed concurrency protection for duplicate/retried requests.
- Real PostgreSQL integration-test coverage for important persistence/idempotency behavior.
- Audit/evidence records intended to make trust decisions reviewable rather than opaque.
- Shadow-mode operation so new rules/thresholds can be measured before they become hard enforcement.
- Administrative review-queue support.
- Calibration/measurement metrics and a Phase 1.5 measurement plan.
- Appeals/policy-simulation scaffolding for evaluating and reviewing decisions.
- ConnectSphere integration for registration-time assessment and trust-tier/capability gating/backfill workflows.

TrustGraph should **not** be described as a finished automated safety system. The current architecture deliberately supports observation, human review, calibration, and policy simulation because trust signals can be incomplete or wrong and may have meaningful consequences for users.

## Role in ConnectSphere

```text
ConnectSphere event / user action
            │
            ▼
         TrustGraph
         ├─ collect/normalize evidence
         ├─ assess against policy
         ├─ record decision + rationale
         ├─ shadow/calibration metrics
         ├─ admin review / appeal path
         └─ return tier/capability decision
            │
            ▼
ConnectSphere capability gating
```

The application should consume a narrow trust/capability contract instead of copying scoring rules into frontend/backend feature code.

## Core principles

### Trust is a decision-support signal, not proof of safety

A score or tier cannot prove that a person is safe, honest, or trustworthy. It is a bounded risk/control signal based on available evidence and policy. Product copy and enforcement behavior should never present it as certainty.

### Explainability and auditability

Trust decisions should retain enough structured evidence, policy/version context, and decision history for an authorized reviewer to understand why an outcome occurred.

### Conservative enforcement

New policies and threshold changes should be measured in shadow/simulation mode before broad enforcement when possible. False positives and false negatives both matter.

### Human review and correction

Administrative review and appeals exist because automated assessment can be wrong. High-impact restrictions should have a review/correction path appropriate to the product's risk level.

### Idempotency

Repeated registration/events or retry storms should not create conflicting trust records or duplicate side effects. Database-backed idempotency/concurrency controls are part of the current implementation.

## Local development

A typical Go validation pass is:

```bash
go build ./...
go vet ./...
go test ./...
```

Use `Makefile`, `docker-compose.yml`, `.env.example`, `DEVELOPER.md`, and the current migrations as the source of truth for the development environment. Do not commit real database credentials, tokens, or private user evidence.

## Documentation

- `PHASE_1_IMPLEMENTATION.md` — detailed implementation/reference material.
- `PHASE_1.5_MEASUREMENT_PLAN.md` — calibration and measurement plan.
- `DEVELOPER.md` — development guidance.
- `SECURITY.md` — security requirements and reporting guidance.
- `docs/` — supporting design/operational documentation.

These files capture point-in-time plans and deeper detail. Code, migrations, tests, and live ConnectSphere integration take precedence if older prose conflicts with current behavior.

## Related repository

`connectionsphere` is the primary current consumer. It uses TrustGraph at registration and capability-gating points while the trust model continues to be calibrated.

## Documentation rule

This README describes what exists on `main` and the current rollout posture. Do not revert to planning-only checklists after implementation has landed, and do not call trust enforcement “complete” until calibration, privacy/safety review, operational monitoring, and intended production policy are demonstrably validated.
