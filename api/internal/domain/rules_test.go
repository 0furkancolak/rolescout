package domain

import (
	"errors"
	"testing"
)

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		from, to ApplicationStatus
		want     error
	}{
		{StatusPlanned, StatusApplied, nil},
		{StatusPlanned, StatusPlanned, nil},
		{StatusApplied, StatusInterviewing, nil},
		{StatusOffer, StatusAccepted, nil},
		{StatusRejected, StatusApplied, nil},
		{StatusPlanned, StatusOffer, ErrConflict},
		{StatusAccepted, StatusRejected, ErrConflict},
		{StatusInterviewing, StatusPlanned, ErrConflict},
		{StatusApplied, "ghosted", ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			err := ValidateTransition(tt.from, tt.to)
			if tt.want == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestEveryStatusHasTransitions(t *testing.T) {
	for _, s := range ApplicationStatuses {
		if len(statusTransitions[s]) == 0 {
			t.Errorf("status %s has no outgoing transitions", s)
		}
	}
}

func TestStatusAfterAttempt(t *testing.T) {
	for _, s := range []ApplicationStatus{StatusPlanned, StatusRejected, StatusWithdrawn} {
		if next, changed := StatusAfterAttempt(s); !changed || next != StatusApplied {
			t.Errorf("%s -> %s, %v; want applied, true", s, next, changed)
		}
	}
	if next, changed := StatusAfterAttempt(StatusInterviewing); changed || next != StatusInterviewing {
		t.Errorf("interviewing changed to %s", next)
	}
}

func TestValidationErrorIsInvalid(t *testing.T) {
	var v ValidationError
	if v.Err() != nil {
		t.Fatal("empty validation error must be nil")
	}
	err := v.Add("name", "is required").Err()
	if !errors.Is(err, ErrInvalid) {
		t.Fatal("validation error must match ErrInvalid")
	}
}
