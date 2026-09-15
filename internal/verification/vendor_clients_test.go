package verification

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

func TestIDVerifierClient_NotConfigured(t *testing.T) {
	client := NewIDVerifierClient("https://example.test", "", "persona")
	_, err := client.Verify(context.Background(), IDVerificationRequest{DocumentImage: []byte("img")})
	assert.ErrorIs(t, err, ErrVendorNotConfigured)
}

func TestIDVerifierClient_VerifySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/inquiries", r.URL.Path)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "completed",
			"reference_id":  "inq_123",
			"name":          "Jane Doe",
			"date_of_birth": "1990-05-17",
			"country_code":  "US",
		})
	}))
	defer server.Close()

	client := NewIDVerifierClient(server.URL, "secret", "persona")
	result, err := client.Verify(context.Background(), IDVerificationRequest{
		DocumentImage: []byte("document-bytes"),
		DocumentType:  "passport",
		CountryCode:   "US",
	})

	require.NoError(t, err)
	assert.Equal(t, "verified", result.Status)
	assert.Equal(t, "Jane Doe", result.VerifiedName)
	require.NotNil(t, result.DateOfBirth)
	assert.Equal(t, "1990-05-17", result.DateOfBirth.Format("2006-01-02"))
}

func TestIDVerifierClient_UnparseableDOBFailsRatherThanGuessing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "completed",
			"date_of_birth": "17/05/1990",
		})
	}))
	defer server.Close()

	client := NewIDVerifierClient(server.URL, "secret", "persona")
	result, err := client.Verify(context.Background(), IDVerificationRequest{DocumentImage: []byte("x")})

	require.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
	assert.Nil(t, result.DateOfBirth)
}

func TestIDVerifierClient_VendorErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer server.Close()

	client := NewIDVerifierClient(server.URL, "secret", "persona")
	_, err := client.Verify(context.Background(), IDVerificationRequest{DocumentImage: []byte("x")})
	assert.Error(t, err)
}

func TestLivenessVerifierClient_ScoreDrivesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":         "verified",
			"liveness_score": 0.42,
			"reference_id":   "live_1",
		})
	}))
	defer server.Close()

	client := NewLivenessVerifierClient(server.URL, "secret")
	result, err := client.Verify(context.Background(), LivenessRequest{VideoFrame: []byte("frame")})

	require.NoError(t, err)
	// The vendor said "verified" but the score is below threshold, so the score wins.
	assert.Equal(t, "failed", result.Status)
	assert.NotEmpty(t, result.FailureReason)
}

func TestLivenessVerifierClient_PassesAboveThreshold(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"liveness_score":   0.95,
			"matched_identity": true,
		})
	}))
	defer server.Close()

	client := NewLivenessVerifierClient(server.URL, "secret")
	result, err := client.Verify(context.Background(), LivenessRequest{VideoFrame: []byte("frame")})

	require.NoError(t, err)
	assert.Equal(t, "verified", result.Status)
	require.NotNil(t, result.MatchedIdentity)
	assert.True(t, *result.MatchedIdentity)
}

func TestReverseImageClient_ParsesWrappedAndBareResponses(t *testing.T) {
	t.Run("wrapped", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"matches": []map[string]interface{}{
					{"url": "https://a.example/img.jpg", "similarity": 0.9},
				},
			})
		}))
		defer server.Close()

		client := NewReverseImageClient(server.URL, "key")
		result, err := client.Search(context.Background(), []byte("image"))
		require.NoError(t, err)
		assert.Len(t, result.Matches, 1)
		assert.Equal(t, 0.9, result.Matches[0].Similarity)
	})

	t.Run("bare array", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"url": "https://b.example/img.jpg", "similarity": 0.5},
			})
		}))
		defer server.Close()

		client := NewReverseImageClient(server.URL, "key")
		result, err := client.Search(context.Background(), []byte("image"))
		require.NoError(t, err)
		assert.Len(t, result.Matches, 1)
	})
}

func TestSyntheticImageClient_ThresholdApplied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"score": 0.88})
	}))
	defer server.Close()

	client := NewSyntheticImageClient(server.URL, "key")
	result, err := client.Detect(context.Background(), []byte("image"))

	require.NoError(t, err)
	assert.True(t, result.IsSynthetic)
	assert.WithinDuration(t, time.Now(), time.Now(), time.Second)
}

func TestSyntheticImageClient_NotConfigured(t *testing.T) {
	client := NewSyntheticImageClient("", "")
	_, err := client.Detect(context.Background(), []byte("image"))
	assert.ErrorIs(t, err, ErrSyntheticUnconfigured)
}
