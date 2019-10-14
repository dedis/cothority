package cothority

import (
	"fmt"

	"golang.org/x/xerrors"
)

// Error is a wrapper around an standard error that allows
// to print the stack trace from the call of the constructor.
type Error struct {
	err   error
	msg   string
	frame xerrors.Frame
}

// ErrorOrNil returns the error if any with the stack trace
// beginning at the call of the function.
func ErrorOrNil(err error, msg string) error {
	return ErrorOrNilSkip(err, msg, 1)
}

// ErrorOrNilSkip returns the error if any with the stack trace
// beginning at the call of the skip-nth caller.
func ErrorOrNilSkip(err error, msg string, skip int) error {
	if err == nil {
		return nil
	}
	return &Error{
		err:   err,
		msg:   msg,
		frame: xerrors.Caller(skip),
	}
}

// WrapError returns a wrapper of the error is it can be used
// for comparison.
func WrapError(err error) error {
	return ErrorOrNilSkip(err, "", 2)
}

func (e *Error) Error() string {
	if e.msg != "" {
		return e.msg + ": " + fmt.Sprintf("%v", e.err)
	}
	return fmt.Sprintf("%v", e.err)
}

// Unwrap returns the next error in the chain.
func (e *Error) Unwrap() error {
	return e.err
}

// Format prints the error to the formatter.
func (e *Error) Format(f fmt.State, c rune) {
	xerrors.FormatError(e, f, c)
}

// FormatError prints the error to the printer. It prints
// the stack trace when the '+' is used in combination with
// 'v'.
func (e *Error) FormatError(p xerrors.Printer) error {
	if e.msg != "" {
		p.Printf("%s: %v", e.msg, e.err)
	} else {
		p.Printf("%v", e.err)
	}

	if p.Detail() {
		e.frame.Format(p)
		p.Printf("%+v", e.err)
	}
	return nil
}
