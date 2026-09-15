package osint

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// UsernameSearchTool checks whether a username exists on a set of well-known
// public sites by probing the profile URL. It is free and needs no credentials.
//
// Only sites with a stable, unauthenticated public profile URL are included.
// Probes are HEAD requests; some sites return 200 for a missing profile, so the
// tool treats "present" as a soft signal rather than proof of identity.
//
// Sites maps a display name to the base profile URL for that site. The
// username is appended as a path segment by buildProfileURL, which keeps the
// request pinned to the site's host.
type UsernameSearchTool struct {
	HTTPClient *http.Client
	Sites      map[string]string
	Timeout    time.Duration
	Logger     *zap.Logger
}

func NewUsernameSearchTool() *UsernameSearchTool {
	return &UsernameSearchTool{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Timeout:    20 * time.Second,
		Sites: map[string]string{
			"GitHub":     "https://github.com",
			"GitLab":     "https://gitlab.com",
			"Reddit":     "https://www.reddit.com/user",
			"Keybase":    "https://keybase.io",
			"Medium":     "https://medium.com/@",
			"Behance":    "https://www.behance.net",
			"SoundCloud": "https://soundcloud.com",
			"Twitch":     "https://www.twitch.tv",
			"Vimeo":      "https://vimeo.com",
			"AboutMe":    "https://about.me",
			"Pinterest":  "https://www.pinterest.com",
		},
	}
}

// buildProfileURL appends the username to a site's base URL as a path segment.
// url.JoinPath fixes the host from the base URL, so the caller-supplied
// username cannot redirect the request elsewhere.
func buildProfileURL(base, username string) (string, error) {
	if !strings.HasPrefix(base, "https://") && !strings.HasPrefix(base, "http://") {
		return "", fmt.Errorf("site base url must be absolute")
	}
	joined, err := url.JoinPath(base, username)
	if err != nil {
		return "", fmt.Errorf("build profile url: %w", err)
	}
	return joined, nil
}

func (t *UsernameSearchTool) Name() string { return "username_search" }

func (t *UsernameSearchTool) Description() string {
	return "Probe well-known public sites for a username's profile page. Free, no credentials required. Presence is a soft signal, not identity proof."
}

func (t *UsernameSearchTool) Configured() bool { return true }

// Run expects {"username": "..."}.
func (t *UsernameSearchTool) Run(ctx context.Context, query map[string]interface{}) (*Result, error) {
	start := time.Now()

	username, err := requireString(query, "username")
	if err != nil {
		return nil, err
	}
	// Inline allowlist check: CodeQL's Go request-forgery model treats a regexp
	// match as a barrier guard, so keeping it here (rather than in a helper)
	// lets the analysis see the sanitization on the value that reaches the URL.
	if !usernameRE.MatchString(username) {
		return nil, fmt.Errorf("username may only contain letters, digits, dot, underscore, and hyphen")
	}

	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()

	var (
		mu       sync.Mutex
		findings []Finding
		skipped  []string
		failed   []string
		wg       sync.WaitGroup
	)

	for site, template := range t.Sites {
		wg.Add(1)
		go func(site, template string) {
			defer wg.Done()
			profileURL, joinErr := buildProfileURL(template, username)
			if joinErr != nil {
				mu.Lock()
				skipped = append(skipped, site)
				mu.Unlock()
				return
			}
			outcome, probeErr := t.probe(ctx, profileURL)
			switch outcome {
			case probePresent:
				mu.Lock()
				findings = append(findings, Finding{
					Type:   "username_presence",
					Value:  profileURL,
					Source: site,
					Details: map[string]interface{}{
						"site":     site,
						"username": username,
					},
				})
				mu.Unlock()
			case probeError:
				// A timed-out or rejected probe is not evidence the profile is
				// absent; recording the site in `failed` keeps a connection error
				// from being read as a confirmed miss.
				t.logger().Warn("username probe failed",
					zap.String("site", site), zap.Error(probeErr))
				mu.Lock()
				failed = append(failed, site)
				mu.Unlock()
			}
		}(site, template)
	}
	wg.Wait()

	summary := map[string]interface{}{
		"sitesChecked": len(t.Sites) - len(skipped) - len(failed),
		"sitesMatched": len(findings),
		"sitesSkipped": len(skipped),
		"sitesFailed":  len(failed),
	}
	if len(failed) > 0 {
		// The result is incomplete: some sites were never actually checked, so a
		// zero-finding answer must not be read as "no profiles found".
		summary["incomplete"] = true
		summary["failedSites"] = failed
	}

	return &Result{
		Tool:     t.Name(),
		Query:    map[string]interface{}{"username": username},
		Summary:  summary,
		Count:    len(findings),
		Findings: findings,
		Duration: time.Since(start),
	}, nil
}

// probeOutcome distinguishes a confirmed absence from a probe that never
// produced an answer.
type probeOutcome int

const (
	probeAbsent probeOutcome = iota
	probePresent
	probeError
)

// probe performs one HEAD request. A transport failure or non-2xx status is
// returned as probeError so the caller can report the site as unchecked rather
// than as a missing profile.
func (t *UsernameSearchTool) probe(ctx context.Context, profileURL string) (probeOutcome, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, profileURL, nil)
	if err != nil {
		return probeError, err
	}
	req.Header.Set("User-Agent", "TrustGraph-OSINT/1.0 (+investigation)")

	resp, err := t.httpClient().Do(req)
	if err != nil {
		return probeError, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return probePresent, nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return probeAbsent, nil
	default:
		return probeError, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
}

func (t *UsernameSearchTool) logger() *zap.Logger {
	if t.Logger != nil {
		return t.Logger
	}
	return zap.NewNop()
}

func (t *UsernameSearchTool) httpClient() *http.Client {
	if t.HTTPClient != nil {
		return t.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}
