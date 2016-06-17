package log

import (
	"io"
	"os"
)

// For debugging purposes we can change the output-writer
var stdOut io.Writer
var stdErr io.Writer

func init() {
	stdOut = os.Stdout
	stdErr = os.Stderr
}

const (
	lvlWarning = iota - 20
	lvlError
	lvlFatal
	lvlPanic
	lvlInfo
	lvlPrint
)

// Format indicates how the prints will be shown
var Format = FormatLvl

const (
	// FormatLvl will print the line-number and method of the caller
	FormatLvl = iota
	// FormatPython uses [x] and others to indicate what is shown
	FormatPython
	// FormatNone is just pure print
	FormatNone
)
