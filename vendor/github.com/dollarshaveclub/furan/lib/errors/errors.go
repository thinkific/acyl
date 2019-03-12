package errors

type user interface {
	User() bool
}

// UserError is an error caused by invalid user input
type UserError string

// User returns true if the error is caused by the user
func (e UserError) User() bool    { return true }
func (e UserError) Error() string { return "user error: " + string(e) }

// IsUserError returns true if err is caused by the user
func IsUserError(err error) bool {
	e, ok := err.(user)
	if ok {
		return e.User()
	}
	return false
}
