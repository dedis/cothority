package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

var in *bufio.Reader
var out io.Writer

func init() {
	in = bufio.NewReader(os.Stdin)
	out = os.Stdout
}

// Format indicates how the prints will be shown
var Format = FormatPython

const (
	// FormatPython uses [x] and others to indicate what is shown
	FormatPython = iota
	// FormatNone is just pure print
	FormatNone
)

const (
	info = iota
	warn
	err
	fatal
	input
)

// Info prints the arguments given with a 'info'-format
func Info(args ...interface{}) {
	print(info, args...)
}

// Infof takes a format-string and calls Info
func Infof(f string, args ...interface{}) {
	Info(fmt.Sprintf(f, args...))
}

// Warn prints the arguments given with a 'warn'-format
func Warn(args ...interface{}) {
	print(warn, args...)
}

// Warnf takes a format and calls Warn
func Warnf(f string, args ...interface{}) {
	Warn(fmt.Sprintf(f, args...))
}

// Error prints the arguments given with a 'err'-format
func Error(args ...interface{}) {
	print(err, args...)
}

// Errorf takes a format and calls Error
func Errorf(f string, args ...interface{}) {
	Error(fmt.Sprintf(f, args...))
}

// Fatal prints the arguments given with a 'fatal'-format
// and calls os.Exit
func Fatal(args ...interface{}) {
	print(fatal, args...)
	os.Exit(1)
}

// Fatalf takes a format and calls Fatal
func Fatalf(f string, args ...interface{}) {
	Fatal(fmt.Sprintf(f, args...))
}

// ErrFatal will call Fatal when the error is non-nil
func ErrFatal(err error, args ...interface{}) {
	if err != nil {
		Fatal(err.Error(), "\n", fmt.Sprint(args...))
	}
}

// ErrFatalf will call Fatalf when the error is non-nil
func ErrFatalf(err error, f string, args ...interface{}) {
	ErrFatal(err, fmt.Sprintf(f, args...))
}

// Input prints the arguments given with a 'input'-format and
// proposes the 'def' string as default. If the user presses
// 'enter', the 'dev' will be returned.
func Input(def string, args ...interface{}) string {
	print(input, args...)
	fmt.Fprintf(out, " [%s]: ", def)
	str, err := in.ReadString('\n')
	if err != nil {
		Fatal("Could not read input.")
	}
	str = strings.TrimSpace(str)
	if str == "" {
		return def
	} else {
		return str
	}
}

// Inputf takes a format and calls Input
func Inputf(def string, f string, args ...interface{}) string {
	return Input(def, fmt.Sprintf(f, args...))
}

// InputYN asks a Yes/No question
func InputYN(def bool, args ...interface{}) bool {
	defStr := "Yn"
	if !def {
		defStr = "Ny"
	}
	return strings.ToLower(string(Input(defStr, args...)[0])) == "y"
}

func print(lvl int, args ...interface{}) {
	switch Format {
	case FormatPython:
		prefix := []string{"[+]", "[-]", "[!]", "[X]", "[?]"}
		fmt.Fprint(out, prefix[lvl], " ")
	case FormatNone:
	}
	for i, a := range args {
		fmt.Fprint(out, a)
		if i != len(args)-1 {
			fmt.Fprint(out, " ")
		}
	}
	if lvl != input {
		fmt.Fprint(out, "\n")
	}
}
