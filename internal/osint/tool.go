// Package osint implements the Plane C open-source-intelligence tools. Every
// tool takes a query and returns a normalised result the investigation API can
// store and audit. Tools that need a third-party service report
// ErrToolNotConfigured rather than returning an empty-but-successful result,
// so an investigator is never shown "no findings" when the tool simply could
// not run.
package osint

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ErrToolNotConfigured is returned by a tool that has no credentials or no
// executable available.
var ErrToolNotConfigured = errors.New("osint tool is not configured")

// Result is the normalised output of a tool call.
type Result struct {
	Tool     string                 `json:"tool"`
	Query    map[string]interface{} `json:"query"`
	Summary  map[string]interface{} `json:"summary"`
	Count    int                    `json:"count"`
	Findings []Finding              `json:"findings,omitempty"`
	CostUSD  float64                `json:"costUsd"`
	Duration time.Duration          `json:"duration"`
}

// Finding is one discrete observation returned by a tool.
type Finding struct {
	Type       string                 `json:"type"`
	Value      string                 `json:"value"`
	Confidence float64                `json:"confidence,omitempty"`
	Source     string                 `json:"source,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// Tool is the interface every OSINT tool implements.
type Tool interface {
	// Name is the stable identifier recorded in the audit trail.
	Name() string
	// Description explains what the tool does and any cost/legal considerations.
	Description() string
	// Configured reports whether the tool can run.
	Configured() bool
	// Run executes the query. query is tool-specific.
	Run(ctx context.Context, query map[string]interface{}) (*Result, error)
}

// Registry holds the configured tools, keyed by name.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry builds a registry from the given tools.
func NewRegistry(tools ...Tool) *Registry {
	r := &Registry{tools: make(map[string]Tool, len(tools))}
	for _, t := range tools {
		r.tools[t.Name()] = t
	}
	return r
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns tool metadata for the discovery endpoint. Unconfigured tools are
// included with configured=false so an operator can see what is available but
// not yet wired up.
func (r *Registry) List() []ToolInfo {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	infos := make([]ToolInfo, 0, len(names))
	for _, name := range names {
		t := r.tools[name]
		infos = append(infos, ToolInfo{
			Name:        t.Name(),
			Description: t.Description(),
			Configured:  t.Configured(),
		})
	}
	return infos
}

// ToolInfo is tool metadata for the API.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Configured  bool   `json:"configured"`
}

// requireString extracts a required string query parameter.
func requireString(query map[string]interface{}, key string) (string, error) {
	raw, ok := query[key]
	if !ok {
		return "", fmt.Errorf("query parameter %q is required", key)
	}
	s, ok := raw.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("query parameter %q must be a non-empty string", key)
	}
	return strings.TrimSpace(s), nil
}
