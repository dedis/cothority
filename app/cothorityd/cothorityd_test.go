package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"testing"
)

func TestCompileAndRun(t *testing.T) {
	// binary named after the package:
	bin := "./cothorityd"
	build := exec.Command("go", "build")
	err := build.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Remove(bin)
		build.Process.Kill()
	}()
	if err = runCommand(bin, makeReader("129u.898.9090e:21-2"), "setup"); err == nil {
		t.Fatal("There should be an error:", err)
	}

	// Test with valid IP + config name
	configName := "config.toml.test"
	if err = verifyCorrectOutput(bin, makeReader("127.0.0.1:2000", "127.0.0.1:2000", configName), "Addresses", "setup"); err != nil {
		t.Fatal(err)
	}

	// Test without giving anything => use the already existing config name
	if err = verifyCorrectOutput(bin, nil, "", "-config", configName); err != nil && err != io.EOF {
		t.Fatal("There should NOT be an error", err)
	}

	if err = os.Remove(configName); err != nil {
		t.Fatal("Error deleting the config file?", err)
	}
}

func runCommand(cmd string, input io.Reader, args ...string) error {
	cmdExec := exec.Command(cmd, args...)

	cmdExec.Stdin = input

	err := cmdExec.Run()
	return err
}

func makeReader(input ...string) io.Reader {
	var buff = new(bytes.Buffer)
	for _, s := range input {
		buff.WriteString(s + "\n")
	}
	return buff
}

func verifyCorrectOutput(cmdStr string, input io.Reader, output string, args ...string) error {
	cmd := exec.Command(cmdStr, args...)
	cmd.Stdin = input
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	defer cmd.Process.Kill()
	var found bool
	var buff = make([]byte, 1024)
	if output == "" {
		return nil
	}
	for {
		n, err := stdout.Read(buff)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if n == 0 {
			return errors.New("No output")
		}
		if bytes.Contains(buff, []byte(output)) {
			found = true
			break
		}
	}
	if !found {
		return errors.New("No Correct Output")
	}
	return nil
}
