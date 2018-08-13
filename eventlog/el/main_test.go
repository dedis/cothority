package main

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/onet/log"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/stretchr/testify/require"
)

// This is required; without it onet/log/testuitl.go:interestingGoroutines will
// call main.main() interesting.
func TestMain(m *testing.M) {
	log.MainTest(m)
}

const blockInterval = 1 * time.Second

func TestCli(t *testing.T) {
	dir, err := ioutil.TempDir("", "el-test")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	// Start a contained cothority, just like Java testing does.
	cname := path.Base(dir)
	cmd := exec.Command("docker", "run", "--rm",
		"-p7003:7003", "-p7005:7005", "-p7007:7007", "-p7009:7009",
		"--detach", "--name", cname, "dedis/conode-test:latest")
	if testing.Verbose() {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	// A goroutine to copy logs from Docker.
	if testing.Verbose() {
		go func() {
			client, _ := client.NewEnvClient()
			reader, err := client.ContainerLogs(context.Background(), cname,
				types.ContainerLogsOptions{
					Follow:     true,
					ShowStdout: true,
				})
			if err != nil {
				t.Fatal(err)
			}

			_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, reader)
			if err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}()
	}

	defer func() {
		cmd := exec.Command("docker", "kill", cname)
		if testing.Verbose() {
			cmd.Stdout = os.Stdout
		}
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			t.Fatal(err)
		}
	}()

	// Make a new OmniLedger in the contained cothority.
	cmd = exec.Command("docker", "exec", cname, "./ol", "create", "--roster", "public.toml", "--interval", blockInterval.String(), "--config", "./ol-cfg")
	if testing.Verbose() {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)

	// Bring back the ol-cfg file so we can use it locally.
	cmd = exec.Command("docker", "exec", cname, "cat", "./ol-cfg")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	olCfg := path.Join(dir, "ol-cfg")
	err = ioutil.WriteFile(olCfg, out, 0644)
	require.NoError(t, err)

	// Add the rules to the Darc.
	s := darc.NewSignerEd25519(nil, nil)
	private := s.Ed25519.Secret.String()

	for _, r := range []string{"spawn:eventlog", "invoke:eventlog"} {
		cmd = exec.Command("docker", "exec", cname, "./ol", "add", r, "--ol", "./ol-cfg",
			"--identity", s.Identity().String())
		if testing.Verbose() {
			cmd.Stdout = os.Stdout
		}
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		require.NoError(t, err)
	}

	cliApp.Writer = ioutil.Discard
	args := []string{"el", "create", "--ol", olCfg, "--priv", private}
	err = cliApp.Run(args)
	require.Nil(t, err)
	el := cliApp.Metadata["el"].(string)

	args = []string{"el", "log", "--ol", olCfg, "--priv", private, "--el", el, "-topic", "testTopic1", "-content", "Test Message"}
	err = cliApp.Run(args)
	require.Nil(t, err)
	args = []string{"el", "log", "--ol", olCfg, "--priv", private, "--el", el, "-topic", "testTopic2", "-content", "ldjkf"}
	err = cliApp.Run(args)
	require.Nil(t, err)

	// Make sure they are commtted to the log
	time.Sleep(2 * blockInterval)

	t.Log("search: get all")
	b := &bytes.Buffer{}
	cliApp.Writer = b
	cliApp.ErrWriter = b
	args = []string{"el", "search", "--ol", olCfg, "--el", el}
	err = cliApp.Run(args)
	require.Contains(t, string(b.Bytes()), "Test Message")
	require.Contains(t, string(b.Bytes()), "ldjkf")

	t.Log("search: limit by topic")
	b = &bytes.Buffer{}
	cliApp.Writer = b
	cliApp.ErrWriter = b
	args = []string{"el", "search", "--ol", olCfg, "--el", el, "-topic", "testTopic1"}
	err = cliApp.Run(args)
	require.Contains(t, string(b.Bytes()), "Test Message")
	require.NotContains(t, string(b.Bytes()), "ldjkf")

	t.Log("search: limit by count")
	b = &bytes.Buffer{}
	cliApp.Writer = b
	cliApp.ErrWriter = b
	args = []string{"el", "search", "-count", "1", "--ol", olCfg, "--el", el}
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
