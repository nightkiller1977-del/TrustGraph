package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
)

// InvestigationRepository stores Plane C cases, notes, tool queries,
// break-glass grants, and access logs.
type InvestigationRepository struct {
	db *PostgresDB
}

func NewInvestigationRepository(db *PostgresDB) *InvestigationRepository {
	return &InvestigationRepository{db: db}
}

// CreateCase opens a case and assigns it the next human-readable case number.
// The number is derived inside the transaction so a concurrent create cannot
// hand out the same number.
func (r *InvestigationRepository) CreateCase(ctx context.Context, req models.CaseCreateRequest, createdBy string) (*models.InvestigationCase, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin case tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Serialise case-number allocation for this transaction.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('investigation_case_number'))`); err != nil {
		return nil, fmt.Errorf("lock case number: %w", err)
	}

	var nextNumber int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(NULLIF(regexp_replace(case_number, '\D', '', 'g'), '')::int), 1000) + 1 FROM investigation_case`,
	).Scan(&nextNumber); err != nil {
		return nil, fmt.Errorf("allocate case number: %w", err)
	}
	caseNumber := fmt.Sprintf("INV-%d", nextNumber)

	var subjectID *uuid.UUID
	if req.SubjectID != "" {
		parsed, err := uuid.Parse(req.SubjectID)
		if err != nil {
			return nil, fmt.Errorf("invalid subject id: %w", err)
		}
		subjectID = &parsed
	}

	priority := req.Priority
	if priority == "" {
		priority = models.CasePriorityNormal
	}

	const insert = `
		INSERT INTO investigation_case (
			case_number, subject_id, title, description, status, priority,
			trigger_type, trigger_ref, assigned_to, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING case_id, opened_at, created_at, updated_at
	`

	var created models.InvestigationCase
	err = tx.QueryRowContext(ctx, insert,
		caseNumber, subjectID, req.Title, req.Description, models.CaseStatusOpen, priority,
		req.TriggerType, req.TriggerRef, req.AssignedTo, createdBy,
	).Scan(&created.CaseID, &created.OpenedAt, &created.CreatedAt, &created.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create case: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit case: %w", err)
	}

	created.CaseNumber = caseNumber
	created.Title = req.Title
	created.Description = req.Description
	created.Status = models.CaseStatusOpen
	created.Priority = priority
	created.TriggerType = req.TriggerType
	created.TriggerRef = req.TriggerRef
	created.AssignedTo = req.AssignedTo
	created.CreatedBy = createdBy
	if subjectID != nil {
		s := subjectID.String()
		created.SubjectID = &s
	}

	return &created, nil
}

// GetCase loads a case with its notes.
func (r *InvestigationRepository) GetCase(ctx context.Context, caseID uuid.UUID) (*models.InvestigationCase, error) {
	query := `
		SELECT case_id, case_number, subject_id, title, COALESCE(description, ''), status,
		       priority, COALESCE(trigger_type, ''), COALESCE(trigger_ref, ''),
		       COALESCE(assigned_to, ''), created_by, COALESCE(findings, ''),
		       COALESCE(resolution, ''), opened_at, closed_at, created_at, updated_at
		FROM investigation_case
		WHERE case_id = $1
	`

	var c models.InvestigationCase
	var subjectID sql.NullString
	var closedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, caseID).Scan(
		&c.CaseID, &c.CaseNumber, &subjectID, &c.Title, &c.Description, &c.Status,
		&c.Priority, &c.TriggerType, &c.TriggerRef, &c.AssignedTo, &c.CreatedBy,
		&c.Findings, &c.Resolution, &c.OpenedAt, &closedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get case: %w", err)
	}
	if subjectID.Valid {
		c.SubjectID = &subjectID.String
	}
	if closedAt.Valid {
		c.ClosedAt = &closedAt.Time
	}

	notes, err := r.listNotes(ctx, caseID)
	if err != nil {
		return nil, err
	}
	c.Notes = notes

	return &c, nil
}

// ListCases returns cases, optionally filtered by status, newest first.
func (r *InvestigationRepository) ListCases(ctx context.Context, status string, limit int) ([]models.InvestigationCase, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := `
		SELECT case_id, case_number, subject_id, title, status, priority,
		       COALESCE(assigned_to, ''), created_by, opened_at, closed_at, created_at
		FROM investigation_case
		WHERE ($1 = '' OR status = $1)
		ORDER BY
		  CASE priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END,
		  opened_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list cases: %w", err)
	}
	defer rows.Close()

	cases := make([]models.InvestigationCase, 0)
	for rows.Next() {
		var c models.InvestigationCase
		var subjectID sql.NullString
		var closedAt sql.NullTime
		if err := rows.Scan(&c.CaseID, &c.CaseNumber, &subjectID, &c.Title, &c.Status,
			&c.Priority, &c.AssignedTo, &c.CreatedBy, &c.OpenedAt, &closedAt, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan case: %w", err)
		}
		if subjectID.Valid {
			c.SubjectID = &subjectID.String
		}
		if closedAt.Valid {
			c.ClosedAt = &closedAt.Time
		}
		cases = append(cases, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cases: %w", err)
	}
	return cases, nil
}

// UpdateCase applies a partial update and stamps updated_at.
func (r *InvestigationRepository) UpdateCase(ctx context.Context, caseID uuid.UUID, req models.CaseUpdateRequest) (*models.InvestigationCase, error) {
	query := `
		UPDATE investigation_case SET
			status = COALESCE($2, status),
			priority = COALESCE($3, priority),
			assigned_to = COALESCE($4, assigned_to),
			findings = COALESCE($5, findings),
			resolution = COALESCE($6, resolution),
			closed_at = CASE
				WHEN $2 IN ('closed', 'resolved') AND closed_at IS NULL THEN now()
				WHEN $2 IN ('open', 'investigating', 'escalated') THEN NULL
				ELSE closed_at
			END,
			updated_at = now()
		WHERE case_id = $1
		RETURNING updated_at
	`

	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, query, caseID,
		nullableString(req.Status), nullableString(req.Priority),
		nullableString(req.AssignedTo), nullableString(req.Findings),
		nullableString(req.Resolution),
	).Scan(&updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("update case: %w", err)
	}

	return r.GetCase(ctx, caseID)
}

// AddNote appends a note to a case.
func (r *InvestigationRepository) AddNote(ctx context.Context, caseID uuid.UUID, author, authorRole, noteType, content string) (*models.InvestigationNote, error) {
	if noteType == "" {
		noteType = "note"
	}
	query := `
		INSERT INTO investigation_note (case_id, author, author_role, note_type, content)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING note_id, created_at
	`

	var note models.InvestigationNote
	if err := r.db.QueryRowContext(ctx, query, caseID, author, authorRole, noteType, content).
		Scan(&note.NoteID, &note.CreatedAt); err != nil {
		return nil, fmt.Errorf("add note: %w", err)
	}

	note.CaseID = caseID.String()
	note.Author = author
	note.AuthorRole = authorRole
	note.NoteType = noteType
	note.Content = content
	return &note, nil
}

func (r *InvestigationRepository) listNotes(ctx context.Context, caseID uuid.UUID) ([]models.InvestigationNote, error) {
	query := `
		SELECT note_id, author, COALESCE(author_role, ''), COALESCE(note_type, 'note'), content, created_at
		FROM investigation_note
		WHERE case_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, caseID)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	notes := make([]models.InvestigationNote, 0)
	for rows.Next() {
		var n models.InvestigationNote
		if err := rows.Scan(&n.NoteID, &n.Author, &n.AuthorRole, &n.NoteType, &n.Content, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		n.CaseID = caseID.String()
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notes: %w", err)
	}
	return notes, nil
}

// RecordToolQuery persists one OSINT invocation.
func (r *InvestigationRepository) RecordToolQuery(ctx context.Context, q *models.InvestigationToolQuery) error {
	paramsJSON, err := json.Marshal(q.QueryParams)
	if err != nil {
		return fmt.Errorf("marshal tool params: %w", err)
	}
	summaryJSON, err := json.Marshal(q.ResultSummary)
	if err != nil {
		return fmt.Errorf("marshal tool summary: %w", err)
	}

	query := `
		INSERT INTO investigation_tool_query (
			case_id, subject_id, tool_name, query_params, result_summary, result_count,
			status, error_message, cost_usd, duration_ms, actor, actor_role
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, $10, $11, $12)
		RETURNING query_id, created_at
	`

	if err := r.db.QueryRowContext(ctx, query,
		q.CaseID, q.SubjectID, q.ToolName, paramsJSON, summaryJSON, q.ResultCount,
		q.Status, q.ErrorMessage, q.CostUSD, q.DurationMS, q.Actor, q.ActorRole,
	).Scan(&q.QueryID, &q.CreatedAt); err != nil {
		return fmt.Errorf("record tool query: %w", err)
	}
	return nil
}

// CreateBreakGlassGrant records a time-boxed emergency elevation.
func (r *InvestigationRepository) CreateBreakGlassGrant(ctx context.Context, actor, actorRole, justification string, subjectID, caseID *uuid.UUID, duration time.Duration) (*models.BreakGlassGrant, error) {
	query := `
		INSERT INTO investigation_break_glass (
			actor, actor_role, subject_id, case_id, justification, expires_at
		) VALUES ($1, $2, $3, $4, $5, now() + $6::interval)
		RETURNING grant_id, granted_at, expires_at
	`

	var grant models.BreakGlassGrant
	if err := r.db.QueryRowContext(ctx, query, actor, actorRole, subjectID, caseID,
		justification, fmt.Sprintf("%d seconds", int(duration.Seconds())),
	).Scan(&grant.GrantID, &grant.GrantedAt, &grant.ExpiresAt); err != nil {
		return nil, fmt.Errorf("create break glass: %w", err)
	}

	grant.Actor = actor
	grant.ActorRole = actorRole
	grant.Justification = justification
	if subjectID != nil {
		s := subjectID.String()
		grant.SubjectID = &s
	}
	if caseID != nil {
		s := caseID.String()
		grant.CaseID = &s
	}
	return &grant, nil
}

// ActiveBreakGlassGrant returns an unexpired, unrevoked grant for an actor that
// covers the requested target. A grant is scoped when it records a subject_id
// or case_id; it only authorises actions on those targets. A grant with neither
// is global for its window. Passing a nil target permits only a global grant, so
// an unknown target cannot borrow a subject- or case-scoped elevation.
func (r *InvestigationRepository) ActiveBreakGlassGrant(ctx context.Context, actor string, subjectID, caseID *uuid.UUID) (*models.BreakGlassGrant, error) {
	query := `
		SELECT grant_id, actor_role, justification, subject_id, case_id, granted_at, expires_at
		FROM investigation_break_glass
		WHERE actor = $1
		  AND revoked_at IS NULL
		  AND expires_at > now()
		  AND (subject_id IS NULL OR ($2::uuid IS NOT NULL AND subject_id = $2::uuid))
		  AND (case_id IS NULL OR ($3::uuid IS NOT NULL AND case_id = $3::uuid))
		ORDER BY granted_at DESC
		LIMIT 1
	`

	var subjectArg, caseArg interface{}
	if subjectID != nil {
		subjectArg = *subjectID
	}
	if caseID != nil {
		caseArg = *caseID
	}

	var grant models.BreakGlassGrant
	var subjectOut, caseOut *uuid.UUID
	err := r.db.QueryRowContext(ctx, query, actor, subjectArg, caseArg).Scan(
		&grant.GrantID, &grant.ActorRole, &grant.Justification, &subjectOut, &caseOut, &grant.GrantedAt, &grant.ExpiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get break glass: %w", err)
	}
	grant.Actor = actor
	if subjectOut != nil {
		s := subjectOut.String()
		grant.SubjectID = &s
	}
	if caseOut != nil {
		s := caseOut.String()
		grant.CaseID = &s
	}
	return &grant, nil
}

// AccessLogEntry is one Plane C access record.
type AccessLogEntry struct {
	CaseID      *uuid.UUID
	SubjectID   *uuid.UUID
	Actor       string
	ActorRole   string
	Action      string
	Resource    string
	ResourceRef string
	Granted     bool
	Reason      string
	BreakGlass  bool
}

// RecordAccess persists one access-log row.
func (r *InvestigationRepository) RecordAccess(ctx context.Context, entry *AccessLogEntry) error {
	query := `
		INSERT INTO investigation_access_log (
			case_id, subject_id, actor, actor_role, action, resource, resource_ref,
			granted, reason, break_glass
		) VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), $8, NULLIF($9, ''), $10)
	`
	if _, err := r.db.ExecContext(ctx, query,
		entry.CaseID, entry.SubjectID, entry.Actor, entry.ActorRole, entry.Action,
		entry.Resource, entry.ResourceRef, entry.Granted, entry.Reason, entry.BreakGlass,
	); err != nil {
		return fmt.Errorf("record access: %w", err)
	}
	return nil
}

// ListAccessLog returns access-log rows for a case, newest first.
func (r *InvestigationRepository) ListAccessLog(ctx context.Context, caseID uuid.UUID, limit int) ([]AccessLogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := `
		SELECT case_id, subject_id, actor, COALESCE(actor_role, ''), action,
		       COALESCE(resource, ''), COALESCE(resource_ref, ''), granted,
		       COALESCE(reason, ''), break_glass
		FROM investigation_access_log
		WHERE case_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, caseID, limit)
	if err != nil {
		return nil, fmt.Errorf("list access log: %w", err)
	}
	defer rows.Close()

	entries := make([]AccessLogEntry, 0)
	for rows.Next() {
		var e AccessLogEntry
		var caseIDVal, subjectIDVal sql.NullString
		if err := rows.Scan(&caseIDVal, &subjectIDVal, &e.Actor, &e.ActorRole, &e.Action,
			&e.Resource, &e.ResourceRef, &e.Granted, &e.Reason, &e.BreakGlass); err != nil {
			return nil, fmt.Errorf("scan access log: %w", err)
		}
		if caseIDVal.Valid {
			if id, err := uuid.Parse(caseIDVal.String); err == nil {
				e.CaseID = &id
			}
		}
		if subjectIDVal.Valid {
			if id, err := uuid.Parse(subjectIDVal.String); err == nil {
				e.SubjectID = &id
			}
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate access log: %w", err)
	}
	return entries, nil
}

func nullableString(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}
