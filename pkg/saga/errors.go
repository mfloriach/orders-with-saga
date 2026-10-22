package saga

import (
	"fmt"
	"uuid"
)

type SagaError struct {
	ID      uuid.UUID
	Step    string
	Err     error
	Details string
}

func (s SagaError) Error() string {
	return fmt.Sprintf("saga error: %v, %v, %v", s.Step, s.Err, s.Details)
}
