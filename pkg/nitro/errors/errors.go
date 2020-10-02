package errors

import (
	"errors"
)

type operationError struct {
	inner        error
	user, system bool
}

func (oe operationError) Error() string {
	return oe.inner.Error()
}

// Implement Unwrap so this error can be friendly for Go's errors.Is and errors.As implementations.
func (oe operationError) Unwrap() error {
	return oe.inner
}

// UserError annotates err in such a way that IsUserError() can be used further up in the callstack.
func UserError(err error) error {
	if err == nil {
		return nil
	}
	return operationError{user: true, inner: err}
}

// SystemError annotates err in such a way that IsSystemError() can be used further up in the callstack.
func SystemError(err error) error {
	if err == nil {
		return nil
	}
	return operationError{system: true, inner: err}
}

// IsUserError finds the first nitro error in the chain and returns true if it is a user error.
func IsUserError(err error) bool {
	var e operationError
	if errors.As(err, &e) {
		return e.user
	}
	return false
}

// IsSystemError finds the first nitro error in the chain and returns true if it is a system error.
func IsSystemError(err error) bool {
	var e operationError
	if errors.As(err, &e) {
		return e.system
	}
	return false
}
