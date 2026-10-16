package saga

import "fmt"

// RunError is the failure result. Completed holds executed step names in
// order — the unwind plan the caller queues for a worker. InDoubt reports
// the failure is unconfirmed and no compensation is owed. Compensation
// itself runs in the worker after Run returns; its errors are worker logs,
// not returned.
type RunError struct {
	SagaID     string
	SagaName   string
	FailedStep string
	StepErr    error
	Completed  []string
	InDoubt    bool
}

func (e *RunError) Error() string {
	if e.InDoubt {
		return fmt.Sprintf("saga %s: step %s in-doubt: %v (no compensation ran)",
			e.SagaID, e.FailedStep, e.StepErr)
	}
	return fmt.Sprintf("saga %s: step %s failed: %v (unwind queued)",
		e.SagaID, e.FailedStep, e.StepErr)
}
func (e *RunError) Unwrap() error { return e.StepErr }

// InDoubtError signals that a step's outcome could not be confirmed
// (e.g. remote call timed out and status query was inconclusive).
// Run skips unwind for it: completed work is kept for later reconciliation.
type InDoubtError struct {
	Step string
	Err  error
}

func (e *InDoubtError) Error() string { return fmt.Sprintf("in-doubt %s: %v", e.Step, e.Err) }
func (e *InDoubtError) Unwrap() error { return e.Err }
