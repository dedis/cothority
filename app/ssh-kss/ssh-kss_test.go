package main

import (
	"testing"

	"io/ioutil"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

func TestAddLine(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "file.txt")
	dbg.ErrFatal(err)
	tmpfile.WriteString("Line1\n")
	tmpfile.Close()
	name := tmpfile.Name()
	addLine(name, "Line2")
	lines, err := ioutil.ReadFile(name)
	dbg.ErrFatal(err)
	assert.Equal(t, "Line1\nLine2\n", string(lines))
	deleteLine(name, "Line1")
	lines, err = ioutil.ReadFile(name)
	dbg.ErrFatal(err)
	assert.Equal(t, "Line2\n", string(lines))
}
