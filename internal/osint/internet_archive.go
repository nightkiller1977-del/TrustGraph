package osint

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// InternetArchiveTool queries the Wayback Machine CDX API for historical
// snapshots of a URL. It is free and needs no credentials, so it is always
// configured; the base URL is overridable for tests.
type InternetArchiveTool struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewInternetArchiveTool(baseURL string) *InternetArchiveTool {
	if baseURL == "" {
		baseURL = "https://web.archive.org/cdx/search/cdx"
	}
	return &InternetArchiveTool{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *InternetArchiveTool) Name() string { return "internet_archive" }

func (t *InternetArchiveTool) Description() string {
	return "Query the Wayback Machine CDX API for historical snapshots of a URL. Free, no credentials required."
}

// Configured is always true: the Wayback Machine needs no API key.
func (t *InternetArchiveTool) Configured() bool { return true }

// Run expects {"url": "..."} and optionally {"limit": n}.
func (t *InternetArchiveTool) Run(ctx context.Context, query map[string]interface{}) (*Result, error) {
	start := time.Now()

	target, err := requireString(query, "url")
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(target)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("url must be an absolute http(s) URL")
	}
	if parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("url must be an absolute http(s) URL without userinfo")
	}

	limit := 50
	if raw, ok := query["limit"]; ok {
		if n, ok := toInt(raw); ok && n > 0 && n <= 500 {
			limit = n
		}
	}

	endpoint, err := queryURL(t.BaseURL, map[string]string{
		"url":      target,
		"output":   "json",
		"limit":    fmt.Sprintf("%d", limit),
		"collapse": "digest",
	})
	if err != nil {
		return nil, fmt.Errorf("build archive url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build archive request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := t.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("archive request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, fmt.Errorf("read archive response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("archive returned %d", resp.StatusCode)
	}

	findings, err := parseCDX(body)
	if err != nil {
		return nil, err
	}

	return &Result{
		Tool:     t.Name(),
		Query:    map[string]interface{}{"url": target, "limit": limit},
		Summary:  map[string]interface{}{"snapshotCount": len(findings), "target": target},
		Count:    len(findings),
		Findings: findings,
		Duration: time.Since(start),
	}, nil
}

// parseCDX decodes the CDX JSON format, which is an array of arrays whose first
// row is the header. Snapshots are reported oldest-first by the archive; we
// return them as given so the caller sees the timeline in order.
func parseCDX(body []byte) ([]Finding, error) {
	var rows [][]string
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("decode archive response: %w", err)
	}
	if len(rows) == 0 {
		return []Finding{}, nil
	}

	header := rows[0]
	index := map[string]int{}
	for i, h := range header {
		index[strings.ToLower(h)] = i
	}

	findings := make([]Finding, 0, len(rows)-1)
	for _, row := range rows[1:] {
		timestamp := at(row, index, "timestamp")
		original := at(row, index, "original")
		statusCode := at(row, index, "statuscode")
		digest := at(row, index, "digest")

		// The snapshot URL is deterministic from the timestamp + original.
		snapshotURL := fmt.Sprintf("https://web.archive.org/web/%s/%s", timestamp, original)
		findings = append(findings, Finding{
			Type:   "archived_snapshot",
			Value:  snapshotURL,
			Source: "internet_archive",
			Details: map[string]interface{}{
				"timestamp":  timestamp,
				"original":   original,
				"statusCode": statusCode,
				"digest":     digest,
			},
		})
	}
	return findings, nil
}

func at(row []string, index map[string]int, key string) string {
	i, ok := index[key]
	if !ok || i >= len(row) {
		return ""
	}
	return row[i]
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func (t *InternetArchiveTool) httpClient() *http.Client {
	if t.HTTPClient != nil {
		return t.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}
