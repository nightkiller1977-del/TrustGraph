# TrustGraph ColdFusion Integration Guide

Integration reference for calling TrustGraph from ConnectionSphere (ColdFusion 2025).

---

## 1. Overview

TrustGraph is a Go trust-and-safety service that evaluates first-party signals
during user registration and returns a trust tier, risk score, and decision.
ConnectionSphere calls TrustGraph immediately after creating a user record so
that the platform can gate messaging, search visibility, and verification badges
based on assessed risk.

Key integration principles:

- **Fail-open**: if TrustGraph is unreachable, assign the `provisional` tier and
  backfill later. Registration must never be blocked by a TrustGraph outage.
- **Idempotent**: resending the same `idempotencyKey` within 24 hours returns the
  cached result, so retries are safe.
- **Fast**: the recommended timeout is 300 ms. TrustGraph is designed to respond
  well within that budget for Phase 1 (first-party signals only).

---

## 2. TrustGraphService.cfc

Complete ColdFusion component. Drop this into your `/services/` directory
(or wherever ConnectionSphere keeps service CFCs) and instantiate it once at
application startup.

> **ColdFusion `cfhttp` gotcha**: `cfhttp.statusCode` returns a string like
> `"200 OK"`, not the integer `200`. The code below uses
> `val(listFirst(cfhttp.statusCode, " "))` to extract the numeric status.
> Comparing directly to the integer `200` without this parse is the single most
> common ColdFusion HTTP integration bug.

```cfscript
/**
 * TrustGraphService.cfc
 *
 * ColdFusion 2025 client for the TrustGraph trust-and-safety API.
 * Provides registration assessment, circuit-breaker protection,
 * and fail-open provisional fallback.
 */
component accessors="true" {

    // ── Configuration properties ──────────────────────────────────────
    property name="baseURL"              type="string";
    property name="timeoutSeconds"       type="numeric";
    property name="maxFailures"          type="numeric";
    property name="failureWindowMinutes" type="numeric";
    property name="contractVersion"      type="string";
    property name="serviceAccount"       type="string";

    // ── Circuit-breaker state (application-scoped) ────────────────────
    // These are stored in the application scope so they survive across
    // requests but reset on application restart.

    /**
     * init()
     *
     * @baseURL               Root URL of the TrustGraph API (no trailing slash).
     *                        e.g. "https://trustgraph-api.onrender.com"
     * @timeoutSeconds        HTTP timeout in seconds. 0.3 (300 ms) recommended.
     * @maxFailures           Consecutive failures before the circuit opens.
     * @failureWindowMinutes  Rolling window for failure counting.
     * @contractVersion       API contract version string, e.g. "2026-08-01".
     * @serviceAccount        Value for the X-Service-Account header.
     */
    public TrustGraphService function init(
        required string baseURL,
        numeric timeoutSeconds        = 0.3,
        numeric maxFailures           = 5,
        numeric failureWindowMinutes  = 5,
        string  contractVersion       = "2026-08-01",
        string  serviceAccount        = "connectionsphere-prod"
    ) {
        variables.baseURL              = arguments.baseURL;
        variables.timeoutSeconds       = arguments.timeoutSeconds;
        variables.maxFailures          = arguments.maxFailures;
        variables.failureWindowMinutes = arguments.failureWindowMinutes;
        variables.contractVersion      = arguments.contractVersion;
        variables.serviceAccount       = arguments.serviceAccount;

        // Initialize circuit-breaker state in application scope if absent
        if ( !structKeyExists(application, "trustgraphFailures") ) {
            application.trustgraphFailures    = [];
            application.trustgraphCircuitOpen = false;
        }

        return this;
    }

    // ══════════════════════════════════════════════════════════════════
    //  PUBLIC: assessRegistration
    // ══════════════════════════════════════════════════════════════════

    /**
     * assessRegistration()
     *
     * Call this after the user record has been inserted but before the
     * user is allowed to send messages.
     *
     * @userId             ConnectionSphere user ID (e.g. "usr_12345").
     * @email              User's email address.
     * @phone              User's phone number in E.164 format.
     * @emailVerified      Whether the email has been verified.
     * @phoneVerified      Whether the phone has been verified.
     * @deviceFingerprint  Client-side device fingerprint token.
     * @ipAddress          Registering IP address.
     * @imageHash          Perceptual hash of the uploaded profile image.
     * @userAgent          The user's browser User-Agent string.
     *
     * @return struct  Assessment result with keys: assessmentId, status,
     *                 trustTier, riskBand, riskScore, reasonCodes,
     *                 decision, requiredActions, policyVersion,
     *                 completedAt, signals, isProvisionalFallback.
     */
    public struct function assessRegistration(
        required string userId,
        string  email             = "",
        string  phone             = "",
        boolean emailVerified     = false,
        boolean phoneVerified     = false,
        string  deviceFingerprint = "",
        string  ipAddress         = "",
        string  imageHash         = "",
        string  userAgent         = ""
    ) {
        // ── Short-circuit if the circuit breaker is open ──────────────
        if ( isCircuitOpen() ) {
            writeLog(
                text = "TrustGraph circuit breaker OPEN — returning provisional for user #arguments.userId#",
                type = "warning",
                log  = "trustgraph"
            );
            return buildProvisionalAssessment(arguments.userId, "CIRCUIT_OPEN");
        }

        // ── Build idempotency key (deterministic per registration attempt) ──
        var idempotencyKey = "reg-#arguments.userId#-#dateFormat(now(), 'yyyymmdd')#-#timeFormat(now(), 'HHmmss')#";

        // ── Build correlation ID for distributed tracing ──────────────
        var correlationId = "cs-reg-#arguments.userId#-#createUUID()#";

        // ── Assemble the request payload ──────────────────────────────
        var payload = {
            "contractVersion": variables.contractVersion,
            "idempotencyKey":  idempotencyKey,
            "subject": {
                "connectionSphereUserId": arguments.userId,
                "email":                  arguments.email,
                "phone":                  arguments.phone
            },
            "signals": {
                "emailVerified":     arguments.emailVerified,
                "phoneVerified":     arguments.phoneVerified,
                "deviceFingerprint": arguments.deviceFingerprint,
                "ipAddress":         arguments.ipAddress,
                "imageHash":         arguments.imageHash
            },
            "requestContext": {
                "userAgent":     arguments.userAgent,
                "correlationId": correlationId
            }
        };

        try {
            // ── Make the HTTP POST ────────────────────────────────────
            cfhttp(
                url     = "#variables.baseURL#/v1/assessments",
                method  = "POST",
                timeout = variables.timeoutSeconds,
                result  = "local.httpResult"
            ) {
                cfhttpparam(type="header", name="Content-Type",      value="application/json");
                cfhttpparam(type="header", name="Accept",            value="application/json");
                cfhttpparam(type="header", name="X-Service-Account", value=variables.serviceAccount);
                cfhttpparam(type="header", name="X-Correlation-ID",  value=correlationId);
                cfhttpparam(type="body",   value=serializeJSON(payload));
            }

            // ── Parse the status code ─────────────────────────────────
            // CRITICAL: ColdFusion returns statusCode as "200 OK", not 200.
            // You MUST extract the numeric portion or comparisons will fail.
            var httpStatus = val( listFirst(local.httpResult.statusCode, " ") );

            // ── Handle success ────────────────────────────────────────
            if ( httpStatus == 200 ) {
                var responseBody = deserializeJSON(local.httpResult.fileContent);

                // Reset the circuit breaker on success
                resetFailures();

                writeLog(
                    text = "TrustGraph assessment OK — user=#arguments.userId# tier=#responseBody.trustTier# score=#responseBody.riskScore#",
                    type = "information",
                    log  = "trustgraph"
                );

                // Attach a flag so callers know this is a real assessment
                responseBody["isProvisionalFallback"] = false;
                return responseBody;
            }

            // ── Handle non-200 responses ──────────────────────────────
            writeLog(
                text = "TrustGraph returned HTTP #httpStatus# for user #arguments.userId# — body: #local.httpResult.fileContent#",
                type = "error",
                log  = "trustgraph"
            );
            recordFailure();
            return buildProvisionalAssessment(arguments.userId, "HTTP_#httpStatus#");

        } catch (any e) {
            // ── Handle timeout and connection errors ──────────────────
            writeLog(
                text = "TrustGraph call failed for user #arguments.userId# — #e.message# (#e.type#)",
                type = "error",
                log  = "trustgraph"
            );
            recordFailure();
            return buildProvisionalAssessment(arguments.userId, "EXCEPTION_#e.type#");
        }
    }

    // ══════════════════════════════════════════════════════════════════
    //  PUBLIC: reassess
    // ══════════════════════════════════════════════════════════════════

    /**
     * reassess()
     *
     * Trigger a re-assessment for an existing user, e.g. after they verify
     * their email or phone. Uses a distinct idempotency key so TrustGraph
     * evaluates fresh signals.
     *
     * @userId             ConnectionSphere user ID.
     * @email              Current email.
     * @phone              Current phone.
     * @emailVerified      Current email verification status.
     * @phoneVerified      Current phone verification status.
     * @deviceFingerprint  Device fingerprint (may be empty for server-side calls).
     * @ipAddress          Current IP (may be empty for scheduled tasks).
     * @imageHash          Current profile image hash.
     * @trigger            What triggered the re-assessment (e.g. "email_verified").
     *
     * @return struct  Same shape as assessRegistration().
     */
    public struct function reassess(
        required string userId,
        string  email             = "",
        string  phone             = "",
        boolean emailVerified     = false,
        boolean phoneVerified     = false,
        string  deviceFingerprint = "",
        string  ipAddress         = "",
        string  imageHash         = "",
        string  trigger           = "manual"
    ) {
        // Re-assessment uses a unique idempotency key so it is never
        // collapsed with the original registration assessment.
        // We include the trigger and a UUID to guarantee uniqueness.
        var idempotencyKey = "reassess-#arguments.userId#-#arguments.trigger#-#createUUID()#";
        var correlationId  = "cs-reassess-#arguments.userId#-#createUUID()#";

        var payload = {
            "contractVersion": variables.contractVersion,
            "idempotencyKey":  idempotencyKey,
            "subject": {
                "connectionSphereUserId": arguments.userId,
                "email":                  arguments.email,
                "phone":                  arguments.phone
            },
            "signals": {
                "emailVerified":     arguments.emailVerified,
                "phoneVerified":     arguments.phoneVerified,
                "deviceFingerprint": arguments.deviceFingerprint,
                "ipAddress":         arguments.ipAddress,
                "imageHash":         arguments.imageHash
            },
            "requestContext": {
                "userAgent":     "ConnectionSphere-Server/Reassessment",
                "correlationId": correlationId
            }
        };

        try {
            if ( isCircuitOpen() ) {
                writeLog(
                    text = "TrustGraph circuit OPEN during reassessment — user=#arguments.userId# trigger=#arguments.trigger#",
                    type = "warning",
                    log  = "trustgraph"
                );
                return buildProvisionalAssessment(arguments.userId, "CIRCUIT_OPEN_REASSESS");
            }

            cfhttp(
                url     = "#variables.baseURL#/v1/assessments",
                method  = "POST",
                timeout = variables.timeoutSeconds,
                result  = "local.httpResult"
            ) {
                cfhttpparam(type="header", name="Content-Type",      value="application/json");
                cfhttpparam(type="header", name="Accept",            value="application/json");
                cfhttpparam(type="header", name="X-Service-Account", value=variables.serviceAccount);
                cfhttpparam(type="header", name="X-Correlation-ID",  value=correlationId);
                cfhttpparam(type="body",   value=serializeJSON(payload));
            }

            var httpStatus = val( listFirst(local.httpResult.statusCode, " ") );

            if ( httpStatus == 200 ) {
                var responseBody = deserializeJSON(local.httpResult.fileContent);
                resetFailures();

                writeLog(
                    text = "TrustGraph reassessment OK — user=#arguments.userId# trigger=#arguments.trigger# newTier=#responseBody.trustTier#",
                    type = "information",
                    log  = "trustgraph"
                );

                responseBody["isProvisionalFallback"] = false;
                return responseBody;
            }

            writeLog(
                text = "TrustGraph reassessment returned HTTP #httpStatus# for user #arguments.userId#",
                type = "error",
                log  = "trustgraph"
            );
            recordFailure();
            return buildProvisionalAssessment(arguments.userId, "REASSESS_HTTP_#httpStatus#");

        } catch (any e) {
            writeLog(
                text = "TrustGraph reassessment failed — user=#arguments.userId# — #e.message#",
                type = "error",
                log  = "trustgraph"
            );
            recordFailure();
            return buildProvisionalAssessment(arguments.userId, "REASSESS_EXCEPTION");
        }
    }

    // ══════════════════════════════════════════════════════════════════
    //  PRIVATE: Circuit breaker
    // ══════════════════════════════════════════════════════════════════

    /**
     * isCircuitOpen()
     *
     * Returns true if the number of failures within the configured rolling
     * window exceeds maxFailures. When the circuit is open, callers should
     * skip the HTTP call and return a provisional fallback immediately.
     */
    private boolean function isCircuitOpen() {
        // Prune failures older than the window
        var cutoff = dateAdd("n", -variables.failureWindowMinutes, now());
        var recent = application.trustgraphFailures.filter(function(ts) {
            return ts > cutoff;
        });
        application.trustgraphFailures = recent;

        var isOpen = recent.len() >= variables.maxFailures;
        application.trustgraphCircuitOpen = isOpen;
        return isOpen;
    }

    /**
     * recordFailure()
     *
     * Appends the current timestamp to the failure list.
     */
    private void function recordFailure() {
        application.trustgraphFailures.append(now());
    }

    /**
     * resetFailures()
     *
     * Clears the failure list after a successful call, closing the circuit.
     */
    private void function resetFailures() {
        application.trustgraphFailures    = [];
        application.trustgraphCircuitOpen = false;
    }

    // ══════════════════════════════════════════════════════════════════
    //  PRIVATE: Provisional fallback
    // ══════════════════════════════════════════════════════════════════

    /**
     * buildProvisionalAssessment()
     *
     * Constructs a synthetic assessment that mirrors the TrustGraph response
     * shape but assigns the user to the provisional tier. This is the
     * fail-open fallback — registration proceeds, and a background job
     * re-assesses the user once TrustGraph recovers.
     *
     * @userId  The ConnectionSphere user ID.
     * @reason  Why the fallback was triggered (for logging/debugging).
     */
    private struct function buildProvisionalAssessment(
        required string userId,
        string reason = "UNKNOWN"
    ) {
        return {
            "contractVersion":      variables.contractVersion,
            "assessmentId":         "provisional-#arguments.userId#-#createUUID()#",
            "status":               "deferred",
            "trustTier":            "provisional",
            "riskBand":             "unknown",
            "riskScore":            -1,
            "reasonCodes":          ["ASSESSMENT_UNAVAILABLE"],
            "decision":             "verify",
            "requiredActions":      ["VERIFY_EMAIL", "VERIFY_PHONE"],
            "policyVersion":        "fail-open-v1",
            "completedAt":          dateTimeFormat(now(), "yyyy-mm-dd'T'HH:nn:ss'Z'"),
            "signals": {
                "processed": [],
                "skipped":   ["ALL"]
            },
            "isProvisionalFallback": true,
            "fallbackReason":        arguments.reason
        };
    }
}
```

### Application.cfc initialization

Instantiate the service once in `onApplicationStart()`:

```cfscript
// In Application.cfc — onApplicationStart()
application.trustGraphService = new services.TrustGraphService().init(
    baseURL              = application.config.trustGraphURL,   // e.g. "https://trustgraph-api.onrender.com"
    timeoutSeconds       = 0.3,                                // 300 ms
    maxFailures          = 5,
    failureWindowMinutes = 5,
    contractVersion      = "2026-08-01",
    serviceAccount       = "connectionsphere-prod"
);
```

---

## 3. Registration Integration

### Where to call TrustGraph

Call `assessRegistration()` **after** the user record is created in the database
but **before** the user gains messaging capability. The sequence is:

```
1. Validate registration form
2. INSERT user record (status = "active", trust_tier = NULL)
3. ──> Call TrustGraph assessRegistration() <──
4. UPDATE user SET trust_tier = result.trustTier, trust_assessment_id = result.assessmentId
5. If provisional: flag for "verification recommended" banner
6. If limited:     flag for account restriction
7. Redirect to profile completion / dashboard
```

### Controller code

```cfscript
// In your registration controller / handler, after the user INSERT succeeds:

// Gather signals available at registration time
var assessment = application.trustGraphService.assessRegistration(
    userId            = newUser.userId,
    email             = form.email,
    phone             = form.phone,
    emailVerified     = false,        // not yet verified at registration
    phoneVerified     = false,
    deviceFingerprint = cookie.deviceFP ?: "",
    ipAddress         = cgi.REMOTE_ADDR,
    imageHash         = local.profileImageHash ?: "",
    userAgent         = cgi.HTTP_USER_AGENT
);

// Persist the trust tier on the user record
queryExecute(
    "UPDATE users
        SET trust_tier         = :trustTier,
            trust_assessment_id = :assessmentId,
            trust_updated_at    = GETDATE()
      WHERE user_id = :userId",
    {
        trustTier:    { value: assessment.trustTier,   cfsqltype: "cf_sql_varchar" },
        assessmentId: { value: assessment.assessmentId, cfsqltype: "cf_sql_varchar" },
        userId:       { value: newUser.userId,          cfsqltype: "cf_sql_varchar" }
    }
);

// If TrustGraph was unreachable, queue a backfill job
if ( assessment.isProvisionalFallback ) {
    application.trustGraphBackfillService.queueBackfill(
        userId = newUser.userId,
        reason = assessment.fallbackReason
    );
}

// Store the assessment result in the session for immediate UI use
session.trustTier       = assessment.trustTier;
session.trustDecision   = assessment.decision;
session.requiredActions = assessment.requiredActions ?: [];
```

### Database columns

Add these columns to your `users` table if not already present:

```sql
ALTER TABLE users ADD trust_tier          VARCHAR(20) DEFAULT 'provisional';
ALTER TABLE users ADD trust_assessment_id VARCHAR(100);
ALTER TABLE users ADD trust_updated_at    DATETIME;
```

---

## 4. Trust Tier Capability Gates

Use the `trust_tier` stored on the user record to gate features throughout the
application.

### Tier summary

| Tier | Browse profiles | Send messages | Message new users | Verified badge | Search priority | Restrictions |
|------|:-:|:-:|:-:|:-:|:-:|---|
| **provisional** | Yes | Limited | No | No | Normal | "Verification recommended" banner |
| **standard** | Yes | Yes | Yes | No | Normal | None |
| **elevated** | Yes | Yes | Yes | Yes | Boosted | None |
| **limited** | Yes | No | No | No | Suppressed | Account restriction banner; human review required |

### Gate helper (UserCapabilityService.cfc)

```cfscript
/**
 * UserCapabilityService.cfc
 *
 * Centralizes trust-tier capability checks so they are consistent
 * across controllers, views, and API endpoints.
 */
component {

    /**
     * canSendMessage()
     *
     * Returns true if the user is allowed to send a message to the
     * given recipient. Provisional users can only reply to conversations
     * initiated by standard or elevated users — they cannot message
     * someone new.
     */
    public boolean function canSendMessage(
        required string senderTier,
        required boolean isExistingConversation
    ) {
        switch ( arguments.senderTier ) {
            case "elevated":
            case "standard":
                return true;

            case "provisional":
                // Provisional users can reply but cannot start new conversations
                return arguments.isExistingConversation;

            case "limited":
                // Limited users cannot message at all
                return false;

            default:
                // Unknown tier — treat as limited for safety
                return false;
        }
    }

    /**
     * canBrowseProfiles()
     *
     * All tiers can browse. This exists so the gate is explicit.
     */
    public boolean function canBrowseProfiles(required string tier) {
        return true;
    }

    /**
     * hasVerifiedBadge()
     */
    public boolean function hasVerifiedBadge(required string tier) {
        return arguments.tier == "elevated";
    }

    /**
     * getSearchBoost()
     *
     * Returns a multiplier for search ranking.
     *   elevated   = 1.2  (boosted)
     *   standard   = 1.0  (normal)
     *   provisional = 1.0  (normal)
     *   limited    = 0.5  (suppressed)
     */
    public numeric function getSearchBoost(required string tier) {
        switch ( arguments.tier ) {
            case "elevated":    return 1.2;
            case "standard":    return 1.0;
            case "provisional": return 1.0;
            case "limited":     return 0.5;
            default:            return 1.0;
        }
    }

    /**
     * getUIBanner()
     *
     * Returns a struct describing a banner to show, or an empty struct
     * if no banner is needed.
     */
    public struct function getUIBanner(required string tier) {
        switch ( arguments.tier ) {
            case "provisional":
                return {
                    "type":    "info",
                    "message": "Complete verification to unlock full messaging. Verify your email and phone to get started.",
                    "action":  "verify",
                    "show":    true
                };

            case "limited":
                return {
                    "type":    "warning",
                    "message": "Your account is under review. Some features are temporarily restricted.",
                    "action":  "none",
                    "show":    true
                };

            default:
                return { "show": false };
        }
    }

    /**
     * requiresHumanReview()
     */
    public boolean function requiresHumanReview(required string tier) {
        return arguments.tier == "limited";
    }
}
```

### Usage in a messaging controller

```cfscript
// Before sending a message:
var capService = new services.UserCapabilityService();

if ( !capService.canSendMessage(session.trustTier, local.isExistingConversation) ) {
    if ( session.trustTier == "provisional" ) {
        // Tell the user they need to verify before messaging new people
        rc.errorMessage = "Complete email or phone verification to message new users.";
    } else {
        // limited tier
        rc.errorMessage = "Your account is currently under review. Messaging is temporarily unavailable.";
    }
    return variables.fw.redirect("messages.inbox");
}
```

### Usage in a view template

```cfscript
// In a layout or view .cfm file:
var banner = application.capabilityService.getUIBanner(session.trustTier);

if ( banner.show ) {
    writeOutput('
        <div class="trust-banner trust-banner--#banner.type#">
            <p>#banner.message#</p>
            #banner.action == "verify"
                ? '<a href="/account/verify" class="btn btn--verify">Verify Now</a>'
                : ''#
        </div>
    ');
}
```

---

## 5. Background Reassessment

When a user completes a verification step after registration, trigger a
reassessment so their trust tier can be upgraded.

### Trigger points

| Event | Where to call | Trigger value |
|---|---|---|
| User verifies email | Email verification callback handler | `"email_verified"` |
| User verifies phone | Phone/SMS verification callback handler | `"phone_verified"` |
| User completes profile (photo, bio) | Profile update controller | `"profile_completed"` |
| Admin manually triggers | Admin panel action | `"admin_manual"` |

### Reassessment call

```cfscript
// Example: after email verification succeeds

// 1. Mark the email as verified in your local DB
queryExecute(
    "UPDATE users SET email_verified = 1 WHERE user_id = :userId",
    { userId: { value: userId, cfsqltype: "cf_sql_varchar" } }
);

// 2. Trigger TrustGraph reassessment with updated signals
var newAssessment = application.trustGraphService.reassess(
    userId        = userId,
    email         = user.email,
    phone         = user.phone,
    emailVerified = true,
    phoneVerified = user.phoneVerified,
    imageHash     = user.profileImageHash ?: "",
    trigger       = "email_verified"
);

// 3. Update the user's trust tier if it changed
if ( newAssessment.trustTier != user.trust_tier ) {
    queryExecute(
        "UPDATE users
            SET trust_tier          = :newTier,
                trust_assessment_id = :assessmentId,
                trust_updated_at    = GETDATE()
          WHERE user_id = :userId",
        {
            newTier:      { value: newAssessment.trustTier,   cfsqltype: "cf_sql_varchar" },
            assessmentId: { value: newAssessment.assessmentId, cfsqltype: "cf_sql_varchar" },
            userId:       { value: userId,                     cfsqltype: "cf_sql_varchar" }
        }
    );

    writeLog(
        text = "Trust tier changed for user #userId#: #user.trust_tier# -> #newAssessment.trustTier# (trigger: email_verified)",
        type = "information",
        log  = "trustgraph"
    );
}
```

### Scheduled backfill task

Create a ColdFusion scheduled task that retries assessments for users who
received a provisional fallback because TrustGraph was unavailable.

```cfscript
/**
 * TrustGraphBackfillTask.cfc
 *
 * Scheduled task that runs every 15 minutes. Finds users whose trust
 * assessment was a provisional fallback and retries the assessment.
 */
component {

    public void function execute() {
        // Find users with deferred/fallback assessments
        var pendingUsers = queryExecute(
            "SELECT user_id, email, phone, email_verified, phone_verified,
                    profile_image_hash
               FROM users
              WHERE trust_tier = 'provisional'
                AND trust_assessment_id LIKE 'provisional-%'
              ORDER BY created_at ASC
              LIMIT 50",
            {},
            { returntype: "array" }
        );

        if ( pendingUsers.len() == 0 ) {
            return;
        }

        writeLog(
            text = "TrustGraph backfill: processing #pendingUsers.len()# deferred assessments",
            type = "information",
            log  = "trustgraph"
        );

        var trustGraph = application.trustGraphService;

        for ( var user in pendingUsers ) {
            try {
                var result = trustGraph.reassess(
                    userId        = user.user_id,
                    email         = user.email,
                    phone         = user.phone ?: "",
                    emailVerified = user.email_verified ?: false,
                    phoneVerified = user.phone_verified ?: false,
                    imageHash     = user.profile_image_hash ?: "",
                    trigger       = "backfill"
                );

                // Only update if we got a real assessment (not another fallback)
                if ( !result.isProvisionalFallback ) {
                    queryExecute(
                        "UPDATE users
                            SET trust_tier          = :tier,
                                trust_assessment_id = :assessmentId,
                                trust_updated_at    = GETDATE()
                          WHERE user_id = :userId",
                        {
                            tier:         { value: result.trustTier,   cfsqltype: "cf_sql_varchar" },
                            assessmentId: { value: result.assessmentId, cfsqltype: "cf_sql_varchar" },
                            userId:       { value: user.user_id,       cfsqltype: "cf_sql_varchar" }
                        }
                    );

                    writeLog(
                        text = "Backfill upgraded user #user.user_id# to tier=#result.trustTier#",
                        type = "information",
                        log  = "trustgraph"
                    );
                }
            } catch (any e) {
                // Log and continue — don't let one failure stop the batch
                writeLog(
                    text = "Backfill failed for user #user.user_id# — #e.message#",
                    type = "error",
                    log  = "trustgraph"
                );
            }
        }
    }
}
```

Register this as a ColdFusion scheduled task in the CF Admin or via
`cfschedule`:

```cfscript
cfschedule(
    action    = "update",
    task      = "TrustGraphBackfill",
    operation = "HTTPRequest",
    url       = "https://your-app.com/scheduled/trustgraph-backfill",
    startDate = "2026-08-01",
    startTime = "00:00",
    interval  = 900,  // 900 seconds = 15 minutes
    resolveURL = true
);
```

---

## 6. Error Handling

### Design principle: never block signup

TrustGraph is an advisory service. If it is down, degraded, or slow, the user
must still be able to register. The application assigns the `provisional` tier
and catches up later.

### Failure scenarios and responses

| Scenario | What happens | User experience |
|---|---|---|
| TrustGraph returns 200 | Trust tier assigned from response | Normal |
| TrustGraph returns 4xx/5xx | `recordFailure()` called; provisional tier assigned | "Verification recommended" banner |
| TrustGraph times out (>300 ms) | ColdFusion `cfhttp` throws; caught in `catch` block; provisional assigned | Same as above |
| Circuit breaker open (5+ failures in 5 min) | HTTP call skipped entirely; provisional returned immediately | Same as above |
| TrustGraph DNS failure | ColdFusion throws connection error; caught; provisional assigned | Same as above |
| Response body is not valid JSON | `deserializeJSON` throws; caught; provisional assigned | Same as above |

### Logging

All TrustGraph interactions are logged to the `trustgraph` log file
(`{cf-home}/logs/trustgraph.log`). Log entries include:

- Every successful assessment (tier, score, user ID)
- Every failure (HTTP status, error message, user ID)
- Circuit breaker state changes (open/closed)
- Backfill task progress

### Monitoring checklist

Set up alerts on these conditions:

1. **Circuit breaker opened** — grep for `"circuit breaker OPEN"` in
   `trustgraph.log`. This means TrustGraph has been failing repeatedly.
2. **High provisional fallback rate** — query your `users` table:
   ```sql
   SELECT COUNT(*) AS fallback_count
     FROM users
    WHERE trust_assessment_id LIKE 'provisional-%'
      AND created_at > DATEADD(hour, -1, GETDATE());
   ```
   Alert if this exceeds 10% of hourly registrations.
3. **Backfill queue depth** — same query but without the time filter shows
   total unresolved provisional assessments.
4. **TrustGraph latency** — if you add timing around the `cfhttp` call,
   alert if p95 exceeds 200 ms (the 300 ms timeout leaves only 100 ms of
   margin).

### Idempotency safety

If the user double-submits the registration form, the same `idempotencyKey`
(based on user ID and timestamp) will be sent to TrustGraph. TrustGraph returns
the cached result within 24 hours, so:

- No duplicate assessments are created.
- No extra cost is incurred.
- The trust tier is consistent regardless of retries.

---

## Appendix: Quick Reference

### Environment configuration

| Setting | Dev | Staging | Production |
|---|---|---|---|
| `trustGraphURL` | `http://localhost:8081` | `https://trustgraph-staging.onrender.com` | `https://trustgraph-api.onrender.com` |
| `timeoutSeconds` | `2` | `0.5` | `0.3` |
| `maxFailures` | `100` | `10` | `5` |
| `failureWindowMinutes` | `60` | `10` | `5` |
| `serviceAccount` | `connectionsphere-dev` | `connectionsphere-staging` | `connectionsphere-prod` |

### TrustGraph API quick reference

```
POST /v1/assessments
Content-Type: application/json
X-Service-Account: connectionsphere-prod

{
  "contractVersion": "2026-08-01",
  "idempotencyKey":  "reg-usr_12345-20260803-142030",
  "subject": {
    "connectionSphereUserId": "usr_12345",
    "email": "user@example.com",
    "phone": "+15551234567"
  },
  "signals": {
    "emailVerified": true,
    "phoneVerified": false,
    "deviceFingerprint": "fp_a1b2c3d4",
    "ipAddress": "192.168.1.100",
    "imageHash": ""
  },
  "requestContext": {
    "userAgent": "Mozilla/5.0 ...",
    "correlationId": "cs-reg-usr_12345-..."
  }
}
```

Response (200):

```json
{
  "contractVersion": "2026-08-01",
  "assessmentId": "a1b2c3d4-...",
  "status": "complete",
  "trustTier": "provisional",
  "riskBand": "elevated",
  "riskScore": 45,
  "reasonCodes": ["EMAIL_VERIFIED", "PHONE_NOT_VERIFIED", "DEVICE_FIRST_SEEN"],
  "decision": "verify",
  "requiredActions": ["VERIFY_PHONE"],
  "policyVersion": "registration-v1",
  "completedAt": "2026-08-03T14:20:31Z",
  "signals": {
    "processed": ["email", "phone", "device", "velocity"],
    "skipped": []
  }
}
```

### ColdFusion status code parsing reminder

```cfscript
// WRONG — will never match because statusCode is "200 OK"
if ( cfhttp.statusCode == 200 ) { ... }

// RIGHT — extract the numeric portion first
var httpStatus = val( listFirst(cfhttp.statusCode, " ") );
if ( httpStatus == 200 ) { ... }
```
