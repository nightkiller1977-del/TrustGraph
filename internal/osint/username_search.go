package osint

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// UsernameSearchTool checks whether a username exists on a set of well-known
// public sites by probing the profile URL. It is free and needs no credentials.
//
// Only sites with a stable, unauthenticated public profile URL are included.
// Probes are GET requests; some sites return 200 for a missing profile, so the
// tool treats "present" as a soft signal rather than proof of identity.
type UsernameSearchTool struct {
	HTTPClient *http.Client
	Sites      map[string]string // name -> URL template with %s for the username
	Timeout    time.Duration
}

func NewUsernameSearchTool() *UsernameSearchTool {
	return &UsernameSearchTool{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Timeout:    20 * time.Second,
		Sites: map[string]string{
			"GitHub":     "https://github.com/%s",
			"GitLab":     "https://gitlab.com/%s",
			"Reddit":     "https://www.reddit.com/user/%s",
			"Keybase":    "https://keybase.io/%s",
			"Medium":     "https://medium.com/@%s",
			"Behance":    "https://www.behance.net/%s",
			"SoundCloud": "https://soundcloud.com/%s",
			"Twitch":     "https://www.twitch.tv/%s",
			"Vimeo":      "https://vimeo.com/%s",
			"AboutMe":    "https://about.me/%s",
			"Pinterest":  "https://www.pinterest.com/%s",
			"Tumblr":     "https://%s.tumblr.com",
		},
	}
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
	username, err = validateUsername(username)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()

	var (
		mu       sync.Mutex
		findings []Finding
		skipped  []string
		wg       sync.WaitGroup
	)

	for site, template := range t.Sites {
		wg.Add(1)
		go func(site, template string) {
			defer wg.Done()
			profileURL := fmt.Sprintf(template, url.PathEscape(username))
			if err := assertHostPinned(template, profileURL); err != nil {
				mu.Lock()
				skipped = append(skipped, site)
				mu.Unlock()
				return
			}
			if t.exists(ctx, profileURL) {
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
			}
		}(site, template)
	}
	wg.Wait()

	return &Result{
		Tool:     t.Name(),
		Query:    map[string]interface{}{"username": username},
		Summary:  map[string]interface{}{"sitesChecked": len(t.Sites), "sitesMatched": len(findings), "sitesSkipped": len(skipped)},
		Count:    len(findings),
		Findings: findings,
		Duration: time.Since(start),
	}, nil
}

func (t *UsernameSearchTool) exists(ctx context.Context, profileURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, profileURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "TrustGraph-OSINT/1.0 (+investigation)")

	resp, err := t.httpClient().Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (t *UsernameSearchTool) httpClient() *http.Client {
	if t.HTTPClient != nil {
		return t.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}
