package policy

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// MinimumAge is the minimum age in completed years required to register.
// A dating/meetup product must not onboard minors, so underage users are
// hard-blocked rather than routed to review.
const MinimumAge = 18

// MaximumPlausibleAge rejects obviously fabricated dates of birth (e.g. a
// typo that makes a subject 400 years old) so a bad date cannot be used to
// silently clear the gate.
const MaximumPlausibleAge = 130

// AgeStatus values reported by EvaluateAgeGate.
const (
	AgeStatusVerified = "verified"
	AgeStatusUnknown  = "unknown"
	AgeStatusInvalid  = "invalid"
	AgeStatusUnderage = "underage"
)

// Errors returned by CheckAgeGate, matching the roadmap's contract.
var (
	ErrUnderage   = errors.New("underage user detected")
	ErrInvalidDOB = errors.New("invalid date of birth")
)

// dateOnlyLayouts are the accepted wire formats for a date of birth, in
// preference order. The canonical form is ISO-8601 calendar date.
var dateOnlyLayouts = []string{
	"2006-01-02",
	time.RFC3339,
	"2006/01/02",
	"01/02/2006",
}

// AgeGateResult is the outcome of evaluating a date of birth against MinimumAge.
type AgeGateResult struct {
	Allowed bool
	Status  string
	Age     int // completed years, or -1 when no valid date was available
	DOB     *time.Time
	Message string
}

// ParseDateOfBirth parses a date of birth. An empty string yields (nil, nil) so
// callers can distinguish "not supplied" from "malformed".
func ParseDateOfBirth(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	for _, layout := range dateOnlyLayouts {
		if parsed, err := time.Parse(layout, s); err == nil {
			dob := dateOnly(parsed)
			return &dob, nil
		}
	}

	return nil, fmt.Errorf("unrecognised date of birth format %q (want YYYY-MM-DD)", s)
}

// dateOnly truncates a timestamp to its UTC calendar date. Dates of birth carry
// no meaningful time-of-day, and comparing them in local time would make the
// gate flip depending on server timezone.
func dateOnly(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// AgeAt returns the number of completed years between dob and now.
//
// A subject whose birthday has not yet occurred this year is one year younger,
// so the day before an 18th birthday still returns 17. A 29 February date of
// birth is treated as maturing on 1 March in non-leap years: that keeps the
// subject younger for one extra day, which is the safe direction for a gate
// whose purpose is to keep minors out.
func AgeAt(dob, now time.Time) int {
	dob = dateOnly(dob)
	now = dateOnly(now)

	years := now.Year() - dob.Year()
	if now.Month() < dob.Month() || (now.Month() == dob.Month() && now.Day() < dob.Day()) {
		years--
	}
	return years
}

// EvaluateAgeGate decides whether a subject clears the minimum-age gate.
//
// An absent date of birth does not block: age is simply unknown, and the
// caller may require ID verification (Plane B) to establish it. A date that is
// malformed at the parse layer never reaches here — ParseDateOfBirth rejects
// it — but a future date or an implausible age is treated as invalid rather
// than trusted.
func EvaluateAgeGate(dob *time.Time, now time.Time) AgeGateResult {
	if dob == nil {
		return AgeGateResult{
			Allowed: true,
			Status:  AgeStatusUnknown,
			Age:     -1,
			Message: "no date of birth supplied",
		}
	}

	d := dateOnly(*dob)
	if d.After(dateOnly(now)) {
		return AgeGateResult{
			Allowed: false,
			Status:  AgeStatusInvalid,
			Age:     -1,
			DOB:     &d,
			Message: "date of birth is in the future",
		}
	}

	age := AgeAt(d, now)
	if age > MaximumPlausibleAge {
		return AgeGateResult{
			Allowed: false,
			Status:  AgeStatusInvalid,
			Age:     age,
			DOB:     &d,
			Message: "date of birth is not plausible",
		}
	}

	if age < MinimumAge {
		return AgeGateResult{
			Allowed: false,
			Status:  AgeStatusUnderage,
			Age:     age,
			DOB:     &d,
			Message: fmt.Sprintf("subject is %d, minimum age is %d", age, MinimumAge),
		}
	}

	return AgeGateResult{
		Allowed: true,
		Status:  AgeStatusVerified,
		Age:     age,
		DOB:     &d,
		Message: fmt.Sprintf("subject is %d", age),
	}
}

// CheckAgeGate evaluates a known date of birth against MinimumAge. It returns
// ErrUnderage when the subject is a minor and ErrInvalidDOB when the date is
// not usable (future, implausible).
func (e *Engine) CheckAgeGate(dob time.Time) error {
	result := EvaluateAgeGate(&dob, time.Now())
	switch result.Status {
	case AgeStatusUnderage:
		return ErrUnderage
	case AgeStatusInvalid:
		return ErrInvalidDOB
	default:
		return nil
	}
}
