package errors

import (
	stdliberrors "errors"
	"testing"

	pkgerrors "github.com/pkg/errors"
)

func TestUserError(t *testing.T) {
	orig := stdliberrors.New("something happened")
	ue := UserError(orig)
	if !IsUserError(ue) {
		t.Fatalf("should have been a user error")
	}
	if IsSystemError(ue) {
		t.Fatalf("should not have been a system error")
	}
	orig = pkgerrors.Wrap(orig, "error in foo")
	orig = pkgerrors.Wrap(orig, "error in bar")
	orig = pkgerrors.Wrap(orig, "error in baz")
	t.Logf("orig: %v", orig)

	ue = UserError(orig)
	t.Logf("user err: %v", ue)

	if !IsUserError(ue) {
		t.Fatalf("should have been a user error")
	}
	if IsSystemError(ue) {
		t.Fatalf("should not have been a system error")
	}
	if IsUserError(stdliberrors.New("something else")) {
		t.Fatalf("standard error should not have been a user error")
	}

	// if called multiple times on the same value, the last one counts
	ue = UserError(orig)
	ue = SystemError(ue)
	if IsUserError(ue) {
		t.Fatalf("multiple error value should not have been a user error")
	}
	if !IsSystemError(ue) {
		t.Fatalf("multiple error value should have been a system error")
	}

	if UserError(nil) != nil {
		t.Fatalf("should have returned nil")
	}

	if IsUserError(nil) {
		t.Fatalf("nil should have returned false")
	}
}

func TestSystemError(t *testing.T) {
	orig := stdliberrors.New("something happened")
	se := SystemError(orig)
	if IsUserError(se) {
		t.Fatalf("should not have been a user error")
	}
	if !IsSystemError(se) {
		t.Fatalf("should have been a system error")
	}
	orig = pkgerrors.Wrap(orig, "error in foo")
	orig = pkgerrors.Wrap(orig, "error in bar")
	orig = pkgerrors.Wrap(orig, "error in baz")

	se = SystemError(orig)

	if IsUserError(se) {
		t.Fatalf("should not have been a user error")
	}
	if !IsSystemError(se) {
		t.Fatalf("should have been a system error")
	}
	if IsSystemError(stdliberrors.New("something else")) {
		t.Fatalf("standard error should not have been a system error")
	}
	if IsSystemError(nil) {
		t.Fatalf("nil should not be a system error")
	}
}

type customError struct {
	message string
}

func (ce customError) Error() string { return ce.message }

func TestSystemErrorUnwrapped(t *testing.T) {

	ce := customError{message: "custom error"}
	se := SystemError(ce)

	res := customError{}
	if ok := stdliberrors.As(se, &res); !ok {
		t.Fatalf("expected error to be unwrapped properly and found by errors.As")
	}
}

func TestUserErrorUnwrapped(t *testing.T) {
	ce := customError{message: "custom error"}
	ue := UserError(ce)

	res := customError{}
	if ok := stdliberrors.As(ue, &res); !ok {
		t.Fatalf("expected error to be unwrapped properly and found by errors.As")
	}
}
