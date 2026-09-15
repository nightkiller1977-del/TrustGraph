package osint

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the SSRF defenses: caller-supplied input must never be able
// to change which host an outbound request is sent to.

func TestValidateHostname_RejectsURLSyntax(t *testing.T) {
	bad := []string{
		"evil.com#@good.com",
		"good.com?x=@evil.com",
		"good.com\\@evil.com",
		"evil.com/path",
		"evil.com:8080",
		"localhost",
		"127.0.0.1",
		"[::1]",
		"good.com%2f..%2fevil",
		"good.com\nHost: evil.com",
		"user@evil.com",
		"",
		"-leading.com",
		"trailing-.com",
	}
	for _, in := range bad {
		_, err := validateHostname(in)
		assert.Error(t, err, "hostname %q must be rejected", in)
	}
}

func TestValidateHostname_AcceptsBareHostname(t *testing.T) {
	got, err := validateHostname("WWW.Example.COM")
	require.NoError(t, err)
	assert.Equal(t, "example.com", got)
}

func TestValidateUsername_RejectsURLDelimiters(t *testing.T) {
	for _, in := range []string{"bad user", "a/b", "a?b", "a#b", "a@b", "a%b", "a:b", "", strings.Repeat("a", 65)} {
		_, err := validateUsername(in)
		assert.Error(t, err, "username %q must be rejected", in)
	}
	got, err := validateUsername("alice.bob-1_2")
	require.NoError(t, err)
	assert.Equal(t, "alice.bob-1_2", got)
}

// buildProfileURL must keep the request on the site's host, no matter what the
// username contains and whether or not the base carries a trailing path.
func TestBuildProfileURL_StaysOnBaseHost(t *testing.T) {
	ok := []struct{ base, user, want string }{
		{"https://github.com", "alice", "https://github.com/alice"},
		{"https://medium.com/@", "alice", "https://medium.com/@/alice"},
		{"https://www.reddit.com/user", "alice", "https://www.reddit.com/user/alice"},
	}
	for _, c := range ok {
		got, err := buildProfileURL(c.base, c.user)
		require.NoError(t, err)
		assert.Equal(t, c.want, got)
	}

	for _, base := range []string{"https://github.com", "https://medium.com/@", "https://www.reddit.com/user"} {
		for _, user := range []string{"..", "../..", "..%2F..%2Fevil", "a/b", "@evil.com", "evil.com", "a:b", "a?x=1", "a#f"} {
			got, err := buildProfileURL(base, user)
			if err != nil {
				continue
			}
			parsed, perr := url.Parse(got)
			require.NoError(t, perr)
			want, _ := url.Parse(base)
			assert.Equal(t, want.Host, parsed.Host, "username %q escaped host via base %q", user, base)
			assert.Nil(t, parsed.User, "username %q injected userinfo", user)
		}
	}
}

func TestBuildProfileURL_RejectsRelativeBase(t *testing.T) {
	_, err := buildProfileURL("/github.com", "alice")
	assert.Error(t, err)
}

// The hostile-domain regression: a domain carrying URL syntax used to pass the
// old blocklist and reach the request builder.
func TestDomainIntelTool_RejectsHostileDomainSyntax(t *testing.T) {
	tool := NewDomainIntelTool("", "")
	for _, d := range []string{
		"evil.com#@good.com",
		"good.com?x=1&y=2",
		"good.com\\@evil.com",
		"evil.com:8080",
		"not a domain",
	} {
		_, err := tool.Run(context.Background(), map[string]interface{}{"domain": d})
		assert.Error(t, err, "domain %q must be rejected", d)
	}
}

// End-to-end: every request the tool makes must land on the configured base
// host, no matter what the caller passes.
func TestDomainIntelTool_AllRequestsStayOnConfiguredHost(t *testing.T) {
	var hosts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hosts = append(hosts, r.Host)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tool := NewDomainIntelTool(server.URL+"/", server.URL)
	tool.HTTPClient = server.Client()
	_, err := tool.Run(context.Background(), map[string]interface{}{"domain": "example.com"})
	require.NoError(t, err)

	base, err := url.Parse(server.URL)
	require.NoError(t, err)
	require.NotEmpty(t, hosts)
	for _, h := range hosts {
		assert.Equal(t, base.Host, h, "request escaped the configured host")
	}
}

func TestDNSQuery_BuildEscapesParameters(t *testing.T) {
	got, err := queryURL("https://dns.google/resolve", map[string]string{"name": "example.com", "type": "A"})
	require.NoError(t, err)
	parsed, err := url.Parse(got)
	require.NoError(t, err)
	assert.Equal(t, "dns.google", parsed.Host)
	assert.Equal(t, "example.com", parsed.Query().Get("name"))
	assert.Equal(t, "A", parsed.Query().Get("type"))
	assert.Empty(t, parsed.Fragment)
}

func TestInternetArchiveTool_RejectsUserinfoURL(t *testing.T) {
	tool := NewInternetArchiveTool("")
	_, err := tool.Run(context.Background(), map[string]interface{}{"url": "http://user@example.com/"})
	assert.Error(t, err)
}

func TestInternetArchiveTool_TargetStaysInQuery(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	tool := NewInternetArchiveTool(server.URL)
	_, err := tool.Run(context.Background(), map[string]interface{}{"url": "http://example.com/a?b=c&d=e"})
	require.NoError(t, err)
	parsed, err := url.ParseQuery(gotQuery)
	require.NoError(t, err)
	assert.Equal(t, "http://example.com/a?b=c&d=e", parsed.Get("url"))
	assert.Equal(t, "json", parsed.Get("output"))
}
