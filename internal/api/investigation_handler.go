package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/audit"
	"github.com/nightkiller1977-del/trustgraph/internal/config"
	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/osint"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// InvestigationHandler serves Plane C: cases, notes, OSINT tools, and
// break-glass access. All routes sit behind investigatorAuth; individual
// operations additionally enforce a minimum role.
type InvestigationHandler struct {
	db       *store.PostgresDB
	logger   *zap.Logger
	cfg      *config.Config
	cases    *store.InvestigationRepository
	subjects *store.SubjectRepository
	tools    *osint.Registry
	auditor  *audit.AuditLogger
}

func NewInvestigationHandler(db *store.PostgresDB, logger *zap.Logger, cfg *config.Config) *InvestigationHandler {
	return &InvestigationHandler{
		db:       db,
		logger:   logger,
		cfg:      cfg,
		cases:    store.NewInvestigationRepository(db),
		subjects: store.NewSubjectRepository(db),
		tools: osint.NewRegistry(
			osint.NewInternetArchiveTool(""),
			osint.NewDomainIntelTool("", ""),
			osint.NewUsernameSearchTool(),
			osint.NewHarvesterTool(cfg.HarvesterBinary),
			osint.NewSpiderFootTool(cfg.SpiderFootBaseURL, cfg.SpiderFootAPIKey, cfg.SpiderFootBinary),
		),
		auditor: audit.NewAuditLogger(db, logger),
	}
}

// ListTools advertises the available OSINT tools and whether each is runnable.
func (h *InvestigationHandler) ListTools(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"tools": h.tools.List()})
}

// CreateCase opens an investigation case.
func (h *InvestigationHandler) CreateCase(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if !requireRole(ctx, w, RoleInvestigatorInvestigator) {
		return
	}

	var req models.CaseCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}
	if req.Title == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "title is required")
		return
	}
	if !isValidCasePriority(req.Priority) {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "priority must be one of: low, normal, high, urgent")
		return
	}

	actor := investigatorActor(ctx)
	created, err := h.cases.CreateCase(ctx, req, actor)
	if err != nil {
		h.logger.Error("create case failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to create case")
		return
	}

	subjectID := parseOptionalUUID(created.SubjectID)
	h.auditor.Log(ctx, audit.AuditEvent{
		Plane: audit.PlaneC, Action: audit.ActionInvestigationCaseCreated,
		Actor: actor, ActorType: audit.ActorTypeInvestigator,
		ResourceType: "investigation_case", SubjectID: subjectID,
		Details: map[string]interface{}{"caseId": created.CaseID, "caseNumber": created.CaseNumber, "priority": created.Priority},
		Result:  "ok", RequestID: r.Header.Get("X-Request-ID"),
	})
	h.recordAccess(ctx, created.CaseID, subjectID, actor, investigatorRole(ctx), "case.create", true, "", nil)

	writeJSON(w, http.StatusCreated, created)
}

// ListCases returns cases, optionally filtered by status.
func (h *InvestigationHandler) ListCases(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	status := r.URL.Query().Get("status")
	if status != "" && !isValidCaseStatus(status) {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Unknown status filter")
		return
	}

	cases, err := h.cases.ListCases(ctx, status, 0)
	if err != nil {
		h.logger.Error("list cases failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to list cases")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"cases": cases})
}

// GetCase returns a case and its notes, and logs the read.
func (h *InvestigationHandler) GetCase(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	caseID, ok := parseCaseID(w, r)
	if !ok {
		return
	}

	investigation, err := h.cases.GetCase(ctx, caseID)
	if err != nil {
		h.logger.Error("get case failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to load case")
		return
	}
	if investigation == nil {
		writeJSONError(w, http.StatusNotFound, "not_found", "Case not found")
		return
	}

	actor := investigatorActor(ctx)
	subjectID := parseOptionalUUID(investigation.SubjectID)
	h.auditor.Log(ctx, audit.AuditEvent{
		Plane: audit.PlaneC, Action: audit.ActionInvestigationCaseViewed,
		Actor: actor, ActorType: audit.ActorTypeInvestigator,
		ResourceType: "investigation_case", SubjectID: subjectID,
		Details: map[string]interface{}{"caseId": caseID.String()},
		Result:  "ok", RequestID: r.Header.Get("X-Request-ID"),
	})
	h.recordAccess(ctx, caseID.String(), subjectID, actor, investigatorRole(ctx), "case.view", true, "", nil)

	writeJSON(w, http.StatusOK, investigation)
}

// UpdateCase applies a partial update.
func (h *InvestigationHandler) UpdateCase(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	caseID, ok := parseCaseID(w, r)
	if !ok {
		return
	}

	var req models.CaseUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}
	if req.Status != nil && !isValidCaseStatus(*req.Status) {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Unknown status")
		return
	}
	if req.Priority != nil && !isValidCasePriority(*req.Priority) {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Unknown priority")
		return
	}

	// Closing or resolving a case is a supervisor action: it ends the
	// investigation and is the point where a false accusation would be sealed
	// into the record. The same protection extends to any later edit of a case
	// that is already terminal — without it an investigator could rewrite
	// findings, resolution, assignment, or priority by simply omitting `status`.
	existing, err := h.cases.GetCase(ctx, caseID)
	if err != nil {
		h.logger.Error("load case for authorization failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to load case")
		return
	}
	if existing == nil {
		writeJSONError(w, http.StatusNotFound, "not_found", "Case not found")
		return
	}
	terminal := existing.Status == models.CaseStatusClosed || existing.Status == models.CaseStatusResolved

	closing := req.Status != nil && (*req.Status == models.CaseStatusClosed || *req.Status == models.CaseStatusResolved)
	required := RoleInvestigatorInvestigator
	if closing || terminal {
		required = RoleInvestigatorSupervisor
	}

	// A break-glass grant may be scoped to a subject or case, so an elevated
	// request must be authorised against this case's identifiers; a grant issued
	// for another case must not cover this one.
	var grant *models.BreakGlassGrant
	if roleRank(investigatorRole(ctx)) < roleRank(required) {
		caseSubjectID := parseOptionalUUID(existing.SubjectID)
		grant = requireRoleOrBreakGlass(ctx, w, required, caseSubjectID, &caseID, h.cases, h.logger)
		if grant == nil {
			return
		}
	}

	updated, err := h.cases.UpdateCase(ctx, caseID, req)
	if err != nil {
		h.logger.Error("update case failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to update case")
		return
	}
	if updated == nil {
		writeJSONError(w, http.StatusNotFound, "not_found", "Case not found")
		return
	}

	actor := investigatorActor(ctx)
	subjectID := parseOptionalUUID(updated.SubjectID)
	action := audit.ActionInvestigationCaseUpdated
	if closing {
		action = audit.ActionInvestigationCaseClosed
	}
	h.auditor.Log(ctx, audit.AuditEvent{
		Plane: audit.PlaneC, Action: action,
		Actor: actor, ActorType: audit.ActorTypeInvestigator,
		ResourceType: "investigation_case", SubjectID: subjectID,
		Details: map[string]interface{}{"caseId": caseID.String(), "status": updated.Status},
		Result:  "ok", RequestID: r.Header.Get("X-Request-ID"),
	})
	h.recordAccess(ctx, caseID.String(), subjectID, actor, investigatorRole(ctx), "case.update", true, "", grant)

	writeJSON(w, http.StatusOK, updated)
}

// AddNote appends a note to a case.
func (h *InvestigationHandler) AddNote(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if !requireRole(ctx, w, RoleInvestigatorInvestigator) {
		return
	}

	caseID, ok := parseCaseID(w, r)
	if !ok {
		return
	}

	var req struct {
		Content  string `json:"content"`
		NoteType string `json:"noteType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}
	if req.Content == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "content is required")
		return
	}

	actor := investigatorActor(ctx)
	note, err := h.cases.AddNote(ctx, caseID, actor, investigatorRole(ctx), req.NoteType, req.Content)
	if err != nil {
		h.logger.Error("add note failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to add note")
		return
	}

	h.auditor.Log(ctx, audit.AuditEvent{
		Plane: audit.PlaneC, Action: audit.ActionInvestigationCaseUpdated,
		Actor: actor, ActorType: audit.ActorTypeInvestigator,
		ResourceType: "investigation_note",
		Details:      map[string]interface{}{"caseId": caseID.String(), "noteId": note.NoteID},
		Result:       "ok", RequestID: r.Header.Get("X-Request-ID"),
	})

	writeJSON(w, http.StatusCreated, note)
}

// RunTool executes an OSINT tool and records the invocation.
func (h *InvestigationHandler) RunTool(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	if !requireRole(ctx, w, RoleInvestigatorInvestigator) {
		return
	}

	toolName := mux.Vars(r)["tool"]
	tool, ok := h.tools.Get(toolName)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "not_found", "Unknown tool")
		return
	}
	if !tool.Configured() {
		writeJSONError(w, http.StatusServiceUnavailable, "not_configured", "That tool is not available on this deployment")
		return
	}

	var req struct {
		Query   map[string]interface{} `json:"query"`
		CaseID  string                 `json:"caseId"`
		Subject string                 `json:"subjectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	actor := investigatorActor(ctx)
	start := time.Now()

	result, err := tool.Run(ctx, req.Query)
	duration := time.Since(start)

	record := &models.InvestigationToolQuery{
		ToolName:    toolName,
		QueryParams: req.Query,
		DurationMS:  int(duration.Milliseconds()),
		Actor:       actor,
		ActorRole:   investigatorRole(ctx),
		Status:      "success",
	}
	if req.CaseID != "" {
		if id, parseErr := uuid.Parse(req.CaseID); parseErr == nil {
			s := id.String()
			record.CaseID = &s
		}
	}
	if req.Subject != "" {
		if id, parseErr := uuid.Parse(req.Subject); parseErr == nil {
			s := id.String()
			record.SubjectID = &s
		}
	}

	if err != nil {
		record.Status = "error"
		record.ErrorMessage = err.Error()
		// The tool deadline may have just expired, which is exactly the case
		// worth auditing. Use a fresh bounded context so the query and audit
		// rows are still written when ctx is already canceled.
		bookkeepingCtx, bookkeepingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer bookkeepingCancel()
		if logErr := h.cases.RecordToolQuery(bookkeepingCtx, record); logErr != nil {
			h.logger.Error("record failed tool query", zap.Error(logErr))
		}
		h.auditor.Log(bookkeepingCtx, audit.AuditEvent{
			Plane: audit.PlaneC, Action: audit.ActionInvestigationToolQueried,
			Actor: actor, ActorType: audit.ActorTypeInvestigator,
			ResourceType: "osint_tool",
			Details:      map[string]interface{}{"tool": toolName, "error": err.Error()},
			Result:       "failed", RequestID: r.Header.Get("X-Request-ID"),
		})
		status := http.StatusBadGateway
		if errors.Is(err, osint.ErrToolNotConfigured) {
			status = http.StatusServiceUnavailable
		}
		writeJSONError(w, status, "tool_failed", "The tool could not complete the query")
		return
	}

	record.ResultCount = result.Count
	record.CostUSD = result.CostUSD
	record.ResultSummary = map[string]interface{}{
		"count":   result.Count,
		"summary": result.Summary,
	}
	if err := h.cases.RecordToolQuery(ctx, record); err != nil {
		h.logger.Error("record tool query failed", zap.Error(err))
	}

	h.auditor.Log(ctx, audit.AuditEvent{
		Plane: audit.PlaneC, Action: audit.ActionInvestigationToolQueried,
		Actor: actor, ActorType: audit.ActorTypeInvestigator,
		ResourceType: "osint_tool",
		Details:      map[string]interface{}{"tool": toolName, "count": result.Count, "queryId": record.QueryID},
		Result:       "ok", RequestID: r.Header.Get("X-Request-ID"),
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"queryId": record.QueryID,
		"tool":    toolName,
		"result":  result,
	})
}

// RequestBreakGlass grants time-boxed elevated access. The grant is always
// recorded and flagged for review; the justification is mandatory because the
// grant is the audit trail's explanation for whatever follows.
func (h *InvestigationHandler) RequestBreakGlass(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var req struct {
		Justification string `json:"justification"`
		SubjectID     string `json:"subjectId"`
		CaseID        string `json:"caseId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}
	if len(req.Justification) < 20 {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "A justification of at least 20 characters is required")
		return
	}

	duration := time.Duration(h.cfg.BreakGlassDurationMinutes) * time.Minute
	if duration <= 0 || duration > 4*time.Hour {
		duration = time.Hour
	}

	var subjectID, caseID *uuid.UUID
	// A malformed scope identifier must be rejected, not ignored: silently dropping
	// it would create an unscoped grant, widening an intended subject- or
	// case-limited emergency request into system-wide supervisor access.
	if req.SubjectID != "" {
		id, err := uuid.Parse(req.SubjectID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "bad_request", "subjectId must be a UUID")
			return
		}
		subjectID = &id
	}
	if req.CaseID != "" {
		id, err := uuid.Parse(req.CaseID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "bad_request", "caseId must be a UUID")
			return
		}
		caseID = &id
	}

	actor := investigatorActor(ctx)
	grant, err := h.cases.CreateBreakGlassGrant(ctx, actor, investigatorRole(ctx), req.Justification, subjectID, caseID, duration)
	if err != nil {
		h.logger.Error("create break glass failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to create break-glass grant")
		return
	}

	h.auditor.Log(ctx, audit.AuditEvent{
		Plane: audit.PlaneC, Action: audit.ActionInvestigationActionTaken,
		Actor: actor, ActorType: audit.ActorTypeInvestigator,
		ResourceType: "break_glass", SubjectID: subjectID,
		Details: map[string]interface{}{
			"grantId":       grant.GrantID,
			"justification": req.Justification,
			"expiresAt":     grant.ExpiresAt.Format(time.RFC3339),
		},
		Result: "ok", RequestID: r.Header.Get("X-Request-ID"),
	})
	h.recordAccess(ctx, req.CaseID, subjectID, actor, investigatorRole(ctx), "break_glass.request", true, req.Justification, nil)
	if caseID != nil {
		h.recordBreakGlassAccess(ctx, caseID.String(), subjectID, actor, investigatorRole(ctx), req.Justification)
	}

	// The response repeats the justification so the console can show the
	// operator exactly what was recorded on their behalf.
	writeJSON(w, http.StatusCreated, grant)
}

// compile-time assertion that audit.PlaneC exists and is used.
var _ = audit.PlaneC

// GetAccessLog returns the Plane C access trail for a case. It is a
// supervisor-only read: the log names investigators and their justifications.
func (h *InvestigationHandler) GetAccessLog(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	caseID, ok := parseCaseID(w, r)
	if !ok {
		return
	}

	// A grant scoped to a subject or case only authorises reading that target's
	// access log, so an elevated request is checked against the case's
	// identifiers. The common path (supervisor) skips the extra lookup.
	var grant *models.BreakGlassGrant
	if roleRank(investigatorRole(ctx)) < roleRank(RoleInvestigatorSupervisor) {
		existing, err := h.cases.GetCase(ctx, caseID)
		if err != nil {
			h.logger.Error("load case for authorization failed", zap.Error(err))
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to load case")
			return
		}
		if existing == nil {
			writeJSONError(w, http.StatusNotFound, "not_found", "Case not found")
			return
		}
		caseSubjectID := parseOptionalUUID(existing.SubjectID)
		grant = requireRoleOrBreakGlass(ctx, w, RoleInvestigatorSupervisor, caseSubjectID, &caseID, h.cases, h.logger)
		if grant == nil {
			return
		}
	}

	entries, err := h.cases.ListAccessLog(ctx, caseID, 0)
	if err != nil {
		h.logger.Error("list access log failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to load access log")
		return
	}

	// A break-glass read of the access log is itself a sensitive, privileged
	// access: record it so the elevation and the grant that authorised it are
	// attributable in the trail, not merely the nil check above.
	if grant != nil {
		h.recordAccess(ctx, caseID.String(), nil, investigatorActor(ctx), investigatorRole(ctx), "access_log.view", true, "", grant)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"caseId": caseID.String(), "entries": entries})
}

func (h *InvestigationHandler) recordAccess(ctx context.Context, caseID string, subjectID *uuid.UUID, actor, role, action string, granted bool, reason string, grant *models.BreakGlassGrant) {
	var caseUUID *uuid.UUID
	if caseID != "" {
		if id, err := uuid.Parse(caseID); err == nil {
			caseUUID = &id
		}
	}
	entry := &store.AccessLogEntry{
		CaseID: caseUUID, SubjectID: subjectID, Actor: actor, ActorRole: role,
		Action: action, Granted: granted, Reason: reason,
	}
	// Mark actions that were authorised by break-glass rather than by standing
	// role, and record why, so the durable trail does not let an elevated action
	// read as an ordinary one.
	if grant != nil {
		entry.BreakGlass = true
		entry.Reason = strings.TrimSpace(reason + " break_glass grant_id=" + grant.GrantID + " justification=" + grant.Justification)
	}
	if err := h.cases.RecordAccess(ctx, entry); err != nil {
		h.logger.Error("record investigation access failed", zap.Error(err))
	}
}

func (h *InvestigationHandler) recordBreakGlassAccess(ctx context.Context, caseID string, subjectID *uuid.UUID, actor, role, reason string) {
	var caseUUID *uuid.UUID
	if id, err := uuid.Parse(caseID); err == nil {
		caseUUID = &id
	}
	entry := &store.AccessLogEntry{
		CaseID: caseUUID, SubjectID: subjectID, Actor: actor, ActorRole: role,
		Action: "break_glass.granted", Granted: true, Reason: reason, BreakGlass: true,
	}
	if err := h.cases.RecordAccess(ctx, entry); err != nil {
		h.logger.Error("record break-glass access failed", zap.Error(err))
	}
}

func parseCaseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(mux.Vars(r)["caseId"])
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid case ID")
		return uuid.Nil, false
	}
	return id, true
}

func parseOptionalUUID(s *string) *uuid.UUID {
	if s == nil {
		return nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil
	}
	return &id
}

func isValidCaseStatus(status string) bool {
	switch status {
	case models.CaseStatusOpen, models.CaseStatusInvestigating, models.CaseStatusEscalated,
		models.CaseStatusResolved, models.CaseStatusClosed:
		return true
	default:
		return false
	}
}

func isValidCasePriority(priority string) bool {
	switch priority {
	case "", models.CasePriorityLow, models.CasePriorityNormal, models.CasePriorityHigh, models.CasePriorityUrgent:
		return true
	default:
		return false
	}
}
