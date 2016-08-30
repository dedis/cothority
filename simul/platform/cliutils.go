package platform

// Used for shell-commands

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"

	"io"

	"path"

	"github.com/dedis/cothority/log"
)

// Scp copies the given files to the remote host
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

// Copy makes a copy of a local file with the same file-mode-bits set.
func Copy(dst, src string) error {
	info, err := os.Stat(dst)
	if err == nil && info.IsDir() {
		return Copy(path.Join(dst, path.Base(src)), src)
	}
	fSrc, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fSrc.Close()
	stat, err := fSrc.Stat()
	if err != nil {
		return err
	}
	fDst, err := os.OpenFile(dst, os.O_CREATE|os.O_RDWR, stat.Mode())
	if err != nil {
		return err
	}
	defer fDst.Close()
	_, err = io.Copy(fDst, fSrc)
	return err
}

// Rsync copies files or directories to the remote host. If the DebugVisible
// is > 1, the rsync-operation is displayed on screen.
func Rsync(username, host, file, dest string) error {
	addr := host + ":" + dest
	if username != "" {
		addr = username + "@" + addr
	}
	cmd := exec.Command("rsync", "-Pauz", "-e", "ssh -T -o Compression=no -x", file, addr)
	cmd.Stderr = os.Stderr
	if log.DebugVisible() > 1 {
		cmd.Stdout = os.Stdout
	}
	return cmd.Run()
}

// SSHRun runs a command on the remote host
func SSHRun(username, host, command string) ([]byte, error) {
	addr := host
	if username != "" {
		addr = username + "@" + addr
	}

	cmd := exec.Command("ssh", "-o", "StrictHostKeyChecking=no", addr,
		"eval '"+command+"'")
	return cmd.Output()
}

// SSHRunStdout runs a command on the remote host but redirects stdout and
// stderr of the Ssh-command to the os.Stderr and os.Stdout
func SSHRunStdout(username, host, command string) error {
	addr := host
	if username != "" {
		addr = username + "@" + addr
	}

	log.Lvl4("Going to ssh to", addr, command)
	cmd := exec.Command("ssh", "-o", "StrictHostKeyChecking=no", addr,
		"eval '"+command+"'")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// Build builds the the golang packages in `path` and stores the result in `out`. Besides specifying the environment
// variables GOOS and GOARCH you can pass any additional argument using the buildArgs
// argument. The command which will be executed is of the following form:
// $ go build -v buildArgs... -o out path
func Build(path, out, goarch, goos string, buildArgs ...string) (string, error) {
	var cmd *exec.Cmd
	var b bytes.Buffer
	buildBuffer := bufio.NewWriter(&b)

	wd, _ := os.Getwd()
	log.Lvl4("In directory", wd)
	var args []string
	args = append(args, "build", "-v")
	args = append(args, buildArgs...)
	args = append(args, "-o", out, path)
	cmd = exec.Command("go", args...)
	log.Lvl4("Building", cmd.Args, "in", path)
	cmd.Stdout = buildBuffer
	cmd.Stderr = buildBuffer
	cmd.Env = append([]string{"GOOS=" + goos, "GOARCH=" + goarch}, os.Environ()...)
	wd, err := os.Getwd()
	log.Lvl4(wd)
	log.LLvl4("Command:", cmd.Args)
	err = cmd.Run()
	log.Lvl4(b.String())
	return b.String(), err
}

// KillGo kills all go-instances
func KillGo() {
	cmd := exec.Command("killall", "go")
	if err := cmd.Run(); err != nil {
		log.Lvl3("Couldn't kill all go instances:", err)
	}
}
