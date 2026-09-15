package osint

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_ListAndGet(t *testing.T) {
	tool := NewInternetArchiveTool("")
	registry := NewRegistry(tool)

	got, ok := registry.Get("internet_archive")
	assert.True(t, ok)
	assert.Equal(t, tool, got)

	_, ok = registry.Get("nope")
	assert.False(t, ok)

	list := registry.List()
	require.Len(t, list, 1)
	assert.True(t, list[0].Configured)
}

func TestRegistry_ListIsSorted(t *testing.T) {
	registry := NewRegistry(
		NewSpiderFootTool("", "", ""),
		NewInternetArchiveTool(""),
		NewDomainIntelTool("", ""),
	)
	list := registry.List()
	require.Len(t, list, 3)
	assert.Equal(t, "domain_intel", list[0].Name)
	assert.Equal(t, "internet_archive", list[1].Name)
	assert.Equal(t, "spiderfoot", list[2].Name)
	// SpiderFoot has neither server nor binary in a test env.
	assert.False(t, list[2].Configured)
}

func TestInternetArchiveTool_ParsesCDX(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "url=")
		_ = json.NewEncoder(w).Encode([][]string{
			{"timestamp", "original", "statuscode", "digest"},
			{"20100101000000", "http://example.com/", "200", "ABC123"},
			{"20200101000000", "http://example.com/", "200", "DEF456"},
		})
	}))
	defer server.Close()

	tool := NewInternetArchiveTool(server.URL)
	result, err := tool.Run(context.Background(), map[string]interface{}{"url": "http://example.com/"})

	require.NoError(t, err)
	assert.Equal(t, 2, result.Count)
	assert.Contains(t, result.Findings[0].Value, "web.archive.org")
	assert.Equal(t, "20100101000000", result.Findings[0].Details["timestamp"])
}

func TestInternetArchiveTool_RejectsNonHTTPURL(t *testing.T) {
	tool := NewInternetArchiveTool("")
	_, err := tool.Run(context.Background(), map[string]interface{}{"url": "ftp://example.com"})
	assert.Error(t, err)
}

func TestInternetArchiveTool_ExplicitEmptyCDX(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	tool := NewInternetArchiveTool(server.URL)
	result, err := tool.Run(context.Background(), map[string]interface{}{"url": "http://example.com/"})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Findings)
}

func TestDomainIntelTool_GathersRDAPAndDNS(t *testing.T) {
	dnsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		typeName := r.URL.Query().Get("type")
		var answers []map[string]interface{}
		switch typeName {
		case "A":
			answers = []map[string]interface{}{{"name": "example.com", "type": 1, "data": "93.184.216.34"}}
		case "MX":
			answers = []map[string]interface{}{{"name": "example.com", "type": 15, "data": "mail.example.com"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"Status": 0, "Answer": answers})
	}))
	defer dnsServer.Close()

	rdapServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ldhName": "EXAMPLE.COM",
			"status":  []string{"client transfer prohibited"},
			"events": []map[string]string{
				{"eventAction": "registration", "eventDate": "1995-08-14T04:00:00Z"},
			},
			"nameservers": []map[string]string{{"ldhName": "A.IANA-SERVERS.NET"}},
		})
	}))
	defer rdapServer.Close()

	tool := NewDomainIntelTool(rdapServer.URL+"/", dnsServer.URL)
	result, err := tool.Run(context.Background(), map[string]interface{}{"domain": "Example.com"})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Count, 4)

	types := map[string]int{}
	for _, f := range result.Findings {
		types[f.Type]++
	}
	assert.Equal(t, 1, types["domain_event"])
	assert.Equal(t, 1, types["nameserver"])
	assert.Equal(t, 2, types["dns_record"])
}

func TestDomainIntelTool_RejectsMalformedDomain(t *testing.T) {
	tool := NewDomainIntelTool("", "")
	_, err := tool.Run(context.Background(), map[string]interface{}{"domain": "not a domain"})
	assert.Error(t, err)
}

func TestHarvesterTool_NotConfiguredWithoutBinary(t *testing.T) {
	tool := NewHarvesterTool("definitely-not-a-real-binary-xyz")
	assert.False(t, tool.Configured())
	_, err := tool.Run(context.Background(), map[string]interface{}{"domain": "example.com"})
	assert.ErrorIs(t, err, ErrToolNotConfigured)
}

func TestParseHarvesterOutput(t *testing.T) {
	output := `[Emails found:]
* alice@example.com
* bob@example.com

[Hosts found:]
* www.example.com
`
	findings := parseHarvesterOutput(output)
	require.Len(t, findings, 3)
	assert.Equal(t, "email", findings[0].Type)
	assert.Equal(t, "alice@example.com", findings[0].Value)
	assert.Equal(t, "host", findings[2].Type)
}

func TestSpiderFootTool_CLIParsesJSON(t *testing.T) {
	findings := parseSpiderFootJSON([]byte(`[{"type":"EMAILADDR","data":"a@b.com","module":"sfp_x"}]`))
	require.Len(t, findings, 1)
	assert.Equal(t, "EMAILADDR", findings[0].Type)
	assert.Equal(t, "a@b.com", findings[0].Value)
}

func TestSpiderFootTool_CLIFallsBackToLines(t *testing.T) {
	findings := parseSpiderFootJSON([]byte("line one\nline two\n"))
	require.Len(t, findings, 2)
	assert.Equal(t, "spiderfoot_result", findings[0].Type)
}

func TestSpiderFootTool_ServerMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer key", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"scanId": "scan-1", "status": "running"})
	}))
	defer server.Close()

	tool := NewSpiderFootTool(server.URL, "key", "")
	assert.True(t, tool.Configured())

	result, err := tool.Run(context.Background(), map[string]interface{}{"target": "example.com"})
	require.NoError(t, err)
	assert.Equal(t, "scan-1", result.Findings[0].Value)
	assert.Equal(t, "server", result.Summary["mode"])
}

func TestSpiderFootTool_RejectsShellMetacharacters(t *testing.T) {
	tool := NewSpiderFootTool("", "", "")
	// Even unconfigured, the guard runs first if configured; force server mode.
	tool.BaseURL = "https://sf.example"
	_, err := tool.Run(context.Background(), map[string]interface{}{"target": "example.com; rm -rf /"})
	assert.Error(t, err)
}

func TestUsernameSearchTool_RejectsMetacharacters(t *testing.T) {
	tool := NewUsernameSearchTool()
	_, err := tool.Run(context.Background(), map[string]interface{}{"username": "bad user"})
	assert.Error(t, err)
}

func TestUsernameSearchTool_FindsMatchingSite(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodHead, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer site.Close()

	tool := NewUsernameSearchTool()
	tool.Sites = map[string]string{"TestSite": site.URL + "/%s"}
	tool.Timeout = 5 * time.Second

	result, err := tool.Run(context.Background(), map[string]interface{}{"username": "alice"})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Count)
	assert.Equal(t, "TestSite", result.Findings[0].Source)
}
