package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	stdToBuf()
	MainTest(m)
}

func TestInfo(t *testing.T) {
	Format = FormatPython
	Info("Python")
	assert.Equal(t, "[+] Python\n", getStdOut())
	Format = FormatNone
	Info("None")
	assert.Equal(t, "None\n", getStdOut())
	Info("None", "Python")
	assert.Equal(t, "None Python\n", getStdOut())
	Format = FormatLvl
}
