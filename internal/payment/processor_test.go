package payment

import (
	"context"
	"errors"
	"testing"

	"paymentgateway/pkg/saga"
)

type stubProcessor struct {
	callErr        error
	calls          int
	statusAccepted bool
	statusErr      error
	statusCalls    int
	retryable      bool
}

func (s *stubProcessor) Call(context.Context, string, int64, string) error {
	s.calls++
	return s.callErr
}

func (s *stubProcessor) Status(context.Context, string) (bool, error) {
	s.statusCalls++
	return s.statusAccepted, s.statusErr
}

func (s *stubProcessor) Retryable(error) bool { return s.retryable }

// TestCallProcessorOutcomes covers single-call success, terminal failure,
// query-accepted, query-absent, and query-failed outcomes. Retry counting
// lives in the client and repo httptest suites; the stub answers once.
func TestCallProcessorOutcomes(t *testing.T) {
	transient := errors.New("timeout")
	refused := errors.New("refused")
	queryDown := errors.New("status down")

	cases := []struct {
		name            string
		stub            *stubProcessor
		wantErr         bool
		wantInDoubt     bool
		wantStatusCalls int
	}{
		{"success",
			&stubProcessor{},
			false, false, 0},
		{"terminal failure no query",
			&stubProcessor{callErr: refused},
			true, false, 0},
		{"failed call then query accepts",
			&stubProcessor{callErr: transient, retryable: true, statusAccepted: true},
			false, false, 1},
		{"failed call then query absent",
			&stubProcessor{callErr: transient, retryable: true},
			true, false, 1},
		{"failed call then query fails",
			&stubProcessor{callErr: transient, retryable: true, statusErr: queryDown},
			true, true, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &Service{processor: tc.stub}
			p := &Payment{ID: "p-1", Amount: 100}
			err := svc.callProcessor(context.Background(), p, "k-1")
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("got %v", err)
			}
			var indoubt *saga.InDoubtError
			if tc.wantInDoubt != errors.As(err, &indoubt) {
				t.Fatalf("in-doubt=%v, err=%v", tc.wantInDoubt, err)
			}
			if tc.stub.calls != 1 {
				t.Fatalf("calls: got %d, want 1", tc.stub.calls)
			}
			if tc.stub.statusCalls != tc.wantStatusCalls {
				t.Fatalf("status calls: got %d, want %d", tc.stub.statusCalls, tc.wantStatusCalls)
			}
		})
	}
}

// TestCallProcessorNilFailsClosed verifies no silent no-op without a client.
func TestCallProcessorNilFailsClosed(t *testing.T) {
	svc := &Service{}
	if err := svc.callProcessor(context.Background(), &Payment{}, "k"); err == nil {
		t.Fatal("expected error")
	}
}
