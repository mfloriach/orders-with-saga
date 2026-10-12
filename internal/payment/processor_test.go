package payment

import (
	"context"
	"errors"
	"testing"

	"paymentgateway/pkg/saga"
)

type stubProcessor struct {
	callErrs       []error
	calls          int
	statusAccepted bool
	statusErr      error
	statusCalls    int
	retryable      bool
}

func (s *stubProcessor) Call(context.Context, string, int64, string) error {
	s.calls++
	if s.calls <= len(s.callErrs) {
		return s.callErrs[s.calls-1]
	}
	return s.callErrs[len(s.callErrs)-1]
}

func (s *stubProcessor) Status(context.Context, string) (bool, error) {
	s.statusCalls++
	return s.statusAccepted, s.statusErr
}

func (s *stubProcessor) Retryable(error) bool { return s.retryable }

// TestCallProcessorOutcomes covers the four reconcile outcomes plus
// retry-then-success, all against a stubbed Processor.
func TestCallProcessorOutcomes(t *testing.T) {
	transient := errors.New("timeout")
	refused := errors.New("refused")
	queryDown := errors.New("status down")

	cases := []struct {
		name           string
		stub           *stubProcessor
		wantErr        bool
		wantInDoubt    bool
		wantCalls      int
		wantStatusCalls int
	}{
		{"success first try",
			&stubProcessor{callErrs: []error{nil}},
			false, false, 1, 0},
		{"terminal failure no retry no query",
			&stubProcessor{callErrs: []error{refused}, retryable: false},
			true, false, 1, 0},
		{"transient then success",
			&stubProcessor{callErrs: []error{transient, nil}, retryable: true},
			false, false, 2, 0},
		{"exhausted then query accepts",
			&stubProcessor{callErrs: []error{transient, transient, transient, transient}, retryable: true, statusAccepted: true},
			false, false, 3, 1},
		{"exhausted then query absent",
			&stubProcessor{callErrs: []error{transient, transient, transient}, retryable: true},
			true, false, 3, 1},
		{"exhausted then query fails",
			&stubProcessor{callErrs: []error{transient, transient, transient}, retryable: true, statusErr: queryDown},
			true, true, 3, 1},
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
			if tc.stub.calls != tc.wantCalls {
				t.Fatalf("calls: got %d, want %d", tc.stub.calls, tc.wantCalls)
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
