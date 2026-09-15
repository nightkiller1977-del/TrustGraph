package osint

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// DomainIntelTool gathers registration and DNS intelligence for a domain. It
// uses RDAP (the modern WHOIS replacement) for registration data and Google's
// public DNS-over-HTTPS resolver for A/MX/NS/TXT records. Both are free and
// need no API key, so the tool is always configured. RDAP responses contain no
// personal registrant data for privacy-protected domains, and any that is
// present is summarised rather than copied wholesale.
type DomainIntelTool struct {
	RDAPBaseURL string
	DNSURL      string
	HTTPClient  *http.Client
}

func NewDomainIntelTool(rdapBaseURL, dnsURL string) *DomainIntelTool {
	if rdapBaseURL == "" {
		rdapBaseURL = "https://rdap.org/domain/"
	}
	if dnsURL == "" {
		dnsURL = "https://dns.google/resolve"
	}
	return &DomainIntelTool{
		RDAPBaseURL: rdapBaseURL,
		DNSURL:      dnsURL,
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *DomainIntelTool) Name() string { return "domain_intel" }

func (t *DomainIntelTool) Description() string {
	return "Registration (RDAP) and DNS (A/MX/NS/TXT) intelligence for a domain. Free, no credentials required."
}

func (t *DomainIntelTool) Configured() bool { return true }

// Run expects {"domain": "example.com"}.
func (t *DomainIntelTool) Run(ctx context.Context, query map[string]interface{}) (*Result, error) {
	start := time.Now()

	domain, err := requireString(query, "domain")
	if err != nil {
		return nil, err
	}
	domain, err = validateHostname(domain)
	if err != nil {
		return nil, err
	}

	findings := make([]Finding, 0)
	summary := map[string]interface{}{"domain": domain}

	rdap, err := t.fetchRDAP(ctx, domain)
	if err != nil {
		summary["rdapError"] = err.Error()
	} else if rdap != nil {
		findings = append(findings, rdap...)
		summary["registrar"] = extractRegistrar(rdap)
	}

	dnsFindings, err := t.fetchDNS(ctx, domain)
	if err != nil {
		summary["dnsError"] = err.Error()
	} else {
		findings = append(findings, dnsFindings...)
	}

	return &Result{
		Tool:     t.Name(),
		Query:    map[string]interface{}{"domain": domain},
		Summary:  summary,
		Count:    len(findings),
		Findings: findings,
		Duration: time.Since(start),
	}, nil
}

type rdapResponse struct {
	LDHName string   `json:"ldhName"`
	Status  []string `json:"status"`
	Events  []struct {
		EventAction string `json:"eventAction"`
		EventDate   string `json:"eventDate"`
	} `json:"events"`
	Entities []struct {
		Roles      []string      `json:"roles"`
		VCardArray []interface{} `json:"vcardArray"`
	} `json:"entities"`
	Nameservers []struct {
		LDHName string `json:"ldhName"`
	} `json:"nameservers"`
}

func (t *DomainIntelTool) fetchRDAP(ctx context.Context, domain string) ([]Finding, error) {
	reqURL, err := url.JoinPath(t.RDAPBaseURL, domain)
	if err != nil {
		return nil, fmt.Errorf("build rdap url: %w", err)
	}
	if err := assertSameHost(t.RDAPBaseURL, reqURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build rdap request: %w", err)
	}
	req.Header.Set("Accept", "application/rdap+json")

	resp, err := t.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("rdap request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rdap returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read rdap response: %w", err)
	}

	var rdap rdapResponse
	if err := json.Unmarshal(body, &rdap); err != nil {
		return nil, fmt.Errorf("decode rdap response: %w", err)
	}

	findings := make([]Finding, 0)
	for _, event := range rdap.Events {
		findings = append(findings, Finding{
			Type:   "domain_event",
			Value:  event.EventAction,
			Source: "rdap",
			Details: map[string]interface{}{
				"eventAction": event.EventAction,
				"eventDate":   event.EventDate,
			},
		})
	}
	for _, ns := range rdap.Nameservers {
		findings = append(findings, Finding{
			Type:   "nameserver",
			Value:  ns.LDHName,
			Source: "rdap",
		})
	}
	for _, status := range rdap.Status {
		findings = append(findings, Finding{
			Type:   "domain_status",
			Value:  status,
			Source: "rdap",
		})
	}
	return findings, nil
}

func extractRegistrar(findings []Finding) string {
	for _, f := range findings {
		if f.Type == "domain_event" && f.Value == "registration" {
			return "registration observed"
		}
	}
	return ""
}

type dnsResponse struct {
	Status int `json:"Status"`
	Answer []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		Data string `json:"data"`
	} `json:"Answer"`
}

var dnsTypeNames = map[int]string{
	1: "A", 2: "NS", 5: "CNAME", 6: "SOA", 15: "MX", 16: "TXT", 28: "AAAA",
}

func (t *DomainIntelTool) fetchDNS(ctx context.Context, domain string) ([]Finding, error) {
	findings := make([]Finding, 0)
	for _, recordType := range []string{"A", "MX", "NS", "TXT"} {
		reqURL, err := queryURL(t.DNSURL, map[string]string{"name": domain, "type": recordType})
		if err != nil {
			return findings, fmt.Errorf("build dns url: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return findings, fmt.Errorf("build dns request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := t.httpClient().Do(req)
		if err != nil {
			return findings, fmt.Errorf("dns request: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			return findings, fmt.Errorf("read dns response: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			continue
		}

		var decoded dnsResponse
		if err := json.Unmarshal(body, &decoded); err != nil {
			continue
		}
		for _, answer := range decoded.Answer {
			name := dnsTypeNames[answer.Type]
			if name == "" {
				name = recordType
			}
			findings = append(findings, Finding{
				Type:   "dns_record",
				Value:  answer.Data,
				Source: "dns",
				Details: map[string]interface{}{
					"recordType": name,
					"name":       answer.Name,
				},
			})
		}
	}
	return findings, nil
}

func (t *DomainIntelTool) httpClient() *http.Client {
	if t.HTTPClient != nil {
		return t.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}
