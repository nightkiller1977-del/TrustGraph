package osint

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// These validators exist because tool input is attacker-influenced: an
// investigator (or anyone who can reach an investigation endpoint) chooses the
// domain or username, and that value is interpolated into an outbound request
// URL. A blocklist is not enough -- for example, `evil.com#@good.com` passes a
// "no spaces or slashes" check and still redirects the request. We therefore
// accept only a strict allowlist of characters and then re-verify that the
// URL we built still points at the host we intended.

var (
	hostnameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)
	usernameRE = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

// validateHostname lowercases and validates a bare DNS hostname such as
// "example.com". It rejects anything carrying URL syntax (scheme, userinfo,
// port, path, query, fragment, escapes) so the value cannot change which host
// a request is sent to.
func validateHostname(raw string) (string, error) {
	host := strings.ToLower(strings.TrimSpace(raw))
	host = strings.TrimPrefix(host, "www.")
	if len(host) > 253 || !hostnameRE.MatchString(host) {
		return "", fmt.Errorf("domain must be a bare hostname such as example.com")
	}
	// Reject IP literals: these tools expect a registrable domain, and letting
	// an address through would invite probing internal ranges.
	if net.ParseIP(host) != nil {
		return "", fmt.Errorf("domain must be a hostname, not an IP address")
	}
	return host, nil
}

// validateUsername validates a username before it is substituted into a
// profile-URL template. The allowed set excludes every character that could
// alter the URL's authority, path, query, or fragment.
func validateUsername(raw string) (string, error) {
	user := strings.TrimSpace(raw)
	if !usernameRE.MatchString(user) {
		return "", fmt.Errorf("username may only contain letters, digits, dot, underscore, and hyphen")
	}
	return user, nil
}

const hostProbe = "trustgraphprobe"

// assertHostPinned checks that built points at the host its template intends.
// Templates are either path-style (https://github.com/%s) or subdomain-style
// (https://%s.tumblr.com); in both cases the substituted value must not move
// the request off the template's host.
func assertHostPinned(template, built string) error {
	probeURL, err := url.Parse(strings.Replace(template, "%s", hostProbe, 1))
	if err != nil {
		return fmt.Errorf("invalid url template: %w", err)
	}
	got, err := url.Parse(built)
	if err != nil {
		return fmt.Errorf("invalid profile url: %w", err)
	}
	if got.Scheme != "http" && got.Scheme != "https" {
		return fmt.Errorf("profile url must be http(s)")
	}
	if got.User != nil {
		return fmt.Errorf("profile url must not carry userinfo")
	}
	if strings.Contains(probeURL.Host, hostProbe) {
		suffix := strings.TrimPrefix(probeURL.Host, hostProbe)
		if suffix == "" || !strings.HasSuffix(got.Host, suffix) || got.Host == strings.TrimPrefix(suffix, ".") {
			return fmt.Errorf("profile url host %q is outside template host", got.Host)
		}
		return nil
	}
	if got.Host != probeURL.Host {
		return fmt.Errorf("profile url host %q does not match template host %q", got.Host, probeURL.Host)
	}
	return nil
}

// assertSameHost verifies that built stayed on the same host and scheme as
// base. It guards endpoints whose path is built by joining a caller-supplied
// segment onto a pinned base.
func assertSameHost(base, built string) error {
	b, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("invalid base url: %w", err)
	}
	g, err := url.Parse(built)
	if err != nil {
		return fmt.Errorf("invalid request url: %w", err)
	}
	if g.Scheme != b.Scheme || g.Host != b.Host {
		return fmt.Errorf("request url host %q is outside configured host %q", g.Host, b.Host)
	}
	if g.User != nil {
		return fmt.Errorf("request url must not carry userinfo")
	}
	return nil
}

// queryURL joins a base endpoint with query parameters, escaping each value so
// that caller-supplied data cannot introduce extra parameters or a fragment.
func queryURL(base string, params map[string]string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	u.Fragment = ""
	return u.String(), nil
}
