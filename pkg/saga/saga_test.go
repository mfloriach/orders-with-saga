package saga

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func ok() func(context.Context) error { return func(context.Context) error { return nil } }
func fail(err error) func(context.Context) error {
	return func(context.Context) error { return err }
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

// verifies a mid-chain failure skips later steps and compensates in reverse with resume data.
func TestFailureStopsForwardAndUnwindsReverse(t *testing.T) {
	var did, undid []string
	boom := errors.New("boom")
	do := func(name string, err error) func(context.Context) error {
		return func(context.Context) error { did = append(did, name); return err }
	}
	s := Saga{ID: "s-1", Name: "t", Steps: []Step{
		{Name: "a", Do: do("a", nil), Compensate: func(context.Context) error { undid = append(undid, "a"); return nil }},
		{Name: "b", Do: do("b", nil), Compensate: func(context.Context) error { undid = append(undid, "b"); return nil }},
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
	if len(re.CompensateErrs) != 0 {
		t.Fatalf("compensations failed: %+v", re.CompensateErrs)
	}
	if re.SagaID != "s-1" || len(re.Completed) != 2 || re.Completed[0] != "a" {
		t.Fatalf("bad resume data: %+v", re)
	}
	if len(undid) != 2 || undid[0] != "b" || undid[1] != "a" {
		t.Fatalf("bad unwind order: %v", undid)
	}
}

// verifies nil compensations are skipped and the failed step itself is never compensated.
func TestNilCompensateSkippedAndFailedStepNotCompensated(t *testing.T) {
	var ran []string
	s := Saga{Name: "t", Steps: []Step{
		{Name: "a", Do: ok()}, // nil compensate
		{Name: "b", Do: fail(errors.New("x")), Compensate: func(context.Context) error {
			ran = append(ran, "b")
			return nil
		}},
	}}
	if err := s.Run(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if len(ran) != 0 {
		t.Fatalf("failed step compensated: %v", ran)
	}
}

// verifies a failing compensation is collected without aborting the remaining unwind.
func TestCompensateErrorsAggregatedAndUnwindContinues(t *testing.T) {
	var ran []string
	ce := errors.New("undo-bad")
	s := Saga{Name: "t", Steps: []Step{
		{Name: "a", Do: ok(), Compensate: func(context.Context) error { ran = append(ran, "a"); return nil }},
		{Name: "b", Do: ok(), Compensate: func(context.Context) error { ran = append(ran, "b"); return ce }},
		{Name: "c", Do: fail(errors.New("main"))},
	}}
	err := s.Run(context.Background())
	var re *RunError
	if !errors.As(err, &re) {
		t.Fatalf("expected *RunError, got %T", err)
	}
	if len(re.CompensateErrs) != 1 || re.CompensateErrs[0].Step != "b" || !errors.Is(re.CompensateErrs[0].Err, ce) {
		t.Fatalf("bad compensate errors: %+v", re.CompensateErrs)
	}
	if len(ran) != 2 || ran[0] != "b" || ran[1] != "a" {
		t.Fatalf("unwind did not continue: %v", ran)
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
	default:
		t.Fatal("completed step was not compensated")
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

// verifies per-step spans exist and failure logs carry saga identity.
func TestObservabilitySpansAndLogs(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(old)

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	boom := errors.New("kaboom")
	s := Saga{Name: "pay", Steps: []Step{
		{Name: "a", Do: ok(), Compensate: ok()},
		{Name: "b", Do: fail(boom), Compensate: ok()},
	}, Logger: log}
	if err := s.Run(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	names := map[string]bool{}
	for _, sp := range sr.Ended() {
		names[sp.Name()] = true
	}
	for _, want := range []string{"saga.pay", "saga.step.a", "saga.step.b", "saga.compensate.a"} {
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

// verifies step failure logs ERROR and compensate failure logs WARN with saga identity.
func TestFailureLogsCarrySagaIdentity(t *testing.T) {
	var buf bytes.Buffer
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
