package oi

import (
	"fmt"
	"os"
)

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

// Input prints the arguments given with a 'input'-format
func Input(args ...interface{}) string {
	print(input, args...)
	return "no input yet"
}

// Inputf takes a format and calls Input
func Inputf(f string, args ...interface{}) string {
	return Input(fmt.Sprintf(f, args...))
}

func print(lvl int, args ...interface{}) {
	switch Format {
	case FormatPython:
		prefix := []string{"[+]", "[-]", "[!]", "[X]", "[?]"}
		fmt.Print(prefix[lvl], " ")
	case FormatNone:
	}
	fmt.Print(args...)
	if lvl != input {
		fmt.Print("\n")
	}
}
