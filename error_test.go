package cothority

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test that the basic function create an error when the parameter
// is not nil, and returns nil otherwise.
func TestError_ErrorOrNil(t *testing.T) {
	err := ErrorOrNil(errors.New("example-error"), "oops")

	require.Equal(t, "oops: example-error", err.Error())
	require.Nil(t, ErrorOrNil(nil, ""))
}
