package api

import "testing"

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
