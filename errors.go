package rrule

import (
	"errors"
	"fmt"
)

var (
	// ErrSyntax is text that RFC 5545's grammar does not allow, such as FREQ=FORTNIGHTLY or BYDAY=1X.
	ErrSyntax = errors.New("rrule: syntax error")

	// ErrInvalid is a value that is well formed but out of range, or not allowed with the rest, such as BYMONTHDAY=32,
	// or COUNT with UNTIL.
	ErrInvalid = errors.New("rrule: invalid")

	// ErrUnsupported is iCalendar that this package does not handle, such as RFC 7529's RSCALE, or an RDATE period.
	ErrUnsupported = errors.New("rrule: unsupported")
)

// PartError is a rule part, a property or a parameter that Parse, ParseSet or Validate refuse. It unwraps to
// ErrSyntax, ErrInvalid, ErrUnsupported, or the error of the time zone loader.
type PartError struct {
	Part   string // such as BYMONTHDAY, DTSTART or TZID
	Value  string // the value refused, if any
	Reason string // why, if Err is not enough
	Err    error
}

func (e *PartError) Error() string {
	s := e.Err.Error() + ", " + e.Part
	if e.Value != "" {
		s += "=" + e.Value
	}

	if e.Reason != "" {
		s += ": " + e.Reason
	}

	return s
}

func (e *PartError) Unwrap() error {
	return e.Err
}

// LineError is returned by ParseSet for a line it refuses. Line counts from 1, and a folded line is numbered by its
// first. Err is a *PartError.
type LineError struct {
	Line int
	Err  error
}

func (e *LineError) Error() string {
	return fmt.Sprintf("%s, on line %d", e.Err, e.Line)
}

func (e *LineError) Unwrap() error {
	return e.Err
}

func partError(err error, part, value, reason string) *PartError {
	return &PartError{Part: part, Value: value, Reason: reason, Err: err}
}
