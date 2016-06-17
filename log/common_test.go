package log

import (
	"bufio"
	"bytes"
	"os"
)

var bufStdOut bytes.Buffer
var testStdOut *bufio.Writer
var bufStdErr bytes.Buffer
var testStdErr *bufio.Writer

func stdToBuf() {
	testStdOut = bufio.NewWriter(&bufStdOut)
	stdOut = testStdOut
	testStdErr = bufio.NewWriter(&bufStdErr)
	stdErr = testStdErr
}

func stdToOs() {
	stdOut = os.Stdout
	stdErr = os.Stderr
}

func getStdOut() string {
	testStdOut.Flush()
	ret := bufStdOut.String()
	bufStdOut.Reset()
	return ret
}

func getStdErr() string {
	testStdErr.Flush()
	ret := bufStdErr.String()
	bufStdErr.Reset()
	return ret
}
