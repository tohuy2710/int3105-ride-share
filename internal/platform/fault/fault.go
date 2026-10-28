// Package fault defines application errors without HTTP or infrastructure imports.
package fault

import "errors"

type Error struct{ Code, Message string }

func (e *Error) Error() string        { return e.Message }
func New(code, message string) *Error { return &Error{code, message} }
func Code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "INTERNAL_ERROR"
}

var (
	NotFound        = New("RIDE_NOT_FOUND", "Ride not found")
	Forbidden       = New("FORBIDDEN", "Access denied")
	Unavailable     = New("DEPENDENCY_UNAVAILABLE", "Dependency unavailable")
	InvalidState    = New("INVALID_RIDE_STATE", "Invalid ride state")
	AlreadyAccepted = New("RIDE_ALREADY_ACCEPTED", "Ride already accepted")
	InProgress      = New("ACCEPT_IN_PROGRESS", "Another acceptance is in progress")
	Unauthorized    = New("UNAUTHORIZED", "Invalid credentials or token")
)
