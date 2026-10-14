package payment

import "testing"

// TestCheckTransition covers every legal edge plus illegal and unknown
// states. The table is the spec; keep it in sync with the migration.
func TestCheckTransition(t *testing.T) {
	legal := [][2]Status{
		{StatusPending, StatusAuthorized},
		{StatusPending, StatusFailed},
		{StatusAuthorized, StatusFailed},
		{StatusAuthorized, StatusCaptured},
	}
	for _, tc := range legal {
		if err := (&Payment{Status: tc[0]}).TransitionTo(tc[1]); err != nil {
			t.Errorf("%s -> %s: got %v", tc[0], tc[1], err)
		}
	}

	illegal := [][2]Status{
		{StatusPending, StatusPending},
		{StatusPending, StatusCaptured},
		{StatusFailed, StatusAuthorized},
		{StatusFailed, StatusFailed},
		{StatusFailed, StatusPending},
		{StatusAuthorized, StatusPending},
		{StatusAuthorized, StatusAuthorized},
		{StatusCaptured, StatusPending},
		{StatusCaptured, StatusAuthorized},
		{StatusCaptured, StatusCaptured},
		{StatusCaptured, StatusFailed},
		{Status("bogus"), StatusFailed},
		{StatusPending, Status("bogus")},
		{Status(""), Status("")},
	}
	for _, tc := range illegal {
		if err := (&Payment{Status: tc[0]}).TransitionTo(tc[1]); err == nil {
			t.Errorf("%s -> %s: expected error", tc[0], tc[1])
		}
	}
}
