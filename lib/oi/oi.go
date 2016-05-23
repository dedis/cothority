package oi

import (
	"fmt"
	"os"
)

var Format = FormatPython

const (
	FormatPython = iota
	FormatNone
)

const (
	info = iota
	warn
	err
	fatal
	input
)

func Info(args ...interface{}) {
	print(info, args...)
}

func Infof(f string, args ...interface{}) {
	Info(fmt.Sprintf(f, args...))
}

func Warn(args ...interface{}) {
	print(warn, args...)
}

func Warnf(f string, args ...interface{}) {
	Warn(fmt.Sprintf(f, args...))
}

func Error(args ...interface{}) {
	print(err, args...)
}

func Errorf(f string, args ...interface{}) {
	Error(fmt.Sprintf(f, args...))
}

func Fatal(args ...interface{}) {
	print(fatal, args...)
	os.Exit(1)
}

func Fatalf(f string, args ...interface{}) {
	Fatal(fmt.Sprintf(f, args...))
}

func ErrFatal(err error, args ...interface{}) {
	if err != nil {
		Fatal(err.Error(), "\n", fmt.Sprint(args...))
	}
}

func ErrFatalf(err error, f string, args ...interface{}) {
	ErrFatal(err, fmt.Sprintf(f, args...))
}

func Input(args ...interface{}) string {
	print(input, args...)
	return "no input yet"
}

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
