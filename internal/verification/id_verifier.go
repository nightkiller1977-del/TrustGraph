package verification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrVendorNotConfigured is returned when a vendor-backed check has no API key.
// Handlers translate it into a 503 rather than pretending the check passed.
var ErrVendorNotConfigured = errors.New("verification vendor is not configured")

// IDVerificationRequest is the input to a government-ID check. The document
// image is sent to the vendor and never persisted by TrustGraph.
type IDVerificationRequest struct {
	DocumentImage []byte
	SelfieImage   []byte
	DocumentType  string
	CountryCode   string
}

// IDVerificationResult is the vendor's verdict. DateOfBirth is the field the
// age gate consumes; it comes from the document, not from the subject's claim.
type IDVerificationResult struct {
	Status          string
	VerifiedName    string
	DateOfBirth     *time.Time
	AddressLine     string
	CountryCode     string
	VendorReference string
	FailureReason   string
	CostUSD         float64
}

// IDVerifierClient calls a Persona/Onfido-style REST identity vendor.
type IDVerifierClient struct {
	BaseURL    string
	APIKey     string
	VendorName string
	HTTPClient *http.Client
}

// NewIDVerifierClient builds an ID verifier. An empty APIKey keeps the client
// in the not-configured state.
func NewIDVerifierClient(baseURL, apiKey, vendorName string) *IDVerifierClient {
	return &IDVerifierClient{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		VendorName: vendorName,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *IDVerifierClient) Configured() bool {
	return c != nil && c.APIKey != "" && c.BaseURL != ""
}

// vendorIDPayload mirrors the subset of the vendor's create-inquiry request we
// send. It is intentionally minimal so it can be adapted to Persona or Onfido
// with a field rename.
type vendorIDPayload struct {
	DocumentType  string `json:"document_type"`
	CountryCode   string `json:"country_code"`
	DocumentImage string `json:"document_image"`
	SelfieImage   string `json:"selfie_image,omitempty"`
}

type vendorIDResponse struct {
	Status        string `json:"status"`
	ReferenceID   string `json:"reference_id"`
	Name          string `json:"name"`
	DateOfBirth   string `json:"date_of_birth"`
	AddressLine   string `json:"address_line"`
	CountryCode   string `json:"country_code"`
	FailureReason string `json:"failure_reason"`
}

// Verify submits a document for verification. It returns ErrVendorNotConfigured
// when the client has no credentials, so the caller never treats a skipped
// check as a pass.
func (c *IDVerifierClient) Verify(ctx context.Context, req IDVerificationRequest) (*IDVerificationResult, error) {
	if !c.Configured() {
		return nil, ErrVendorNotConfigured
	}
	if len(req.DocumentImage) == 0 {
		return nil, errors.New("document image is required")
	}

	payload := vendorIDPayload{
		DocumentType:  req.DocumentType,
		CountryCode:   req.CountryCode,
		DocumentImage: encodeBase64(req.DocumentImage),
	}
	if len(req.SelfieImage) > 0 {
		payload.SelfieImage = encodeBase64(req.SelfieImage)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal id payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/inquiries", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build id request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("id vendor request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read id vendor response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("id vendor returned %d: %s", resp.StatusCode, string(respBody))
	}

	var decoded vendorIDResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode id vendor response: %w", err)
	}

	result := &IDVerificationResult{
		Status:          mapVendorStatus(decoded.Status),
		VerifiedName:    decoded.Name,
		AddressLine:     decoded.AddressLine,
		CountryCode:     decoded.CountryCode,
		VendorReference: decoded.ReferenceID,
		FailureReason:   decoded.FailureReason,
	}
	if decoded.DateOfBirth != "" {
		if dob, err := time.Parse("2006-01-02", decoded.DateOfBirth); err == nil {
			result.DateOfBirth = &dob
		} else {
			result.FailureReason = fmt.Sprintf("vendor returned an unparseable date of birth: %q", decoded.DateOfBirth)
			result.Status = "failed"
		}
	}
	return result, nil
}

// mapVendorStatus normalises vendor-specific status strings.
func mapVendorStatus(raw string) string {
	switch raw {
	case "completed", "verified", "approved", "passed":
		return "verified"
	case "failed", "declined", "rejected":
		return "failed"
	case "pending", "created", "processing", "in_progress":
		return "pending"
	default:
		return raw
	}
}

func (c *IDVerifierClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}
