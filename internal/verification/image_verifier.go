package verification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// ErrSyntheticUnconfigured is returned when synthetic-image detection has no
// provider configured.
var ErrSyntheticUnconfigured = errors.New("synthetic image detection is not configured")

// ReverseImageMatch is one hit from a reverse-image search.
type ReverseImageMatch struct {
	URL        string  `json:"url"`
	Similarity float64 `json:"similarity"`
	Source     string  `json:"source,omitempty"`
}

// ReverseImageResult is the outcome of a reverse-image search.
type ReverseImageResult struct {
	Provider string              `json:"provider"`
	Matches  []ReverseImageMatch `json:"matches"`
}

// ReverseImageClient calls a reverse-image-search API.
type ReverseImageClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewReverseImageClient(baseURL, apiKey string) *ReverseImageClient {
	return &ReverseImageClient{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *ReverseImageClient) Configured() bool {
	return c != nil && c.APIKey != "" && c.BaseURL != ""
}

// Search uploads an image and returns the matches the provider found.
func (c *ReverseImageClient) Search(ctx context.Context, image []byte) (*ReverseImageResult, error) {
	if !c.Configured() {
		return nil, ErrVendorNotConfigured
	}
	if len(image) == 0 {
		return nil, errors.New("image is required")
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("image", "subject.jpg")
	if err != nil {
		return nil, fmt.Errorf("build reverse image form: %w", err)
	}
	if _, err := part.Write(image); err != nil {
		return nil, fmt.Errorf("write reverse image form: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close reverse image form: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("build reverse image request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("reverse image request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read reverse image response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("reverse image provider returned %d: %s", resp.StatusCode, string(body))
	}

	matches, err := decodeReverseImageMatches(body)
	if err != nil {
		return nil, err
	}

	return &ReverseImageResult{Provider: c.BaseURL, Matches: matches}, nil
}

// decodeReverseImageMatches accepts the two shapes seen in the wild: a bare
// JSON array of matches, or an object wrapping them under "matches"/"results".
func decodeReverseImageMatches(body []byte) ([]ReverseImageMatch, error) {
	var direct []ReverseImageMatch
	if err := json.Unmarshal(body, &direct); err == nil {
		return direct, nil
	}

	var wrapped struct {
		Matches []ReverseImageMatch `json:"matches"`
		Results []ReverseImageMatch `json:"results"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, fmt.Errorf("decode reverse image response: %w", err)
	}
	if wrapped.Matches != nil {
		return wrapped.Matches, nil
	}
	return wrapped.Results, nil
}

func (c *ReverseImageClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// SyntheticImageClient calls a synthetic/AI-generated image detector.
type SyntheticImageClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewSyntheticImageClient(baseURL, apiKey string) *SyntheticImageClient {
	return &SyntheticImageClient{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *SyntheticImageClient) Configured() bool {
	return c != nil && c.APIKey != "" && c.BaseURL != ""
}

// SyntheticImageResult carries the detector's confidence that an image is
// AI-generated. A higher score means more likely synthetic.
type SyntheticImageResult struct {
	IsSynthetic bool    `json:"isSynthetic"`
	Score       float64 `json:"score"`
	Provider    string  `json:"provider"`
}

// DefaultSyntheticThreshold: at or above this score the image is treated as
// synthetic.
const DefaultSyntheticThreshold = 0.75

// Detect submits an image for synthetic-content scoring.
func (c *SyntheticImageClient) Detect(ctx context.Context, image []byte) (*SyntheticImageResult, error) {
	if !c.Configured() {
		return nil, ErrSyntheticUnconfigured
	}
	if len(image) == 0 {
		return nil, errors.New("image is required")
	}

	payload, err := json.Marshal(map[string]string{"image": encodeBase64(image)})
	if err != nil {
		return nil, fmt.Errorf("marshal synthetic payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/detect", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build synthetic request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("synthetic provider request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read synthetic response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("synthetic provider returned %d: %s", resp.StatusCode, string(body))
	}

	var decoded struct {
		Score       float64 `json:"score"`
		IsSynthetic *bool   `json:"is_synthetic"`
		Provider    string  `json:"provider"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode synthetic response: %w", err)
	}

	result := &SyntheticImageResult{Score: decoded.Score, Provider: decoded.Provider}
	if decoded.IsSynthetic != nil {
		result.IsSynthetic = *decoded.IsSynthetic
	} else {
		result.IsSynthetic = decoded.Score >= DefaultSyntheticThreshold
	}
	return result, nil
}

func (c *SyntheticImageClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}
