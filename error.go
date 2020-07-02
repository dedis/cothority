package cothority

import (
	"fmt"
)

// ErrorOrNil returns the error if any with the stack trace
// beginning at the call of the function.
func ErrorOrNil(err error, msg string) error {
	if err == nil {
		return err
	}
	return fmt.Errorf("%s: %v", msg, err)
}
