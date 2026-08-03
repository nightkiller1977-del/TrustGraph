package policy

import "time"

// CurrentPolicyVersion is the active policy version applied to all assessments.
const CurrentPolicyVersion = "registration-v1"

// PolicyVersion describes a policy revision with its effective date.
type PolicyVersion struct {
	Version     string
	Description string
	EffectiveAt time.Time
}

// KnownVersions enumerates every policy version the engine has shipped.
// The slice is ordered newest-first; the head element is always the active
// version referenced by CurrentPolicyVersion.
var KnownVersions = []PolicyVersion{
	{
		Version:     "registration-v1",
		Description: "Initial registration-time trust assessment policy",
		EffectiveAt: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC),
	},
}
