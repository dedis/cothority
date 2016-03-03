package cliutils

import (
	"bufio"
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dedis/cothority/lib/dbg"
)

func Boldify(s string) string {
	return "\033[1m" + s + "\033[0m"
}

func ReadLines(filename string) ([]string, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(b)), nil
}

func Scp(username, host, file, dest string) error {
	addr := host + ":" + dest
	if username != "" {
		addr = username + "@" + addr
	}
	cmd := exec.Command("scp", "-r", file, addr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Rsync(username, host, file, dest string) error {
	addr := host + ":" + dest
	if username != "" {
		addr = username + "@" + addr
	}
	cmd := exec.Command("rsync", "-Pauz", "-e", "ssh -T -c arcfour -o Compression=no -x", file, addr)
	cmd.Stderr = os.Stderr
	if dbg.DebugVisible() > 1 {
		cmd.Stdout = os.Stdout
	}
	return cmd.Run()
}

func SshRun(username, host, command string) ([]byte, error) {
	addr := host
	if username != "" {
		addr = username + "@" + addr
	}

	cmd := exec.Command("ssh", "-o", "StrictHostKeyChecking=no", addr,
		"eval '"+command+"'")
	//cmd.Stderr = os.Stderr
	return cmd.Output()
}

func SshRunStdout(username, host, command string) error {
	addr := host
	if username != "" {
		addr = username + "@" + addr
	}

	dbg.Lvl4("Going to ssh to", addr, command)
	cmd := exec.Command("ssh", "-o", "StrictHostKeyChecking=no", addr,
		"eval '"+command+"'")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func SshRunBackground(username, host, command string) error {
	addr := host
	if username != "" {
		addr = username + "@" + addr
	}

	cmd := exec.Command("ssh", "-v", "-o", "StrictHostKeyChecking=no", addr,
		"eval '"+command+" > /dev/null 2>/dev/null < /dev/null &' > /dev/null 2>/dev/null < /dev/null &")
	return cmd.Run()

}

func Build(path, out, goarch, goos string) (string, error) {
	var cmd *exec.Cmd
	var b bytes.Buffer
	build_buffer := bufio.NewWriter(&b)

	wd, _ := os.Getwd()
	dbg.Lvl4("In directory", wd)

	cmd = exec.Command("go", "build", "-v", "-o", out, path)
	dbg.Lvl4("Building", cmd.Args, "in", path)
	cmd.Stdout = build_buffer
	cmd.Stderr = build_buffer
	cmd.Env = append([]string{"GOOS=" + goos, "GOARCH=" + goarch}, os.Environ()...)
	wd, err := os.Getwd()
	dbg.Lvl4(wd)
	dbg.Lvl4("Command:", cmd.Args)
	err = cmd.Run()
	dbg.Lvl4(b.String())
	return b.String(), err
}

func KillGo() {
	cmd := exec.Command("killall", "go")
	cmd.Run()
}

func TimeoutRun(d time.Duration, f func() error) error {
	echan := make(chan error)
	go func() {
		echan <- f()
	}()
	var e error
	select {
	case e = <-echan:
	case <-time.After(d):
		e = errors.New("function timed out")
	}
	return e
}
