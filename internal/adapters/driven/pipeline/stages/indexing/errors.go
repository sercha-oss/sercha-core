package indexing

import "fmt"

// StageError represents an error from a stage.
type StageError struct {
	Stage   string
	Message string
	Err     error
}

func (e *StageError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("stage %s: %s: %v", e.Stage, e.Message, e.Err)
	}
	return fmt.Sprintf("stage %s: %s", e.Stage, e.Message)
}

func (e *StageError) Unwrap() error {
	return e.Err
}
