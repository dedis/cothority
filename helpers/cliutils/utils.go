package cliutils

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"
	dbg "github.com/ineiti/cothorities/helpers/debug_lvl"
)

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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func SshRun(username, host, command string) ([]byte, error) {
	addr := host
	if username != "" {
		addr = username + "@" + addr
	}
	cmd := exec.Command("ssh", "-o", "StrictHostKeyChecking=no", addr,
		"eval '"+command+"'")
	//log.Println(cmd)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func SshRunStdout(username, host, command string) error {
	addr := host
	if username != "" {
		addr = username + "@" + addr
	}

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

func Build(path, out, goarch, goos string) error {
	var cmd *exec.Cmd
	cmd = exec.Command("go", "build", "-v", "-o", out, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append([]string{"GOOS=" + goos, "GOARCH=" + goarch}, os.Environ()...)
	dbg.Lvl3(os.Getwd())
	dbg.Lvl3("Command:", cmd.Args)
	return cmd.Run()
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
