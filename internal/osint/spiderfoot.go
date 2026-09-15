package osint

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// SpiderFootTool integrates SpiderFoot, an OSINT automation platform. Two
// deployment shapes are supported:
//
//   - server: call the SpiderFoot REST API when SpiderFootBaseURL is set.
//   - cli:    shell out to the SpiderFoot CLI when only a binary is available.
//
// The server form is preferred because it does not require the binary on the
// API host. When neither is available the tool reports unconfigured.
type SpiderFootTool struct {
	BaseURL    string
	APIKey     string
	BinaryPath string
	Timeout    time.Duration
	HTTPClient *http.Client
}

func NewSpiderFootTool(baseURL, apiKey, binaryPath string) *SpiderFootTool {
	return &SpiderFootTool{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     apiKey,
		BinaryPath: binaryPath,
		Timeout:    180 * time.Second,
		HTTPClient: &http.Client{Timeout: 180 * time.Second},
	}
}

func (t *SpiderFootTool) Name() string { return "spiderfoot" }

func (t *SpiderFootTool) Description() string {
	return "Run an OSINT scan against a target using a SpiderFoot server or the SpiderFoot CLI."
}

// Configured reports whether either the server or the CLI is available.
func (t *SpiderFootTool) Configured() bool {
	if t.BaseURL != "" {
		return true
	}
	if t.BinaryPath == "" {
		return false
	}
	_, err := exec.LookPath(t.BinaryPath)
	return err == nil
}

// Run expects {"target": "example.com"} (or an email/username).
func (t *SpiderFootTool) Run(ctx context.Context, query map[string]interface{}) (*Result, error) {
	start := time.Now()

	if !t.Configured() {
		return nil, ErrToolNotConfigured
	}

	target, err := requireString(query, "target")
	if err != nil {
		return nil, err
	}
	if strings.ContainsAny(target, " ;|&$`") {
		return nil, fmt.Errorf("target must not contain shell metacharacters")
	}

	if t.BaseURL != "" {
		return t.runServer(ctx, target, start)
	}
	return t.runCLI(ctx, target, start)
}

type spiderfootScanRequest struct {
	ScanName string `json:"scanName"`
	Target   string `json:"target"`
	Type     string `json:"type"`
}

type spiderfootScanResponse struct {
	ScanID string `json:"scanId"`
	Status string `json:"status"`
}

func (t *SpiderFootTool) runServer(ctx context.Context, target string, start time.Time) (*Result, error) {
	payload, err := json.Marshal(spiderfootScanRequest{
		ScanName: "trustgraph_investigation",
		Target:   target,
		Type:     "all",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal spiderfoot request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.BaseURL+"/api/v1/scan", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build spiderfoot request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if t.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.APIKey)
	}

	resp, err := t.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("spiderfoot request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read spiderfoot response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("spiderfoot returned %d: %s", resp.StatusCode, string(body))
	}

	var decoded spiderfootScanResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode spiderfoot response: %w", err)
	}

	return &Result{
		Tool:  t.Name(),
		Query: map[string]interface{}{"target": target},
		Summary: map[string]interface{}{
			"scanId": decoded.ScanID,
			"status": decoded.Status,
			"mode":   "server",
		},
		// A server scan is asynchronous: the scanId is the finding, and results
		// are pulled later. Reporting count 1 reflects that one scan was started.
		Count: 1,
		Findings: []Finding{{
			Type:   "spiderfoot_scan",
			Value:  decoded.ScanID,
			Source: "spiderfoot",
			Details: map[string]interface{}{
				"status": decoded.Status,
				"target": target,
			},
		}},
		Duration: time.Since(start),
	}, nil
}

func (t *SpiderFootTool) runCLI(ctx context.Context, target string, start time.Time) (*Result, error) {
	runCtx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()

	// -s target, -q (quiet), -o json to keep the output parseable.
	cmd := exec.CommandContext(runCtx, t.BinaryPath, "-s", target, "-q", "-o", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("spiderfoot cli failed: %w", err)
	}

	findings := parseSpiderFootJSON(output)
	return &Result{
		Tool:     t.Name(),
		Query:    map[string]interface{}{"target": target},
		Summary:  map[string]interface{}{"target": target, "mode": "cli", "findingCount": len(findings)},
		Count:    len(findings),
		Findings: findings,
		Duration: time.Since(start),
	}, nil
}

// parseSpiderFootJSON decodes SpiderFoot's JSON output (a list of events) and
// falls back to line-based parsing when the output is not valid JSON.
func parseSpiderFootJSON(output []byte) []Finding {
	var events []struct {
		Type   string `json:"type"`
		Data   string `json:"data"`
		Module string `json:"module"`
	}
	if err := json.Unmarshal(output, &events); err == nil {
		findings := make([]Finding, 0, len(events))
		for _, e := range events {
			if e.Data == "" {
				continue
			}
			findings = append(findings, Finding{
				Type:   e.Type,
				Value:  e.Data,
				Source: "spiderfoot",
				Details: map[string]interface{}{
					"module": e.Module,
				},
			})
		}
		return findings
	}

	findings := make([]Finding, 0)
	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		findings = append(findings, Finding{
			Type:   "spiderfoot_result",
			Value:  trimmed,
			Source: "spiderfoot",
		})
	}
	return findings
}

func (t *SpiderFootTool) httpClient() *http.Client {
	if t.HTTPClient != nil {
		return t.HTTPClient
	}
	return &http.Client{Timeout: 180 * time.Second}
}
