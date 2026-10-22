package saga

type StepStatus string

const (
	StepPending            StepStatus = "PENDING"
	StepRunning            StepStatus = "RUNNING"
	StepCompleted          StepStatus = "COMPLETED"
	StepFailed             StepStatus = "FAILED"
	StepCompensating       StepStatus = "COMPENSATING"
	StepCompensated        StepStatus = "COMPENSATED"
	StepCompensationFailed StepStatus = "COMPENSATION_FAILED"
	StepSkipped            StepStatus = "SKIPPED"
)
