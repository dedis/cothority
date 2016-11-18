package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	StdToBuf()
	MainTest(m)
}

func TestInfo(t *testing.T) {
	SetDebugVisible(FormatPython)
	Info("Python")
	assert.Equal(t, "[+] Python\n", GetStdOut())
	SetDebugVisible(FormatNone)
	Info("None")
	assert.Equal(t, "None\n", GetStdOut())
	Info("None", "Python")
	assert.Equal(t, "None Python\n", GetStdOut())
	SetDebugVisible(1)
}

func TestLvl(t *testing.T) {
	SetDebugVisible(1)
	Info("TestLvl")
	assert.Equal(t, "I : (                             log.TestLvl:   0) - TestLvl\n",
		GetStdOut())
	Print("TestLvl")
	assert.Equal(t, "I : (                             log.TestLvl:   0) - TestLvl\n",
		GetStdOut())
	Warn("TestLvl")
	assert.Equal(t, "W : (                             log.TestLvl:   0) - TestLvl\n",
		GetStdErr())
}
