//go:build integration

// Package store integration tests run against a real PostgreSQL instance.
//
// They are gated twice: by the `integration` build tag and by TEST_DATABASE_URL.
// Without the env var they skip, so `go test ./...` stays green on a machine
// with no database, while CI (or a developer) can run:
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration ./internal/store/...
//
// The schema is created by running the real migrations, not a hand-written
// fixture, so a migration that does not match the repository code fails here.
package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/verification"
)

func testDB(t *testing.T) *PostgresDB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping store integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := NewPostgres(ctx, dsn)
	require.NoError(t, err, "connect to test database")
	t.Cleanup(func() { _ = db.Close() })

	migrationsDir := filepath.Join("..", "..", "migrations")
	require.NoError(t, RunMigrations(db, migrationsDir), "run migrations")

	// Each test starts from a clean slate. Drop-and-recreate the schema rather
	// than TRUNCATE-ing a hard-coded table list: the list would silently fall
	// out of date as migrations add tables, and a stale list can leave data
	// behind between tests. schema_migrations is repopulated by RunMigrations
	// in each test's setup.
	if _, err := db.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		require.NoError(t, err, "reset schema")
	}
	require.NoError(t, RunMigrations(db, migrationsDir), "re-run migrations after reset")

	return db
}

func TestIntegration_MigrationsApplyCleanly(t *testing.T) {
	db := testDB(t)

	// Re-running migrations must be a no-op, and every migration file should be
	// recorded. This catches a file that fails to parse, a version that
	// collides, or an applied-marker bug.
	require.NoError(t, RunMigrations(db, filepath.Join("..", "..", "migrations")))

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count))
	assert.GreaterOrEqual(t, count, 7, "expected at least the seven shipped migrations")
}

func TestIntegration_SubjectFindOrCreateIsIdempotent(t *testing.T) {
	db := testDB(t)
	repo := NewSubjectRepository(db)
	ctx := context.Background()

	first, err := repo.FindOrCreateSubject(ctx, "cs_user_integration_1")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, first)

	second, err := repo.FindOrCreateSubject(ctx, "cs_user_integration_1")
	require.NoError(t, err)
	assert.Equal(t, first, second, "repeated calls must return the same subject")

	fetched, err := repo.GetSubjectByCSUserID(ctx, "cs_user_integration_1")
	require.NoError(t, err)
	assert.Equal(t, first, fetched)

	missing, err := repo.GetSubjectByCSUserID(ctx, "cs_user_does_not_exist")
	require.NoError(t, err)
	assert.Equal(t, uuid.Nil, missing)
}

func TestIntegration_ConsentLifecycle(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	consents := NewConsentRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_consent")
	require.NoError(t, err)

	// No consent yet: the guard must refuse.
	err = consents.RequireConsent(ctx, subjectID, "B", models.ConsentTypeLinkedIn)
	assert.ErrorIs(t, err, ErrConsentRequired)

	_, err = consents.GrantConsent(ctx, subjectID, "B", models.ConsentTypeLinkedIn, "policy-v1", nil)
	require.NoError(t, err)
	require.NoError(t, consents.RequireConsent(ctx, subjectID, "B", models.ConsentTypeLinkedIn))

	// A different consent type is still ungranted.
	err = consents.RequireConsent(ctx, subjectID, "B", models.ConsentTypeGovernmentID)
	assert.ErrorIs(t, err, ErrConsentRequired)

	_, err = consents.WithdrawConsentAndEraseData(ctx, subjectID, "B", models.ConsentTypeLinkedIn)
	require.NoError(t, err)
	err = consents.RequireConsent(ctx, subjectID, "B", models.ConsentTypeLinkedIn)
	assert.ErrorIs(t, err, ErrConsentRequired, "withdrawn consent must stop being usable")
}

func TestIntegration_WithdrawConsentDeletesPlaneBData(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	consents := NewConsentRepository(db)
	linkedin := NewLinkedInRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_delete")
	require.NoError(t, err)
	_, err = consents.GrantConsent(ctx, subjectID, "B", models.ConsentTypeLinkedIn, "policy-v1", nil)
	require.NoError(t, err)

	profile := &LinkedInProfile{
		SubjectID:  subjectID,
		LinkedInID: "li-123",
		Email:      "user@example.com",
		FirstName:  "Jane",
		LastName:   "Doe",
		Headline:   "Engineer",
	}
	require.NoError(t, linkedin.UpsertProfile(ctx, profile, nil))

	// The profile is readable while consent holds.
	stored, err := linkedin.GetProfileBySubject(ctx, subjectID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, "li-123", stored.LinkedInID)

	// Employment is also Plane B data and must be removed by the same call.
	employment := NewEmploymentRepository(db)
	require.NoError(t, employment.ReplaceEmploymentForSubject(ctx, subjectID, []SubjectEmployment{{
		CompanyName: "Acme", Title: "Engineer", StartDate: time.Now().AddDate(-1, 0, 0),
		IsCurrent: true, Source: "linkedin",
	}}))

	require.NoError(t, consents.DeleteSubjectPlaneBData(ctx, subjectID))

	// After deletion the PII must be gone, not merely marked withdrawn. This is
	// the regression guard for a partial erase that leaves LinkedIn data behind.
	stored, err = linkedin.GetProfileBySubject(ctx, subjectID)
	require.NoError(t, err)
	assert.Nil(t, stored, "LinkedIn profile should be deleted with consent withdrawal")

	jobs, err := employment.ListBySubject(ctx, subjectID)
	require.NoError(t, err)
	assert.Empty(t, jobs, "employment history should be deleted with consent withdrawal")
}

func TestIntegration_VerificationBadgeState(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	verifications := NewVerificationRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_verify")
	require.NoError(t, err)

	flags, err := verifications.GetSubjectVerificationFlags(ctx, subjectID)
	require.NoError(t, err)
	assert.False(t, flags.HasGovernmentID)
	assert.False(t, flags.HasLiveness)

	require.NoError(t, verifications.MarkVerified(ctx, subjectID, models.VerificationTypeGovernmentID))

	flags, err = verifications.GetSubjectVerificationFlags(ctx, subjectID)
	require.NoError(t, err)
	assert.True(t, flags.HasGovernmentID)
	assert.False(t, flags.HasLiveness, "liveness must be independent of government ID")
}

func TestIntegration_InvestigationCaseLifecycle(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	cases := NewInvestigationRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_case")
	require.NoError(t, err)

	created, err := cases.CreateCase(ctx, models.CaseCreateRequest{
		SubjectID: subjectID.String(),
		Title:     "Integration case",
		Priority:  models.CasePriorityHigh,
	}, "investigator@example.com")
	require.NoError(t, err)
	assert.Equal(t, models.CaseStatusOpen, created.Status)
	assert.Equal(t, models.CasePriorityHigh, created.Priority)
	assert.Regexp(t, `^INV-\d+$`, created.CaseNumber)

	caseID, err := uuid.Parse(created.CaseID)
	require.NoError(t, err)

	note, err := cases.AddNote(ctx, caseID, "investigator@example.com", "investigator", "note", "first observation")
	require.NoError(t, err)
	assert.NotEmpty(t, note.NoteID)

	fetched, err := cases.GetCase(ctx, caseID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Len(t, fetched.Notes, 1)
	assert.Equal(t, "first observation", fetched.Notes[0].Content)

	// Resolve the case and confirm the close timestamp is stamped.
	resolved := models.CaseStatusResolved
	updated, err := cases.UpdateCase(ctx, caseID, models.CaseUpdateRequest{
		Status:     &resolved,
		Resolution: strPtr("no_violation"),
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, models.CaseStatusResolved, updated.Status)
	assert.NotNil(t, updated.ClosedAt, "resolving a case must stamp closed_at")
}

func TestIntegration_CaseNumberAllocationIsUnique(t *testing.T) {
	db := testDB(t)
	cases := NewInvestigationRepository(db)
	ctx := context.Background()

	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		created, err := cases.CreateCase(ctx, models.CaseCreateRequest{Title: "concurrent"}, "investigator@example.com")
		require.NoError(t, err)
		require.False(t, seen[created.CaseNumber], "case number %q was handed out twice", created.CaseNumber)
		seen[created.CaseNumber] = true
	}
}

func TestIntegration_InvestigationAccessLog(t *testing.T) {
	db := testDB(t)
	cases := NewInvestigationRepository(db)
	ctx := context.Background()

	created, err := cases.CreateCase(ctx, models.CaseCreateRequest{Title: "access log case"}, "investigator@example.com")
	require.NoError(t, err)
	caseID, err := uuid.Parse(created.CaseID)
	require.NoError(t, err)

	require.NoError(t, cases.RecordAccess(ctx, &AccessLogEntry{
		CaseID: &caseID, Actor: "investigator@example.com", ActorRole: "investigator",
		Action: "case.view", Granted: true,
	}))
	require.NoError(t, cases.RecordAccess(ctx, &AccessLogEntry{
		CaseID: &caseID, Actor: "investigator@example.com", ActorRole: "investigator",
		Action: "break_glass.granted", Granted: true, BreakGlass: true, Reason: "emergency",
	}))

	entries, err := cases.ListAccessLog(ctx, caseID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	// Newest first: the break-glass row was written last.
	assert.True(t, entries[0].BreakGlass)
	assert.Equal(t, "case.view", entries[1].Action)
}

func TestIntegration_BreakGlassGrantExpires(t *testing.T) {
	db := testDB(t)
	cases := NewInvestigationRepository(db)
	ctx := context.Background()

	// A one-second grant, then wait it out.
	grant, err := cases.CreateBreakGlassGrant(ctx, "investigator@example.com", "investigator",
		"integration test justification", nil, nil, time.Second)
	require.NoError(t, err)
	assert.NotEmpty(t, grant.GrantID)

	active, err := cases.ActiveBreakGlassGrant(ctx, "investigator@example.com")
	require.NoError(t, err)
	require.NotNil(t, active)

	time.Sleep(1100 * time.Millisecond)

	active, err = cases.ActiveBreakGlassGrant(ctx, "investigator@example.com")
	require.NoError(t, err)
	assert.Nil(t, active, "an expired grant must not be returned as active")
}

func TestIntegration_EmploymentRoundTrip(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	employment := NewEmploymentRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_employment")
	require.NoError(t, err)

	records := []SubjectEmployment{{
		CompanyName: "Acme Corp",
		Title:       "Engineer",
		StartDate:   time.Now().AddDate(-2, 0, 0),
		IsCurrent:   true,
		Source:      "linkedin",
	}}
	require.NoError(t, employment.ReplaceEmploymentForSubject(ctx, subjectID, records))

	stored, err := employment.ListBySubject(ctx, subjectID)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, "Acme Corp", stored[0].CompanyName)
}

func TestIntegration_ScopedEraseLeavesOtherPurposesIntact(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	consents := NewConsentRepository(db)
	linkedin := NewLinkedInRepository(db)
	verifications := NewVerificationRepository(db)
	employment := NewEmploymentRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_scoped")
	require.NoError(t, err)

	// Grant two independent purposes and populate both.
	_, err = consents.GrantConsent(ctx, subjectID, "B", models.ConsentTypeLinkedIn, "policy-v1", nil)
	require.NoError(t, err)
	_, err = consents.GrantConsent(ctx, subjectID, "B", models.ConsentTypeImageVerification, "policy-v1", nil)
	require.NoError(t, err)

	require.NoError(t, linkedin.UpsertProfile(ctx, &LinkedInProfile{
		SubjectID: subjectID, LinkedInID: "li-scoped", FirstName: "Jane",
	}, nil))
	require.NoError(t, employment.ReplaceEmploymentForSubject(ctx, subjectID, []SubjectEmployment{{
		CompanyName: "Acme", Title: "Engineer", StartDate: time.Now().AddDate(-1, 0, 0), Source: "linkedin",
	}}))
	require.NoError(t, verifications.RecordImageVerification(ctx, &ImageVerificationRecord{
		SubjectID: subjectID, ImageHash: "hash-scoped", ReverseProvider: "tineye",
	}))

	// Withdraw only the image purpose, mirroring the handler call: the status
	// flip and the data erase happen in one transaction.
	_, err = consents.WithdrawConsentAndEraseData(ctx, subjectID, "B", models.ConsentTypeImageVerification)
	require.NoError(t, err)

	var imageRows int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM image_verification WHERE subject_id = $1`, subjectID).Scan(&imageRows))
	assert.Equal(t, 0, imageRows, "image consent withdrawal must erase image verification rows")

	// LinkedIn data was collected under a different consent and must survive.
	profile, err := linkedin.GetProfileBySubject(ctx, subjectID)
	require.NoError(t, err)
	assert.NotNil(t, profile, "withdrawing image consent must not erase LinkedIn data")

	jobs, err := employment.ListBySubject(ctx, subjectID)
	require.NoError(t, err)
	assert.Len(t, jobs, 1, "withdrawing image consent must not erase employment history")

	// Both consent rows survive as an audit trail of what was granted and
	// withdrawn; only the image row's status changes.
	remaining, err := consents.ListBySubject(ctx, subjectID)
	require.NoError(t, err)
	require.Len(t, remaining, 2)

	byType := map[string]models.SubjectConsent{}
	for _, c := range remaining {
		byType[c.ConsentType] = c
	}
	assert.Equal(t, models.ConsentStatusWithdrawn, byType[models.ConsentTypeImageVerification].ConsentStatus)
	assert.Equal(t, models.ConsentStatusGranted, byType[models.ConsentTypeLinkedIn].ConsentStatus,
		"an unrelated consent must remain granted")
}

func TestIntegration_ScopedEraseRefreshesFlagsFromSurvivors(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	consents := NewConsentRepository(db)
	verifications := NewVerificationRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_flags")
	require.NoError(t, err)

	_, err = consents.GrantConsent(ctx, subjectID, "B", models.ConsentTypeGovernmentID, "policy-v1", nil)
	require.NoError(t, err)
	require.NoError(t, verifications.RecordGovernmentID(ctx, subjectID, &verification.IDVerificationResult{
		Status: models.VerificationStatusVerified, VendorReference: "inq_flags",
	}, "persona"))
	require.NoError(t, verifications.MarkVerified(ctx, subjectID, models.VerificationTypeGovernmentID))

	flags, err := verifications.GetSubjectVerificationFlags(ctx, subjectID)
	require.NoError(t, err)
	require.True(t, flags.HasGovernmentID)

	// Withdrawing an unrelated purpose must leave the government-ID badge up.
	_, err = consents.GrantConsent(ctx, subjectID, "B", models.ConsentTypeImageVerification, "policy-v1", nil)
	require.NoError(t, err)
	_, err = consents.WithdrawConsentAndEraseData(ctx, subjectID, "B", models.ConsentTypeImageVerification)
	require.NoError(t, err)

	flags, err = verifications.GetSubjectVerificationFlags(ctx, subjectID)
	require.NoError(t, err)
	assert.True(t, flags.HasGovernmentID, "unrelated withdrawal must not un-verify a government ID")

	// Withdrawing the government-ID purpose itself must clear the badge.
	_, err = consents.WithdrawConsentAndEraseData(ctx, subjectID, "B", models.ConsentTypeGovernmentID)
	require.NoError(t, err)

	flags, err = verifications.GetSubjectVerificationFlags(ctx, subjectID)
	require.NoError(t, err)
	assert.False(t, flags.HasGovernmentID, "withdrawing government-ID consent must clear the badge")
}

func TestIntegration_ScopedEraseRejectsUnknownConsentType(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	consents := NewConsentRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_unknown")
	require.NoError(t, err)

	// An unrecognised purpose must fail loudly rather than fall through to a
	// full erase.
	err = consents.DeleteSubjectPlaneBDataForConsent(ctx, subjectID, "not_a_real_purpose")
	require.Error(t, err)
}

func strPtr(s string) *string { return &s }

// TestIntegration_WithdrawRetryAfterPartialFailureErasesData covers the retry
// case the atomic withdrawal exists to support: if the data erase failed after
// the status flip, a second call must still remove the data rather than
// returning "no active consent" and leaving PII behind.
func TestIntegration_WithdrawRetryErasesData(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	consents := NewConsentRepository(db)
	verifications := NewVerificationRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_retry")
	require.NoError(t, err)

	_, err = consents.GrantConsent(ctx, subjectID, "B", models.ConsentTypeImageVerification, "policy-v1", nil)
	require.NoError(t, err)
	require.NoError(t, verifications.RecordImageVerification(ctx, &ImageVerificationRecord{
		SubjectID: subjectID, ImageHash: "hash-retry", ReverseProvider: "tineye",
	}))

	// First withdrawal flips the status and erases in one transaction.
	_, err = consents.WithdrawConsentAndEraseData(ctx, subjectID, "B", models.ConsentTypeImageVerification)
	require.NoError(t, err)

	// Simulate the state a partial failure would have left: consent withdrawn but
	// data still present.
	require.NoError(t, verifications.RecordImageVerification(ctx, &ImageVerificationRecord{
		SubjectID: subjectID, ImageHash: "hash-retry-2", ReverseProvider: "tineye",
	}))

	consent, err := consents.WithdrawConsentAndEraseData(ctx, subjectID, "B", models.ConsentTypeImageVerification)
	require.NoError(t, err)
	require.NotNil(t, consent, "an already-withdrawn consent must still be erasable, not a 404")

	var rows int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM image_verification WHERE subject_id = $1`, subjectID).Scan(&rows))
	assert.Equal(t, 0, rows, "a retried withdrawal must erase data left behind by a partial failure")
}

// TestIntegration_WithdrawLinkedInErasesEducation pins the purpose-scoped erase
// for data sourced from LinkedIn OAuth: employment and education both go, and
// the education badge cannot survive the withdrawal that supposedly deleted it.
func TestIntegration_WithdrawLinkedInErasesEducation(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	consents := NewConsentRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_edu_erase")
	require.NoError(t, err)
	_, err = consents.GrantConsent(ctx, subjectID, "B", models.ConsentTypeLinkedIn, "policy-v1", nil)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO subject_education (subject_id, school_name, field_of_study) VALUES ($1, $2, $3)`,
		subjectID, "Example University", "Computer Science")
	require.NoError(t, err)

	_, err = consents.WithdrawConsentAndEraseData(ctx, subjectID, "B", models.ConsentTypeLinkedIn)
	require.NoError(t, err)

	var eduRows int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM subject_education WHERE subject_id = $1`, subjectID).Scan(&eduRows))
	assert.Equal(t, 0, eduRows, "LinkedIn withdrawal must erase education sourced from the OAuth profile")
}

// TestIntegration_ConsentListReturnsTerms covers the contract that the consent
// list and status reads expose the terms that were accepted, not just the grant
// response.
func TestIntegration_ConsentListReturnsTerms(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	consents := NewConsentRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_terms")
	require.NoError(t, err)

	terms := map[string]interface{}{"tosVersion": "2.1", "privacyVersion": "1.4"}
	_, err = consents.GrantConsent(ctx, subjectID, "B", models.ConsentTypeGovernmentID, "policy-v1", terms)
	require.NoError(t, err)

	listed, err := consents.ListBySubject(ctx, subjectID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.NotNil(t, listed[0].TermsAccepted, "the terms accepted must be returned by the list query")
	assert.Equal(t, "2.1", listed[0].TermsAccepted["tosVersion"])
	assert.Equal(t, "1.4", listed[0].TermsAccepted["privacyVersion"])
}

// TestIntegration_ReopenCaseClearsClosedAt guards the lifecycle timestamp: a
// reopened case must not still look closed at an earlier time.
func TestIntegration_ReopenCaseClearsClosedAt(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	cases := NewInvestigationRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_reopen")
	require.NoError(t, err)

	created, err := cases.CreateCase(ctx, models.CaseCreateRequest{
		SubjectID: subjectID.String(),
		Title:     "Reopen me",
		Priority:  models.CasePriorityNormal,
	}, "investigator@example.com")
	require.NoError(t, err)

	caseID, err := uuid.Parse(created.CaseID)
	require.NoError(t, err)

	closed := models.CaseStatusClosed
	updated, err := cases.UpdateCase(ctx, caseID, models.CaseUpdateRequest{Status: &closed})
	require.NoError(t, err)
	require.NotNil(t, updated.ClosedAt, "closing must stamp closed_at")

	reopened := models.CaseStatusOpen
	updated, err = cases.UpdateCase(ctx, caseID, models.CaseUpdateRequest{Status: &reopened})
	require.NoError(t, err)
	assert.Nil(t, updated.ClosedAt, "reopening must clear closed_at")
}

// TestIntegration_AgeBlockPersists covers the authoritative underage finding:
// it must survive the request that discovered it so capability gating can read
// it from the subject flags.
func TestIntegration_AgeBlockPersists(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	verifications := NewVerificationRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_age_block")
	require.NoError(t, err)

	require.NoError(t, verifications.SetAgeBlock(ctx, subjectID, "government_id_verification"))

	flags, err := verifications.GetSubjectVerificationFlags(ctx, subjectID)
	require.NoError(t, err)
	assert.True(t, flags.AgeBlocked, "a verified underage outcome must be persisted")
	require.NotNil(t, flags.AgeBlockedAt)

	// A full Plane B erase removes the restriction with the rest of the data.
	require.NoError(t, NewConsentRepository(db).DeleteSubjectPlaneBData(ctx, subjectID))
	flags, err = verifications.GetSubjectVerificationFlags(ctx, subjectID)
	require.NoError(t, err)
	assert.False(t, flags.AgeBlocked, "erasing Plane B data must clear the age restriction")
}

func TestIntegration_GovernmentIDAndLivenessRecords(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	verifications := NewVerificationRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_id_verify")
	require.NoError(t, err)

	dob := time.Date(1990, 5, 17, 0, 0, 0, 0, time.UTC)
	require.NoError(t, verifications.RecordGovernmentID(ctx, subjectID, &verification.IDVerificationResult{
		Status:          models.VerificationStatusVerified,
		VendorReference: "inq_123",
		VerifiedName:    "Jane Doe",
		DateOfBirth:     &dob,
		CountryCode:     "US",
		CostUSD:         1.25,
	}, "persona"))

	// A NULL date_of_birth must round-trip too: not every document has one.
	require.NoError(t, verifications.RecordGovernmentID(ctx, subjectID, &verification.IDVerificationResult{
		Status:          models.VerificationStatusFailed,
		VendorReference: "inq_124",
		FailureReason:   "unreadable",
	}, "persona"))

	matched := true
	require.NoError(t, verifications.RecordLiveness(ctx, subjectID, &verification.LivenessResult{
		Status:          models.VerificationStatusVerified,
		VendorReference: "live_1",
		LivenessScore:   0.97,
		MatchedIdentity: &matched,
		CostUSD:         0.40,
	}))

	var idRows, liveRows int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM government_id_verification WHERE subject_id = $1`, subjectID).Scan(&idRows))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM liveness_verification WHERE subject_id = $1`, subjectID).Scan(&liveRows))
	assert.Equal(t, 2, idRows)
	assert.Equal(t, 1, liveRows)

	var storedDOB *time.Time
	require.NoError(t, db.QueryRow(
		`SELECT date_of_birth FROM government_id_verification WHERE vendor_reference_id = 'inq_123'`).Scan(&storedDOB))
	require.NotNil(t, storedDOB)
	assert.Equal(t, "1990-05-17", storedDOB.Format("2006-01-02"))

	var storedScore float64
	require.NoError(t, db.QueryRow(
		`SELECT liveness_score FROM liveness_verification WHERE subject_id = $1`, subjectID).Scan(&storedScore))
	assert.InDelta(t, 0.97, storedScore, 0.001)
}

func TestIntegration_ImageVerificationRecord(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	verifications := NewVerificationRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_image")
	require.NoError(t, err)

	score := 0.91
	require.NoError(t, verifications.RecordImageVerification(ctx, &ImageVerificationRecord{
		SubjectID:         subjectID,
		ImageHash:         "abc123",
		ReverseProvider:   "tineye",
		ReverseMatchCount: 2,
		ReverseMatches:    []map[string]interface{}{{"url": "https://example.com/a.jpg", "similarity": 0.9}},
		SyntheticProvider: "hive",
		SyntheticScore:    &score,
		IsSynthetic:       true,
	}))

	var (
		hash       string
		isSynth    bool
		matches    []byte
		matchCnt   int
		synthScore float64
	)
	require.NoError(t, db.QueryRow(`
		SELECT image_hash, is_synthetic, matches, match_count, synthetic_score
		FROM image_verification WHERE subject_id = $1`, subjectID).
		Scan(&hash, &isSynth, &matches, &matchCnt, &synthScore))

	assert.Equal(t, "abc123", hash)
	assert.True(t, isSynth)
	assert.Equal(t, 2, matchCnt)
	assert.InDelta(t, 0.91, synthScore, 0.001)
	assert.Contains(t, string(matches), "example.com")
}

func TestIntegration_OAuthStateIsSingleUse(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	linkedin := NewLinkedInRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_oauth")
	require.NoError(t, err)

	require.NoError(t, linkedin.CreateOAuthState(ctx, "state-abc", subjectID, "linkedin", "https://app/cb"))

	gotSubject, _, err := linkedin.ConsumeOAuthState(ctx, "state-abc", "linkedin")
	require.NoError(t, err)
	assert.Equal(t, subjectID, gotSubject)

	// Replaying the same state must not resolve to a subject: that is the CSRF
	// protection the callback depends on.
	replaySubject, _, err := linkedin.ConsumeOAuthState(ctx, "state-abc", "linkedin")
	require.NoError(t, err)
	assert.Equal(t, uuid.Nil, replaySubject, "a consumed OAuth state must not be reusable")

	unknownSubject, _, err := linkedin.ConsumeOAuthState(ctx, "never-issued", "linkedin")
	require.NoError(t, err)
	assert.Equal(t, uuid.Nil, unknownSubject)
}

func TestIntegration_VerificationTokenLifecycle(t *testing.T) {
	db := testDB(t)
	subjects := NewSubjectRepository(db)
	verifications := NewVerificationRepository(db)
	ctx := context.Background()

	subjectID, err := subjects.FindOrCreateSubject(ctx, "cs_user_token")
	require.NoError(t, err)

	tokenID, err := verifications.CreateVerification(ctx, subjectID, nil,
		models.VerificationTypeGovernmentID, "persona", models.VerificationStatusProcessing)
	require.NoError(t, err)

	require.NoError(t, verifications.CompleteVerification(ctx, tokenID,
		models.VerificationStatusVerified, "", 1.25, map[string]interface{}{"vendorReference": "inq_9"}))

	records, err := verifications.ListBySubject(ctx, subjectID)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, models.VerificationStatusVerified, records[0].Status)
	assert.InDelta(t, 1.25, records[0].CostUSD, 0.001)
}
