package saga

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func ok() func(context.Context) error { return func(context.Context) error { return nil } }
func fail(err error) func(context.Context) error {
	return func(context.Context) error { return err }
}

// rec is a mutex-guarded append recorder for background unwind assertions.
type rec struct {
	mu    sync.Mutex
	names []string
}

func (r *rec) add(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = append(r.names, name)
}

func (r *rec) get() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.names...)
}

// syncBuf is a goroutine-safe log sink for background unwind assertions.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// waitFor polls cond until true or the timeout, failing the test.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// verifies a clean run returns nil and skips compensation.
func TestSuccessRunsNoCompensation(t *testing.T) {
	compensated := false
	s := Saga{Name: "t", Steps: []Step{
		{Name: "a", Do: ok(), Compensate: func(context.Context) error { compensated = true; return nil }},
		{Name: "b", Do: ok()},
	}}
	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("got %v", err)
	}
	if compensated {
		t.Fatal("compensation ran on success")
	}
}

// verifies a mid-chain failure skips later steps and compensates in reverse
// in the background with resume data returned immediately.
func TestFailureStopsForwardAndUnwindsReverse(t *testing.T) {
	var did []string
	undid := &rec{}
	boom := errors.New("boom")
	do := func(name string, err error) func(context.Context) error {
		return func(context.Context) error { did = append(did, name); return err }
	}
	s := Saga{ID: "s-1", Name: "t", Steps: []Step{
		{Name: "a", Do: do("a", nil), Compensate: func(context.Context) error { undid.add("a"); return nil }},
		{Name: "b", Do: do("b", nil), Compensate: func(context.Context) error { undid.add("b"); return nil }},
		{Name: "c", Do: do("c", boom)},
		{Name: "d", Do: do("d", nil)},
	}}
	err := s.Run(context.Background())
	var re *RunError
	if !errors.As(err, &re) {
		t.Fatalf("expected *RunError, got %T", err)
	}
	if re.FailedStep != "c" || !errors.Is(re.StepErr, boom) {
		t.Fatalf("bad failure identity: %+v", re)
	}
	if len(did) != 3 || did[0] != "a" || did[1] != "b" || did[2] != "c" {
		t.Fatalf("step d executed or order wrong: %v", did)
	}
	if re.SagaID != "s-1" || len(re.Completed) != 2 || re.Completed[0] != "a" {
		t.Fatalf("bad resume data: %+v", re)
	}
	waitFor(t, "background unwind", func() bool { return len(undid.get()) == 2 })
	if got := undid.get(); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("bad unwind order: %v", got)
	}
}

// verifies nil compensations are skipped and the failed step itself is never compensated.
func TestNilCompensateSkippedAndFailedStepNotCompensated(t *testing.T) {
	ran := &rec{}
	s := Saga{Name: "t", Steps: []Step{
		{Name: "a", Do: ok()}, // nil compensate
		{Name: "b", Do: fail(errors.New("x")), Compensate: func(context.Context) error {
			ran.add("b")
			return nil
		}},
	}}
	if err := s.Run(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	// Give the background unwind a chance to wrongly run, then assert silence.
	time.Sleep(100 * time.Millisecond)
	if got := ran.get(); len(got) != 0 {
		t.Fatalf("failed step compensated: %v", got)
	}
}

// verifies a failing compensation is logged without aborting the remaining unwind.
func TestCompensateErrorsLoggedAndUnwindContinues(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	ran := &rec{}
	ce := errors.New("undo-bad")
	s := Saga{Name: "t", Steps: []Step{
		{Name: "a", Do: ok(), Compensate: func(context.Context) error { ran.add("a"); return nil }},
		{Name: "b", Do: ok(), Compensate: func(context.Context) error { ran.add("b"); return ce }},
		{Name: "c", Do: fail(errors.New("main"))},
	}, Logger: log}
	err := s.Run(context.Background())
	var re *RunError
	if !errors.As(err, &re) {
		t.Fatalf("expected *RunError, got %T", err)
	}
	waitFor(t, "background unwind", func() bool { return len(ran.get()) == 2 })
	if got := ran.get(); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("unwind did not continue: %v", got)
	}
	out := buf.String()
	for _, want := range []string{`"level":"WARN"`, `"step":"b"`, "undo-bad"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in logs: %s", want, out)
		}
	}
}

// verifies a cancelled context stops the saga before any step runs.
func TestCancelledCtxAbortsForward(t *testing.T) {
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	compensated := false
	s := Saga{Name: "t", Steps: []Step{
		{Name: "a", Do: ok(), Compensate: func(context.Context) error { compensated = true; return nil }},
		{Name: "b", Do: ok()},
	}}
	err := s.Run(cancelled)
	var re *RunError
	if !errors.As(err, &re) || re.FailedStep != "a" {
		t.Fatalf("expected abort at a, got %v", err)
	}
	if compensated {
		t.Fatal("nothing completed, nothing to compensate")
	}
}

// verifies completed steps are still compensated when a later step fails.
func TestCancelledCtxStillUnwindsCompleted(t *testing.T) {
	base := context.Background()
	done := make(chan struct{})
	s := Saga{Name: "t", Steps: []Step{
		{Name: "a", Do: ok(), Compensate: func(context.Context) error { close(done); return nil }},
		{Name: "b", Do: func(ctx context.Context) error {
			// Cancel is not visible here; simulate client disconnect mid-run
			// by returning an error after "a" completed.
			return errors.New("downstream down")
		}},
	}}
	if err := s.Run(base); err == nil {
		t.Fatal("expected error")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("completed step was not compensated in background")
	}
}

// verifies an empty saga ID defaults to a generated uuid.
func TestDefaultSagaIDGenerated(t *testing.T) {
	s := Saga{Name: "t", Steps: []Step{{Name: "a", Do: fail(errors.New("x"))}}}
	var re *RunError
	if !errors.As(s.Run(context.Background()), &re) {
		t.Fatal("expected *RunError")
	}
	if re.SagaID == "" {
		t.Fatal("empty saga ID")
	}
}

// verifies per-step spans exist, unwind is re-rooted in the background,
// and failure logs carry saga identity.
func TestObservabilitySpansAndLogs(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(old)

	var buf syncBuf
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	boom := errors.New("kaboom")
	s := Saga{Name: "pay", Steps: []Step{
		{Name: "a", Do: ok(), Compensate: ok()},
		{Name: "b", Do: fail(boom), Compensate: ok()},
	}, Logger: log}
	if err := s.Run(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	waitFor(t, "background unwind span", func() bool {
		for _, sp := range sr.Ended() {
			if sp.Name() == "saga.compensate.a" {
				return true
			}
		}
		return false
	})
	names := map[string]bool{}
	for _, sp := range sr.Ended() {
		names[sp.Name()] = true
	}
	for _, want := range []string{"saga.pay", "saga.step.a", "saga.step.b", "saga.pay.unwind", "saga.compensate.a"} {
		if !names[want] {
			t.Fatalf("missing span %q in %v", want, names)
		}
	}
	out := buf.String()
	for _, want := range []string{`"saga_name":"pay"`, `"step":"b"`, "kaboom"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in logs: %s", want, out)
		}
	}
}

// verifies step failure logs ERROR and background compensate failure logs
// WARN with saga identity.
func TestFailureLogsCarrySagaIdentity(t *testing.T) {
	var buf syncBuf
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	boom := errors.New("boom")
	undoBad := errors.New("undo-bad")
	s := Saga{ID: "s-log", Name: "pay", Steps: []Step{
		{Name: "a", Do: ok(), Compensate: fail(undoBad)},
		{Name: "b", Do: fail(boom)},
	}, Logger: log}
	if err := s.Run(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	waitFor(t, "background compensate log", func() bool {
		return strings.Contains(buf.String(), `"level":"WARN"`)
	})
	out := buf.String()
	for _, want := range []string{
		`"level":"ERROR"`, `"saga_id":"s-log"`, `"saga_name":"pay"`, `"step":"b"`, "boom",
		`"level":"WARN"`, `"step":"a"`, "undo-bad",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in logs: %s", want, out)
		}
	}
}

// verifies a successful run emits no error or warning logs.
func TestSuccessEmitsNoErrorLogs(t *testing.T) {
	var buf bytes.Buffer
	s := Saga{Name: "t", Steps: []Step{
		{Name: "a", Do: ok(), Compensate: ok()},
	}, Logger: slog.New(slog.NewJSONHandler(&buf, nil))}
	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("got %v", err)
	}
	if out := buf.String(); strings.Contains(out, "ERROR") || strings.Contains(out, "WARN") {
		t.Fatalf("unexpected error logs on success: %s", out)
	}
}

// verifies a run with no logger still fails cleanly via slog.Default.
func TestNilLoggerSafe(t *testing.T) {
	s := Saga{Name: "t", Steps: []Step{{Name: "a", Do: fail(errors.New("x"))}}}
	if err := s.Run(context.Background()); err == nil {
		t.Fatal("expected error with nil logger")
	}
}

// verifies an in-doubt step skips unwind and keeps completed steps reported.
func TestInDoubtSkipsUnwind(t *testing.T) {
	compensated := false
	cause := errors.New("status query inconclusive")
	s := Saga{ID: "s-1", Name: "t", Steps: []Step{
		{Name: "a", Do: ok(), Compensate: func(context.Context) error { compensated = true; return nil }},
		{Name: "b", Do: ok(), Compensate: func(context.Context) error { compensated = true; return nil }},
		{Name: "c", Do: func(context.Context) error { return &InDoubtError{Step: "c", Err: cause} }},
	}}
	err := s.Run(context.Background())
	var re *RunError
	if !errors.As(err, &re) {
		t.Fatalf("expected *RunError, got %T", err)
	}
	if !re.InDoubt || re.FailedStep != "c" || !errors.Is(re.StepErr, cause) {
		t.Fatalf("bad in-doubt identity: %+v", re)
	}
	if len(re.Completed) != 2 {
		t.Fatalf("bad resume data: %+v", re)
	}
	if compensated {
		t.Fatal("compensation ran for in-doubt outcome")
	}
}

// verifies an ordinary error still unwinds in reverse when in-doubt exists elsewhere.
func TestInDoubtOrdinaryErrorStillUnwinds(t *testing.T) {
	undid := &rec{}
	boom := errors.New("boom")
	s := Saga{Name: "t", Steps: []Step{
		{Name: "a", Do: ok(), Compensate: func(context.Context) error { undid.add("a"); return nil }},
		{Name: "b", Do: fail(boom)},
	}}
	err := s.Run(context.Background())
	var re *RunError
	if !errors.As(err, &re) {
		t.Fatalf("expected *RunError, got %T", err)
	}
	if re.InDoubt {
		t.Fatalf("ordinary error flagged in-doubt: %+v", re)
	}
	waitFor(t, "background unwind", func() bool { return len(undid.get()) == 1 })
	if got := undid.get(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("ordinary error did not unwind: %v", got)
	}
}
