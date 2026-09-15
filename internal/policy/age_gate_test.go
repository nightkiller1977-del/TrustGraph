package policy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", s)
	require.NoError(t, err)
	return parsed
}

func TestParseDateOfBirth(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		wantErr bool
		wantDay string
	}{
		{name: "empty is treated as not supplied", input: "", wantNil: true},
		{name: "whitespace is treated as not supplied", input: "   ", wantNil: true},
		{name: "iso calendar date", input: "1990-05-17", wantDay: "1990-05-17"},
		{name: "rfc3339 timestamp", input: "1990-05-17T00:00:00Z", wantDay: "1990-05-17"},
		{name: "rejects nonsense", input: "not-a-date", wantErr: true},
		{name: "rejects impossible day", input: "1990-02-31", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateOfBirth(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.wantDay, got.Format("2006-01-02"))
		})
	}
}

func TestAgeAt(t *testing.T) {
	now := mustDate(t, "2026-09-14")

	tests := []struct {
		name string
		dob  string
		want int
	}{
		{
			// Day before the 18th birthday: must still be 17, otherwise the gate
			// would admit a minor on a technicality.
			name: "day before eighteenth birthday is 17",
			dob:  "2008-09-15",
			want: 17,
		},
		{name: "on eighteenth birthday is 18", dob: "2008-09-14", want: 18},
		{name: "day after eighteenth birthday is 18", dob: "2008-09-13", want: 18},
		{name: "birthday later this year not yet reached", dob: "2000-12-25", want: 25},
		{name: "birthday already passed this year", dob: "2000-01-02", want: 26},
		{name: "leap day matures on march 1 in non-leap year", dob: "2008-02-29", want: 18},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AgeAt(mustDate(t, tt.dob), now))
		})
	}
}

func TestEvaluateAgeGate(t *testing.T) {
	now := mustDate(t, "2026-09-14")

	tests := []struct {
		name       string
		dob        *time.Time
		wantStatus string
		wantAllow  bool
		wantAge    int
	}{
		{name: "no dob is unknown but not blocked", dob: nil, wantStatus: AgeStatusUnknown, wantAllow: true, wantAge: -1},
		{name: "adult clears the gate", dob: ptrDate(t, "1990-01-01"), wantStatus: AgeStatusVerified, wantAllow: true, wantAge: 36},
		{name: "minor is blocked", dob: ptrDate(t, "2010-01-01"), wantStatus: AgeStatusUnderage, wantAllow: false, wantAge: 16},
		{name: "exactly 18 clears", dob: ptrDate(t, "2008-09-14"), wantStatus: AgeStatusVerified, wantAllow: true, wantAge: 18},
		{name: "future dob is invalid", dob: ptrDate(t, "2030-01-01"), wantStatus: AgeStatusInvalid, wantAllow: false, wantAge: -1},
		{
			// Guards against a typo'd century being read as "obviously an adult".
			name: "implausibly old dob is invalid", dob: ptrDate(t, "1850-01-01"),
			wantStatus: AgeStatusInvalid, wantAllow: false, wantAge: 176,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EvaluateAgeGate(tt.dob, now)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, tt.wantAllow, result.Allowed)
			assert.Equal(t, tt.wantAge, result.Age)
		})
	}
}

func TestEngineCheckAgeGate(t *testing.T) {
	engine := NewEngine(nil)

	t.Run("adult passes", func(t *testing.T) {
		assert.NoError(t, engine.CheckAgeGate(mustDate(t, "1990-01-01")))
	})

	t.Run("minor returns ErrUnderage", func(t *testing.T) {
		assert.ErrorIs(t, engine.CheckAgeGate(mustDate(t, "2015-01-01")), ErrUnderage)
	})

	t.Run("future date returns ErrInvalidDOB", func(t *testing.T) {
		assert.ErrorIs(t, engine.CheckAgeGate(time.Now().AddDate(1, 0, 0)), ErrInvalidDOB)
	})
}

func ptrDate(t *testing.T, s string) *time.Time {
	t.Helper()
	d := mustDate(t, s)
	return &d
}
