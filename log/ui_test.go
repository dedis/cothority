package log

import (
	"testing"

	"bufio"
	"bytes"
	"os"

	"github.com/stretchr/testify/assert"
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

func TestMain(m *testing.M) {
	stdToBuf()
	MainTest(m)
}

func TestInfo(t *testing.T) {
	SetDebugVisible(FormatPython)
	Info("Python")
	assert.Equal(t, "[+] Python\n", getStdOut())
	SetDebugVisible(FormatNone)
	Info("None")
	assert.Equal(t, "None\n", getStdOut())
	Info("None", "Python")
	assert.Equal(t, "None Python\n", getStdOut())
	SetDebugVisible(1)
}

func TestLvl(t *testing.T) {
	SetDebugVisible(1)
	Info("TestLvl")
	assert.Equal(t, "I : (                             log.TestLvl:   0) - TestLvl\n",
		getStdOut())
	Print("TestLvl")
	assert.Equal(t, "I : (                             log.TestLvl:   0) - TestLvl\n",
		getStdOut())
	Warn("TestLvl")
	assert.Equal(t, "W : (                             log.TestLvl:   0) - TestLvl\n",
		getStdErr())
}
