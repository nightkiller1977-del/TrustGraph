# TrustGraph: Identity, Trust, and Verification Intelligence

**Status:** Phase 1 Planning (See `PHASE_1_IMPLEMENTATION.md`)

TrustGraph is a separate, dedicated trust, safety, and verification service consumed by [ConnectionSphere](https://github.com/nightkiller1977-del/connectionsphere) (ColdFusion dating app) and orchestrated by [AI Commander](https://github.com/nightkiller1977-del/Ai-Command-Center-Desktop-App).

It is **not** an investigation platform, though it can support one. It is **not** a surveillance system, though its Investigation plane enables lawful investigations. It is built on the principle that ConnectionSphere remains the product; TrustGraph provides safety and trust signals.

## Three-Plane Architecture

TrustGraph separates concerns by data source, consent model, authorization, and use case:

### Plane A: First-Party Signals
**Automatic, silent, no consent required**

Used automatically during registration and ongoing account lifecycle.

- Email and phone verification status
- Registration velocity
- Device fingerprinting
- Network reputation (IP/ASN)
- Disposable-email detection
- Image perceptual hashing (own corpus only)
- Duplicate account detection
- In-platform behavioral patterns
- User reports and enforcement history

**Access:** ConnectionSphere service account only; no human access without audit escalation.

**Retention:** Account lifetime, deletion cascade on account removal.

### Plane B: Consented Verification
**User-initiated, transparent, disclosed, visible**

Offered to users as a capability unlock. Results are shown to the user and may be shown to other users (verification badges).

- OAuth-linked social accounts (age, follower count)
- Government ID verification (vendor-provided, vendor-deleted)
- Liveness verification (vendor-provided, vendor-deleted)
- Reverse image search (optional, user-initiated)
- Synthetic image detection

**Verification is never required.** Non-completion is never a penalty. A user can reach full account capabilities through Plane A signals + phone/email verification.

**Access:** User-initiated; all actions logged per consent record.

**Retention:** Vendor-governed (immediate deletion after decision). Metadata retained per consent policy.

### Plane C: Investigation
**Case-gated, authorized, audited**

Accessible only by named investigators via separate MCP tools for lawful investigations.

- Internet Archive and historical sources
- Domain/certificate/DNS intelligence
- Username enumeration (Sherlock)
- Domain discovery (theHarvester)
- Public-record services
- SpiderFoot (selective modules, per-case rate-limited)
- User-supplied evidence
- Threat-intelligence integrations (later)

**Access:** Named investigator roles only; case ID required on every query; immutable per-query audit log; break-glass alerting.

**Retention:** Case-driven; automatic retention review per case disposition.

---

## Current State (Early August 2026)

- [ ] Go service scaffolding
- [ ] PostgreSQL schema (Phase 1)
- [ ] Assessment endpoint
- [ ] ConnectionSphere integration
- [ ] Audit logging
- [ ] Deployment manifest

---

## Getting Started

See [`PHASE_1_IMPLEMENTATION.md`](./PHASE_1_IMPLEMENTATION.md) for the complete Phase 1 design, including:

- Go service structure and file layout
- PostgreSQL schema (all tables, indexes)
- OpenAPI contract
- ColdFusion integration (corrected)
- Failure modes and resilience
- Deployment configuration
- Blocking Phase 0 legal/compliance requirements

---

## Design Principles

1. **Separate data planes from the start.** First-party + consented + investigation are legally, operationally, and ethically distinct. Don't fold them together and risk leaking investigation PII into registration signals.

2. **Never auto-deny users for lack of outside data.** A person with no public footprint is not fraud. Lack of searchable presence should never equal account suspension.

3. **Consent must be explicit and separate.** Plane B data is only collected after the user asks for it and sees what disclosure applies.

4. **Verification is a capability unlock, not a prerequisite.** Badges and trust tiers reward linked accounts; they don't penalize disconnected ones.

5. **Observations precede inferences.** Store the raw signal first. The relationship or risk assessment is secondary and revisable.

6. **Entity resolution must be reversible.** Never destructively merge identities. Use edges that can be rejected or split.

7. **Confidence scores must be defined.** Not just `0.84`—what does that mean? Calibrated against what outcome? The definition belongs in the contract.

8. **Fail open under load.** TrustGraph downtime equals signup disruption. Timeouts trigger provisional accounts and async backfill, not signup blocks.

9. **Investigator access is audited per query.** Every lookup records who asked, why (case ID), when, and with what result. Break-glass and self-lookup alert automatically.

10. **AI Commander is aware, not in control.** It receives events and can query for explanations. It does not directly modify the graph or override policies.

---

## Integration Points

### ConnectionSphere (ColdFusion)

During registration, call:

```cfml
var assessment = trustGraphService.assessRegistration(
    connectionSphereUserId = newUser.id,
    signals = {
        emailVerified = true,
        phoneVerified = false,
        deviceToken = "...",
        imageHash = "..."
    }
);

// Response includes trustTier (provisional/standard/elevated/limited)
// and requiredActions (VERIFY_EMAIL, PROVIDE_ID, etc.)

userService.setTrustTier(newUser.id, assessment.trustTier);
if (assessment.requiredActions.contains("VERIFY_PHONE")) {
    // Prompt user to verify phone
}
```

See [`PHASE_1_IMPLEMENTATION.md`](./PHASE_1_IMPLEMENTATION.md#6-coldfusion-integration-phase-1) for full ColdFusion stub with circuit breaker and fail-open behavior.

### AI Commander

Once Plane C is implemented, AI Commander can:

```
trustgraph.assessment.explain(userId)
  → why did this user get provisional tier?

trustgraph.investigation.case.create({
  subject: "email@example.com",
  purpose: "fraud_report",
  authorizedInvestigator: "alice@investigator.local"
})
  → creates case, publishes event

trustgraph.investigation.entity.expand(caseId, entityId)
  → returns connected entities with evidence

trustgraph.investigation.relationship.explain(caseId, sourceId, targetId)
  → why are these two connected? Which observations support it?
```

Phase 1 implements read-only explanatory tools only. Write operations and external searches require explicit authorization and case assignment.

---

## Data Retention and Deletion

### Plane A (First-Party)
- Account lifetime by default
- Deleted when ConnectionSphere account is deleted
- Assessment history retained for 2 years (audit/dispute)

### Plane B (Consented)
- Vendor-deleted immediately after verification decision
- Consent records retained for 1 year (legal proof of disclosure)
- User can withdraw consent; triggers consent record update (not destructive deletion)

### Plane C (Investigation)
- Case-specific retention plan established at case creation
- Automatic retention review 90 days after case closure
- Deletion requires investigator sign-off

---

## Legal and Compliance Notes

**This is not legal advice.** Before deploying:

1. Confirm with counsel whether your product involves children. If yes, entire compliance posture changes.
2. Write explicit sex-offender screening policy (which offense categories, which registries, which outcome).
3. Audit state dating-safety laws (CA, TX, NY, IL, CT, NJ have meaningful provisions).
4. If using consumer reporting agencies, review FCRA permissible-purpose and adverse-action requirements.
5. Confirm with insurance broker that background screening is covered under your policy.
6. Document data-processing agreements with any third-party vendors (ID verification, reverse image search).

---

## Technology Stack

- **Language:** Go (core) + Python (analysis, later)
- **Database:** PostgreSQL (initial) + Neo4j (when path queries hurt, Phase 4 onward)
- **Queue:** Redis (backfill, async assessment)
- **Deployment:** Render (containerized services)
- **External Services:** Persona/Stripe Identity (ID verification), TinEye (reverse image search)

---

## Contributing

This is a single-user repository for now. Future contributions should follow:

1. Create a branch from `main`
2. Update PHASE_1_IMPLEMENTATION.md or create new phase docs
3. Implement corresponding code in Go/SQL
4. Run migrations and tests
5. Commit with reference to phase and acceptance criteria
6. Open PR with link to AI Commander work

---

## License

TBD

---

## Contact

Questions about TrustGraph architecture or design should be raised in AI Commander's investigation planning issue.
