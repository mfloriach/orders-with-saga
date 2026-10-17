package saga

import (
	"context"
	"errors"
	"testing"
)

// fnStep is a Step[string] with injectable Do behavior.
type fnStep struct {
	name string
	do   func(context.Context, *string) error
}

func (s fnStep) Name() string { return s.name }

func (s fnStep) Do(ctx context.Context, item *string) error {
	if s.do == nil {
		return nil
	}
	return s.do(ctx, item)
}

func (s fnStep) Compensate(context.Context, *string) error { return nil }

// TestOnStepObservesExecutedSteps verifies the hook fires once per
// executed step, in order, with its result — including the failed step
// and never an unexecuted one — without affecting the saga outcome.
func TestOnStepObservesExecutedSteps(t *testing.T) {
	type seen struct {
		step string
		err  error
	}
	var got []seen
	item := "x"
	boom := errors.New("boom")
	s := New("t", []Step[string]{
		fnStep{name: "a"},
		fnStep{name: "b", do: func(context.Context, *string) error { return boom }},
		fnStep{name: "c"},
	}, WithOnStep(func(_ context.Context, it *string, step string, err error) {
		if it != &item {
			t.Errorf("hook got item %p, want %p", it, &item)
		}
		got = append(got, seen{step, err})
	}))

	err := s.Run(context.Background(), &item)
	var re *RunError
	if !errors.As(err, &re) || re.FailedStep != "b" {
		t.Fatalf("expected RunError at b, got %v", err)
	}
	if len(got) != 2 || got[0].step != "a" || got[0].err != nil {
		t.Fatalf("bad first observation: %+v", got)
	}
	if got[1].step != "b" || !errors.Is(got[1].err, boom) {
		t.Fatalf("bad failure observation: %+v", got)
	}
}

// TestRunWithoutHook verifies a nil hook observes nothing and changes
// nothing: success still returns nil.
func TestRunWithoutHook(t *testing.T) {
	s := New("t", []Step[string]{fnStep{name: "a"}})
	if err := s.Run(context.Background(), new(string)); err != nil {
		t.Fatalf("got %v", err)
	}
}
