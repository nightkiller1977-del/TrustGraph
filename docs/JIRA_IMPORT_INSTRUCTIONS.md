# JIRA Import Instructions: Sprint 1 & Sprint 2

## Overview

This guide explains how to import Sprint 1 and Sprint 2 tasks into JIRA for the TrustGraph Plane B implementation.

**Files to Import:**
- `JIRA_SPRINT1_SPRINT2_IMPORT.csv` — CSV format (for JIRA cloud bulk import)
- `JIRA_SPRINT1_SPRINT2_IMPORT.json` — JSON format (for JIRA API or CLI)

**Tasks Included:**
- 1 Epic (TG-PLANE-B: Plane B - Consented Verification)
- 2 Features (Sprint 1, Sprint 2)
- 9 Stories (6 for Sprint 1, 3 for Sprint 2)

---

## Import Method 1: JIRA Cloud Bulk Import (Recommended)

### Step 1: Access JIRA Bulk Import Tool

1. Go to **Jira Settings** → **Tools** → **Bulk Import**
2. Or navigate directly: `https://YOUR-DOMAIN.atlassian.net/secure/BulkImportStepOne.jspa`

### Step 2: Download & Prepare CSV

1. Copy the CSV file:
   ```bash
   cp docs/JIRA_SPRINT1_SPRINT2_IMPORT.csv ~/Desktop/
   ```

2. Open in Excel or Google Sheets and review:
   - Verify all fields are correct
   - Replace `[Engineer A]` and `[Engineer B]` with actual usernames
   - Verify sprint names match your JIRA sprint configuration

3. Save as `.csv` (UTF-8 encoding)

### Step 3: Upload CSV

1. In JIRA Bulk Import:
   - **Select CSV File** → Choose `JIRA_SPRINT1_SPRINT2_IMPORT.csv`
   - **Project** → Select `TrustGraph` (or `TG`)
   - Click **Next**

2. **Map Fields** (if needed):
   - Project Key → Project Key
   - Issue Type → Issue Type
   - Summary → Summary
   - Description → Description
   - Story Points → Custom Field (Story Points)
   - etc.

3. Click **Next** to review

### Step 4: Review & Confirm

1. Review the preview:
   - 1 Epic should be listed
   - 2 Features should be listed
   - 9 Stories should be listed
   - Total: 12 issues

2. Verify:
   - All issue types correct
   - All summaries readable
   - All story points assigned
   - All labels correct

3. Click **Import**

### Step 5: Verify Import

```bash
# Wait 2-3 minutes for import to complete

# Verify in JIRA:
# 1. Search: "TG-LI-001" (should find LinkedIn OAuth story)
# 2. Check Epic: Open TG-PLANE-B (should show 2 features + 9 stories)
# 3. Check Sprint: Go to Sprint 1 (should show 6 stories)
# 4. Check Labels: Filter by "ai-commander-ready" (should show 12 issues)
```

---

## Import Method 2: JIRA API (CLI)

### Prerequisites

```bash
# Install JIRA CLI
brew install jira-cli  # macOS
# or
apt install jira-cli   # Linux
# or
# Download from https://github.com/go-jira/jira

# Configure JIRA CLI
jira init
# Enter: Site URL, Username, API Token (NOT password)
```

### Import JSON

```bash
# Option 1: Using jira CLI (if supported)
jira bulk --file docs/JIRA_SPRINT1_SPRINT2_IMPORT.json

# Option 2: Using curl (direct API)
curl -X POST \
  -H "Authorization: Bearer YOUR_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d @docs/JIRA_SPRINT1_SPRINT2_IMPORT.json \
  https://YOUR-DOMAIN.atlassian.net/rest/api/2/issues/bulk

# Option 3: Using gh CLI (if JIRA integrated with GitHub)
gh api repos/nightkiller1977-del/TrustGraph/issues create \
  --input docs/JIRA_SPRINT1_SPRINT2_IMPORT.json
```

---

## Import Method 3: Manual Creation (If Bulk Import Fails)

If the bulk import fails, create tasks manually:

### 1. Create Epic

```
Type: Epic
Key: TG-PLANE-B
Title: Plane B - Consented Verification (Phase 1)
Description: [Copy from JIRA_SPRINT1_SPRINT2_IMPORT.csv]
Labels: ai-commander-ready, plane-b, phase-1
Priority: High
Due Date: 2025-04-25
```

### 2. Create Features

```
Type: Feature
Parent: TG-PLANE-B
Title: Sprint 1: LinkedIn OAuth Integration
Labels: ai-commander-ready, sprint-1, linkedin, plane-b
Priority: High
Due Date: 2025-03-16
```

```
Type: Feature
Parent: TG-PLANE-B
Title: Sprint 2: Government ID Verification + Age Gate
Labels: ai-commander-ready, sprint-2, government-id, plane-b, critical
Priority: Critical
Due Date: 2025-03-30
```

### 3. Create Stories (6 for Sprint 1)

Create each story with:
- Parent: TG-PLANE-B (Epic)
- Sprint: Sprint 1 or Sprint 2 (as appropriate)
- Story Points: As listed in CSV
- Labels: `ai-commander-ready` + functional labels
- Assignee: [Engineer A] or [Engineer B]

Stories to create:
- TG-LI-001: LinkedIn OAuth Flow Setup (8 pts)
- TG-LI-002: LinkedIn Data Extraction & Storage (5 pts)
- TG-LI-003: Employment Validator (Free) (5 pts)
- TG-LI-004: Employment Signal Provider (3 pts)
- TG-LI-005: LinkedIn Integration Testing (5 pts)
- TG-LI-006: LinkedIn Badges & UI (5 pts)
- TG-ID-001: Persona API Integration (8 pts)
- TG-ID-002: Age Gate Implementation (3 pts, CRITICAL)
- TG-ID-003: Government ID Testing (5 pts)

---

## Post-Import Verification

### Checklist

- [ ] All 12 issues created successfully
- [ ] Epic TG-PLANE-B has 2 features + 9 stories
- [ ] All stories linked to correct feature/epic
- [ ] All labels include `ai-commander-ready`
- [ ] All due dates set correctly
- [ ] Story points assigned (total: 47 points for Sprint 1+2)
- [ ] Assignees set to correct engineers
- [ ] Sprints assigned (Sprint 1 and Sprint 2)
- [ ] Issues visible in AI Commander (wait 2 minutes)

### Verify in JIRA

```bash
# Check Epic
open https://YOUR-DOMAIN.atlassian.net/browse/TG-PLANE-B

# Check Sprint 1
open https://YOUR-DOMAIN.atlassian.net/software/c/projects/TG/boards/BOARD-ID?sprint=SPRINT-1

# Search by label
open https://YOUR-DOMAIN.atlassian.net/issues/?jql=labels=ai-commander-ready
# Should find 12 issues

# Search by assignee
open https://YOUR-DOMAIN.atlassian.net/issues/?jql=assignee=[Engineer%20A]
# Should find 5-6 issues
```

---

## AI Commander Ingestion Verification

After import, verify that **Aicc-Coordinator** can ingest the tasks:

### 1. Wait for First Poll

The Aicc-Coordinator polls JIRA every 2 minutes (configurable). Wait 2-5 minutes after import.

### 2. Check Aicc-Coordinator Logs

```bash
# If Aicc-Coordinator is running locally:
docker logs aicc-coordinator | grep "TG-PLANE-B"
# Should see: "ingested operation TG-PLANE-B"

# Or check via admin dashboard:
open http://localhost:8080/admin/operations
# Filter by source_tenant = "TG"
# Should see 12 operations (1 epic + 2 features + 9 stories)
```

### 3. Verify Task Status in AI Commander

```bash
# Tasks should appear as "operations" in Aicc-Coordinator
# Status flow: Ready for AI Commander → Claimed → In Progress → Resolved

# Check via API:
curl http://localhost:8080/operations \
  -H "Authorization: Bearer YOUR_TOKEN"
# Should return all ingested TrustGraph tasks
```

---

## Troubleshooting

### Issue: "Invalid Project Key"

**Solution:** Verify project key is `TG` in JIRA. If different:
1. Open JIRA Settings → Projects → Project Keys
2. Update CSV/JSON to use correct project key
3. Re-import

### Issue: "Unknown Issue Type"

**Solution:** Verify issue types exist in JIRA:
1. Go to Settings → Issue Types
2. Ensure these types exist:
   - Epic
   - Feature
   - Story
3. If not, create them or map to existing types

### Issue: "Invalid Custom Fields"

**Solution:** Custom fields in JIRA may have different IDs:
1. Go to Settings → Custom Fields
2. Find the field IDs for:
   - Story Points
   - Sprint
   - Target Start Date
   - Target End Date
3. Update JSON with correct `customfield_XXXXX` IDs

Example:
```json
"customfield_10004": 8,  // Story Points
"customfield_10003": "Sprint 1"  // Sprint
```

### Issue: "Aicc-Coordinator Not Ingesting"

**Solution:** Verify JIRA connector config:
```bash
# Check config:
env | grep AICC_JIRA

# Should show:
# AICC_JIRA_ENABLED=true
# AICC_JIRA_PROJECTS=TG
# AICC_JIRA_READY_STATUS=Ready for AI Commander
# AICC_JIRA_REQUIRED_LABEL=ai-commander-ready
```

If missing:
1. Set environment variables
2. Restart Aicc-Coordinator
3. Wait 2-5 minutes for next poll

---

## After Import: Next Steps

### 1. Assign Engineers

Replace placeholder assignees with real team members:
```bash
# In JIRA, bulk update:
# Find: assignee = "placeholder_engineer_a"
# Replace: assignee = "john.doe@company.com"
```

### 2. Create Sprints (if not exist)

```bash
# In JIRA Sprint Planning:
# Create "Sprint 1" (2025-03-10 to 2025-03-16)
# Create "Sprint 2" (2025-03-24 to 2025-03-30)
```

### 3. Add to Board

```bash
# Add tasks to board in JIRA:
# Settings → Board → Sprints → Add Sprint 1, Sprint 2
```

### 4. Start Sprint 1

```bash
# In JIRA Sprint Planning:
# Click "Start Sprint" for Sprint 1
# Set goal: "Implement LinkedIn OAuth integration"
```

### 5. Monitor in AI Commander

```bash
# Tasks are now visible to AI Commander
# Engineers can claim tasks: POST /operations/{id}/claim
# Track progress via dashboard
```

---

## Rollback (If Needed)

If import fails or needs to be redone:

```bash
# Delete all imported issues:
# In JIRA: Search for label:ai-commander-ready
# Bulk delete all issues
# Or delete the entire epic (will cascade delete)

# Then re-import:
# Start from "Import Method 1" step 1
```

---

## Summary

✅ **Sprint 1 & 2 tasks are ready for import**
- 12 total issues (1 epic + 2 features + 9 stories)
- 47 story points across both sprints
- All labeled for AI Commander ingestion
- Due dates: Sprint 1 ends 2025-03-16, Sprint 2 ends 2025-03-30

**Next:** Choose an import method above and execute. Tasks will be visible to AI Commander within 2-5 minutes after import.

