# Security Policy

## Reporting Vulnerabilities

**Do not open a public issue for security vulnerabilities.**

If you discover a security vulnerability in TrustGraph, please email security details to the maintainers rather than using the public issue tracker.

### What to Include
- Description of the vulnerability
- Steps to reproduce (if applicable)
- Potential impact
- Suggested remediation (if you have one)

---

## Security Best Practices

### Environment Configuration
- **Never commit `.env` files** to the repository. Use `.env.example` as a template only.
- All sensitive configuration (database passwords, API keys, tokens) must be provided via environment variables at runtime.
- In development, use a local `.env` file (ignored by `.gitignore`).
- In production, use your deployment platform's secrets management (Render, Kubernetes secrets, etc.).

### Database Security
- Always use `sslmode=require` (or higher) in production PostgreSQL connections.
- Rotate database credentials regularly.
- Use strong, randomly-generated passwords.
- Restrict database network access via firewall rules.

### API Security
- Audit logging is mandatory for all operations (Plane A, B, C).
- All investigator access requires authentication and case authorization.
- Break-glass alerting triggers on unauthorized escalation attempts.
- Idempotency keys prevent duplicate assessment processing.
- Circuit breaker prevents cascading failures.

### Data Handling
- Plane A data (first-party signals) is never shared with external parties.
- Plane B data (consented verification) is vendor-deleted immediately after decision.
- Plane C data (investigation) is case-gated and case-audited.
- Assessment history is retained for 2 years for audit/dispute purposes only.
- User deletion cascades all associated data within retention windows.

### Code Security
- All external inputs are validated before processing.
- SQL queries use parameterized statements to prevent injection.
- No hardcoded credentials in code, documentation, or examples.
- Dependencies are regularly audited for vulnerabilities.

### Deployment Security
- Container images are built from scratch or minimal base images.
- Health checks and circuit breakers prevent bad state propagation.
- Fail-open behavior (timeouts) prevents signup disruption.
- Deployment manifests use environment variable substitution for secrets.

---

## Compliance Considerations

Before deploying TrustGraph:

1. **Children/Youth Safety:** If your product involves users under 18, ensure compliance with COPPA and state laws. This changes the entire assessment posture.
2. **Sex Offender Screening:** Document your policy (if any) on which registries you query, what you do with results, and what outcomes they trigger.
3. **State Laws:** Dating-safety laws in CA, TX, NY, IL, CT, and NJ have specific background-check requirements. Consult legal counsel.
4. **FCRA Compliance:** If you use consumer reporting agencies, follow adverse-action requirements and permissible-purpose rules.
5. **Insurance:** Confirm that background screening/verification is covered under your liability policy.
6. **Data Processing:** Execute Data Processing Agreements (DPAs) with all third-party vendors (ID verification, image search, etc.).

---

## Vulnerability Disclosure Timeline

- **Acknowledgment:** Within 48 hours of report
- **Investigation:** 1-2 weeks
- **Patch Release:** As soon as practicable; critical issues take priority
- **Public Disclosure:** After patch is released and users have reasonable time to upgrade

---

## Security Audit Checklist

- [ ] No hardcoded secrets in code or documentation
- [ ] All environment variables use placeholders in `.env.example`
- [ ] Database connections use SSL/TLS in production
- [ ] Audit logging captures all sensitive operations
- [ ] API authentication/authorization is enforced
- [ ] Dependency vulnerabilities are scanned (go mod audit)
- [ ] Secrets are never logged or exposed in error messages
- [ ] Data retention policies are documented and enforced
- [ ] Investigator access is case-gated and audited

---

## Questions?

For security-related questions or clarifications, please reach out to the maintainers privately.
