package osint

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// HarvesterTool wraps the external theHarvester utility, which enumerates
// emails, subdomains, and hosts for a domain from public sources.
//
// theHarvester is a separate binary, not a library, so this tool shells out to
// it. Two consequences are handled deliberately:
//   - It is reported as unconfigured when the binary is absent, rather than
//     failing at query time with a confusing error.
//   - Arguments are passed as a slice (never a shell string), so a domain that
//     contains shell metacharacters cannot escape into the shell.
type HarvesterTool struct {
	BinaryPath string
	Timeout    time.Duration
}

func NewHarvesterTool(binaryPath string) *HarvesterTool {
	if binaryPath == "" {
		binaryPath = "theHarvester"
	}
	return &HarvesterTool{BinaryPath: binaryPath, Timeout: 120 * time.Second}
}

func (t *HarvesterTool) Name() string { return "theharvester" }

func (t *HarvesterTool) Description() string {
	return "Enumerate emails, subdomains, and hosts for a domain using the external theHarvester utility. Requires the theHarvester binary."
}

// Configured reports whether the binary is on PATH.
func (t *HarvesterTool) Configured() bool {
	if t.BinaryPath == "" {
		return false
	}
	_, err := exec.LookPath(t.BinaryPath)
	return err == nil
}

// Run expects {"domain": "example.com"} and optionally {"sources": "bing"}.
func (t *HarvesterTool) Run(ctx context.Context, query map[string]interface{}) (*Result, error) {
	start := time.Now()

	if !t.Configured() {
		return nil, ErrToolNotConfigured
	}

	domain, err := requireString(query, "domain")
	if err != nil {
		return nil, err
	}
	if !strings.Contains(domain, ".") || strings.ContainsAny(domain, " /;|&$`") {
		return nil, fmt.Errorf("domain must be a bare hostname such as example.com")
	}

	args := []string{"-d", domain, "-b", "0"}
	if raw, ok := query["sources"]; ok {
		if sources, ok := raw.(string); ok && sources != "" {
			if strings.ContainsAny(sources, " ;|&$`") {
				return nil, fmt.Errorf("sources must not contain shell metacharacters")
			}
			args = append(args, "-b", sources)
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, t.BinaryPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("theharvester failed: %w", err)
	}

	findings := parseHarvesterOutput(string(output))
	return &Result{
		Tool:     t.Name(),
		Query:    map[string]interface{}{"domain": domain},
		Summary:  map[string]interface{}{"domain": domain, "findingCount": len(findings)},
		Count:    len(findings),
		Findings: findings,
		Duration: time.Since(start),
	}, nil
}

// parseHarvesterOutput reads theHarvester's "[section]" style text output.
func parseHarvesterOutput(output string) []Finding {
	findings := make([]Finding, 0)
	section := ""
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.ToLower(strings.Trim(trimmed, "[]"))
			continue
		}
		if strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "-") {
			value := strings.TrimSpace(strings.TrimLeft(trimmed, "*-"))
			if value == "" {
				continue
			}
			findings = append(findings, Finding{
				Type:   harvesterSectionType(section),
				Value:  value,
				Source: "theharvester",
				Details: map[string]interface{}{
					"section": section,
				},
			})
		}
	}
	return findings
}

func harvesterSectionType(section string) string {
	switch {
	case strings.Contains(section, "email"):
		return "email"
	case strings.Contains(section, "host"):
		return "host"
	case strings.Contains(section, "ip"):
		return "ip_address"
	default:
		return "harvester_result"
	}
}
