package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecodeUploadBody_RejectsOversizedPayload guards the memory-exhaustion
// fix: the upload handlers must reject a body above the cap before decoding it
// rather than buffering it whole (and then base64-decoding it, doubling the
// footprint).
func TestDecodeUploadBody_RejectsOversizedPayload(t *testing.T) {
	// A valid JSON body whose string value exceeds the cap. The size check must
	// fire on read, before the whole payload is buffered.
	oversized := []byte(`{"frame":"` + strings.Repeat("a", maxUploadPayloadBytes) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(oversized))
	rec := httptest.NewRecorder()

	var dst map[string]interface{}
	assert.False(t, decodeUploadBody(rec, req, &dst), "an oversized payload must be rejected")
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Contains(t, rec.Body.String(), "payload_too_large")
}

func TestDecodeUploadBody_AcceptsPayloadWithinLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"frame":"abc"}`)))
	rec := httptest.NewRecorder()

	var dst map[string]interface{}
	require.True(t, decodeUploadBody(rec, req, &dst), "a small payload must decode")
	assert.Equal(t, "abc", dst["frame"])
}

func TestDecodeUploadBody_RejectsMalformedJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"frame":`)))
	rec := httptest.NewRecorder()

	var dst map[string]interface{}
	assert.False(t, decodeUploadBody(rec, req, &dst))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestIsKnownConsentType guards the allow-list that both the grant and withdraw
// endpoints validate against. An unknown purpose reaching the store would be
// rejected there, but the API should reject it first with a 400 rather than a
// 500, and a purpose missing from this list is unusable even though the store
// supports it.
func TestIsKnownConsentType(t *testing.T) {
	known := []string{
		"linkedin_oauth",
		"government_id",
		"liveness",
		"image_verification",
	}
	for _, c := range known {
		if !isKnownConsentType(c) {
			t.Errorf("isKnownConsentType(%q) = false, want true", c)
		}
	}

	unknown := []string{
		"",
		"linkedin", // the old value; must not be accepted
		"education",
		"image",
		"Likeness",
		"government_id ",
	}
	for _, c := range unknown {
		if isKnownConsentType(c) {
			t.Errorf("isKnownConsentType(%q) = true, want false", c)
		}
	}
}
