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

type stubApp struct {
	authorized []string
	captured   []string
}

func (s *stubApp) InsertPending(context.Context, *Payment) error { return nil }
func (s *stubApp) FindByIdem(context.Context, string, string) (*Payment, error) {
	return nil, nil
}
func (s *stubApp) MarkFailed(context.Context, string) error { return nil }
func (s *stubApp) MarkAuthorized(_ context.Context, id string) error {
	s.authorized = append(s.authorized, id)
	return nil
}
func (s *stubApp) MarkCaptured(_ context.Context, id string) error {
	s.captured = append(s.captured, id)
	return nil
}
func (s *stubApp) EnqueueReconciliation(context.Context, string, string, string, error) error {
	return nil
}

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
		wantStatus      string
	}{
		{"success",
			&stubProcessor{},
			false, false, 0, StatusAuthorized},
		{"terminal failure no query",
			&stubProcessor{callErr: refused},
			true, false, 0, StatusPending},
		{"failed call then query accepts",
			&stubProcessor{callErr: transient, retryable: true, statusAccepted: true},
			false, false, 1, StatusCaptured},
		{"failed call then query absent",
			&stubProcessor{callErr: transient, retryable: true},
			true, false, 1, StatusPending},
		{"failed call then query fails",
			&stubProcessor{callErr: transient, retryable: true, statusErr: queryDown},
			true, true, 1, StatusPending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := &stubApp{}
			svc := &Service{processor: tc.stub, app: app}
			p := &Payment{ID: "p-1", Amount: 100, Status: StatusPending}
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
			if tc.wantErr {
				if len(app.authorized) != 0 || len(app.captured) != 0 {
					t.Fatalf("marked on failure: %v %v", app.authorized, app.captured)
				}
				return
			}
			if p.Status != tc.wantStatus {
				t.Fatalf("status=%q, want %q", p.Status, tc.wantStatus)
			}
			if tc.wantStatus == StatusAuthorized && (len(app.authorized) != 1 || app.authorized[0] != "p-1") {
				t.Fatalf("not marked authorized: %v", app.authorized)
			}
			if tc.wantStatus == StatusCaptured && (len(app.authorized) != 1 || len(app.captured) != 1) {
				t.Fatalf("not marked authorized+captured: %v %v", app.authorized, app.captured)
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
