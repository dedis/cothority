package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
)

// This is required; without it onet/log/testuitl.go:interestingGoroutines will
// call main.main() interesting.
func TestMain(m *testing.M) {
	log.MainTest(m)
}

const blockInterval = 1 * time.Second

func TestCli(t *testing.T) {
	// TODO: Fix this test.
	t.Skip("Disabled for now; we would need to test ol and el at the same time?")

	l := onet.NewTCPTest(cothority.Suite)
	_, _, _ = l.GenTree(2, true)

	defer l.CloseAll()

	// TODO: Make this correct args?
	args := []string{"el", "create"}
	err := cliApp.Run(args)
	require.Nil(t, err)

	// Make sure the eventlogs are in the blockchain.
	time.Sleep(2 * blockInterval)

	args = []string{"el", "log", "-topic", "testTopic1", "-content", "Test Message"}
	err = cliApp.Run(args)
	require.Nil(t, err)

	// Make sure they are commtted to the log (in separate tx, so their
	// order is repeatable.)
	time.Sleep(2 * blockInterval)

	args = []string{"el", "log", "-topic", "testTopic2", "-content", "ldjkf"}
	err = cliApp.Run(args)
	require.Nil(t, err)

	// Make sure they are commtted to the log (in separate tx, so their
	// order is repeatable.)
	time.Sleep(2 * blockInterval)

	t.Log("search: get all")
	b := &bytes.Buffer{}
	cliApp.Writer = b
	cliApp.ErrWriter = b
	args = []string{"el", "search"}
	err = cliApp.Run(args)
	require.Contains(t, string(b.Bytes()), "Test Message")
	require.Contains(t, string(b.Bytes()), "ldjkf")

	t.Log("search: limit by topic")
	b = &bytes.Buffer{}
	cliApp.Writer = b
	cliApp.ErrWriter = b
	args = []string{"el", "search", "-topic", "testTopic1"}
	err = cliApp.Run(args)
	require.Contains(t, string(b.Bytes()), "Test Message")
	require.NotContains(t, string(b.Bytes()), "ldjkf")

	t.Log("search: limit by count")
	b = &bytes.Buffer{}
	cliApp.Writer = b
	cliApp.ErrWriter = b
	args = []string{"el", "search", "-count", "1"}
	err = cliApp.Run(args)
	require.Contains(t, string(b.Bytes()), "Test Message")
	require.NotContains(t, string(b.Bytes()), "ldjkf")

	// It would be interesting to try to write a test for
	// -from/-to, but it would make this test too fragile.
	// TODO:
	// Write a separate test, gated by testing.Slow() that adds
	// one event per second, and then selects some of them using
	// "-from 10s ago -for 5s".
}
