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

// LivenessRequest is a proof-of-person check: a short video/selfie plus the
// government-ID reference it should match.
type LivenessRequest struct {
	VideoFrame            []byte
	GovernmentIDReference string
}

// LivenessResult is the vendor's verdict.
type LivenessResult struct {
	Status          string
	LivenessScore   float64
	MatchedIdentity *bool
	VendorReference string
	FailureReason   string
	CostUSD         float64
}

// LivenessVerifierClient calls a liveness vendor's REST API.
type LivenessVerifierClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewLivenessVerifierClient(baseURL, apiKey string) *LivenessVerifierClient {
	return &LivenessVerifierClient{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *LivenessVerifierClient) Configured() bool {
	return c != nil && c.APIKey != "" && c.BaseURL != ""
}

type vendorLivenessPayload struct {
	Frame           string `json:"frame"`
	GovernmentIDRef string `json:"government_id_reference,omitempty"`
}

type vendorLivenessResponse struct {
	Status        string  `json:"status"`
	ReferenceID   string  `json:"reference_id"`
	LivenessScore float64 `json:"liveness_score"`
	Matched       *bool   `json:"matched_identity"`
	FailureReason string  `json:"failure_reason"`
}

// DefaultLivenessThreshold is the score a result must reach to count as live.
const DefaultLivenessThreshold = 0.80

// Verify submits a liveness check and applies the score threshold.
func (c *LivenessVerifierClient) Verify(ctx context.Context, req LivenessRequest) (*LivenessResult, error) {
	if !c.Configured() {
		return nil, ErrVendorNotConfigured
	}
	if len(req.VideoFrame) == 0 {
		return nil, errors.New("liveness frame is required")
	}

	payload := vendorLivenessPayload{
		Frame:           encodeBase64(req.VideoFrame),
		GovernmentIDRef: req.GovernmentIDReference,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal liveness payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/liveness", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build liveness request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("liveness vendor request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read liveness response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("liveness vendor returned %d: %s", resp.StatusCode, string(respBody))
	}

	var decoded vendorLivenessResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode liveness response: %w", err)
	}

	result := &LivenessResult{
		Status:          decoded.Status,
		LivenessScore:   decoded.LivenessScore,
		MatchedIdentity: decoded.Matched,
		VendorReference: decoded.ReferenceID,
		FailureReason:   decoded.FailureReason,
	}

	// A vendor can return status "verified" with a low score, or omit the
	// status entirely. The score is the authoritative signal, so derive the
	// final status from it rather than trusting the label.
	if result.Status == "" || result.Status == "verified" {
		if decoded.LivenessScore >= DefaultLivenessThreshold {
			result.Status = "verified"
		} else {
			result.Status = "failed"
			if result.FailureReason == "" {
				result.FailureReason = fmt.Sprintf("liveness score %.2f below threshold %.2f", decoded.LivenessScore, DefaultLivenessThreshold)
			}
		}
	}

	return result, nil
}

func (c *LivenessVerifierClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 60 * time.Second}
}
