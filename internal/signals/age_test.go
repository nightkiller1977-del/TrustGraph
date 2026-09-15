package signals

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
)

func TestAgeGateProvider_Name(t *testing.T) {
	assert.Equal(t, "age_gate", NewAgeGateProvider().Name())
}

func TestAgeGateProvider_Evaluate(t *testing.T) {
	dob := func(s string) *time.Time {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatalf("bad test date %q: %v", s, err)
		}
		return &d
	}

	provider := NewAgeGateProvider()

	tests := []struct {
		name        string
		dob         *time.Time
		wantScore   int
		wantConf    float64
		wantReasons []string
	}{
		{
			name:        "underage scores maximum risk and blocks",
			dob:         dob("2015-01-01"),
			wantScore:   100,
			wantConf:    1.0,
			wantReasons: []string{models.ReasonCodeUnderageUser},
		},
		{
			name:        "implausible date is flagged but below the block threshold",
			dob:         dob("1850-01-01"),
			wantScore:   60,
			wantConf:    1.0,
			wantReasons: []string{models.ReasonCodeAgeInvalid},
		},
		{
			name:        "missing dob contributes no risk",
			dob:         nil,
			wantScore:   0,
			wantConf:    0.0,
			wantReasons: []string{models.ReasonCodeAgeUnknown},
		},
		{
			name:        "adult contributes no risk",
			dob:         dob("1990-01-01"),
			wantScore:   0,
			wantConf:    0.5,
			wantReasons: []string{models.ReasonCodeAgeVerified},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.Evaluate(context.Background(), &EvalContext{DateOfBirth: tt.dob}, nil)
			assert.Equal(t, "age_gate", result.Provider)
			assert.Equal(t, tt.wantScore, result.Score)
			assert.Equal(t, tt.wantConf, result.Confidence)
			assert.Equal(t, tt.wantReasons, result.ReasonCodes)
			assert.Nil(t, result.Error)
		})
	}
}
