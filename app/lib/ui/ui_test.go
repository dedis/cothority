package ui

import (
	"bytes"
	"testing"

	"bufio"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/stretchr/testify/assert"
)

var b bytes.Buffer
var o *bufio.Writer

func TestMain(m *testing.M) {
	o = bufio.NewWriter(&b)
	out = o
	dbg.MainTest(m)
}

func TestInput(t *testing.T) {
	setInput("Y")
	assert.Equal(t, "Y", Input("def", "Question"))
	assert.Equal(t, "[?] Question [def]: ", getOutput())
	setInput("")
	assert.Equal(t, "def", Input("def", "Question"))
}

func TestInputYN(t *testing.T) {
	setInput("")
	assert.True(t, InputYN(true))
	setInput("")
	assert.False(t, InputYN(false, "Are you sure?"))
	assert.Equal(t, "[?] Are you sure? [Ny]: ", getOutput())
	setInput("")
	assert.True(t, InputYN(true, "Are you sure?"))
	assert.Equal(t, "[?] Are you sure? [Yn]: ", getOutput(), "one")
}

func TestInfo(t *testing.T) {
	Info("Python")
	assert.Equal(t, "[+] Python\n", getOutput())
	Format = FormatNone
	Info("None")
	assert.Equal(t, "None\n", getOutput())
}

func setInput(s string) {
	// Flush output
	getOutput()
	in = bytes.NewReader([]byte(s + "\n"))
}

func getOutput() string {
	o.Flush()
	ret := b.String()
	b.Reset()
	return ret
}
