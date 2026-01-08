package errorx

import (
	"errors"
	"fmt"
	"os"
)

// ContextError wraps an error with a short operation context.
// This keeps error chains readable while still preserving the original error via Unwrap.
type ContextError struct {
	Op  string
	Err error
}

func (e *ContextError) Error() string {
	if e == nil {
		return ""
	}
	if e.Op == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *ContextError) Unwrap() error { return e.Err }

// WithContextError wraps err with an operation label.
func WithContextError(op string, err error) error {
	if err == nil {
		return nil
	}
	return &ContextError{Op: op, Err: err}
}

// WrapIfErr is meant to be used with defer to add context on return.
// Example:
//
//	func f() (err error) {
//	  defer errorx.WrapIfErr(&err, "sync.push")
//	  ...
//	}
func WrapIfErr(errp *error, op string) {
	if errp == nil || *errp == nil {
		return
	}
	*errp = WithContextError(op, *errp)
}

// UserError converts internal errors into a user-facing message.
func UserError(err error) string {
	if err == nil {
		return ""
	}
	if err.Error() == "aborted" {
		return "Aborted."
	}
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Sprintf("Not found: %v", err)
	}
	return err.Error()
}
