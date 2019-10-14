package cothority

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/xerrors"
)

var errExample = xerrors.New("example")

func makeError() error {
	return xerrors.Errorf("oops: %w", errExample)
}

// Test that the basic function create an error when the parameter
// is not nil, and returns nil otherwise.
func TestError_ErrorOrNil(t *testing.T) {
	err := ErrorOrNil(makeError(), "test")

	require.Equal(t, "test: oops: example", err.Error())
	require.Nil(t, ErrorOrNil(nil, ""))
}

// Test that the skip option is correctly used to prevent a call
// to be included in the stack trace.
func TestError_ErrorOrNilSkip(t *testing.T) {
	err := ErrorOrNilSkip(makeError(), "test", 2)

	println(t.Name())
	require.NotContains(t, fmt.Sprintf("%+v", err), t.Name())
	require.Contains(t, fmt.Sprintf("%+v", err), ".makeError")
}

// Test that the wrapper is invisible but allows the error
// comparison to work.
func TestError_WrapError(t *testing.T) {
	err := WrapError(makeError())

	require.Equal(t, "oops: example", err.Error())
	require.Contains(t, fmt.Sprintf("%+v", err), ".makeError")
	require.True(t, xerrors.Is(err, errExample))
	require.False(t, xerrors.Is(err, xerrors.New("abc")))
}
