package dbg_test

import (
	"os"
	"strings"
	"testing"

	"bytes"
	"io"

	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"time"
)

func init() {
	dbg.Testing = 1
	dbg.SetUseColors(false)
}

func TestTime(t *testing.T) {
	dbg.Testing = 2
	defer func() { dbg.Testing = 1 }()
	dbg.Lvl1("No time")
	if !strings.Contains(dbg.TestStr, "1 : (") {
		t.Fatal("Didn't get correct string: ", dbg.TestStr)
	}
	dbg.SetShowTime(true)
	defer func() { dbg.SetShowTime(false) }()
	dbg.Lvl1("With time")
	if strings.Contains(dbg.TestStr, "1 : (") {
		t.Fatal("Didn't get correct string: ", dbg.TestStr)
	}
	if strings.Contains(dbg.TestStr, " +") {
		t.Fatal("Didn't get correct string: ", dbg.TestStr)
	}
	if !strings.Contains(dbg.TestStr, "With time") {
		t.Fatal("Didn't get correct string: ", dbg.TestStr)
	}
}

func TestFlags(t *testing.T) {
	test := dbg.Testing
	dbg.Testing = 2
	lvl := dbg.DebugVisible()
	time := dbg.ShowTime()
	color := dbg.UseColors()

	os.Setenv("DEBUG_LVL", "")
	os.Setenv("DEBUG_TIME", "")
	os.Setenv("DEBUG_COLOR", "")
	dbg.ParseEnv()
	if dbg.DebugVisible() != 1 {
		t.Fatal("Debugvisible should be 1")
	}
	if dbg.ShowTime() {
		t.Fatal("ShowTime should be false")
	}
	if dbg.UseColors() {
		t.Fatal("UseColors should be true")
	}

	os.Setenv("DEBUG_LVL", "3")
	os.Setenv("DEBUG_TIME", "true")
	os.Setenv("DEBUG_COLOR", "false")
	dbg.ParseEnv()
	if dbg.DebugVisible() != 3 {
		t.Fatal("DebugVisible should be 3")
	}
	if !dbg.ShowTime() {
		t.Fatal("ShowTime should be true")
	}
	if dbg.UseColors() {
		t.Fatal("UseColors should be false")
	}

	os.Setenv("DEBUG_LVL", "")
	os.Setenv("DEBUG_TIME", "")
	os.Setenv("DEBUG_COLOR", "")
	dbg.SetDebugVisible(lvl)
	dbg.SetShowTime(time)
	dbg.SetUseColors(color)
	dbg.Testing = test
}

func TestOutput(t *testing.T) {
	dbg.ErrFatal(checkOutput(func() {
		dbg.Lvl1("Testing stdout")
	}, true, false))
	dbg.ErrFatal(checkOutput(func() {
		dbg.LLvl1("Testing stdout")
	}, true, false))
	dbg.ErrFatal(checkOutput(func() {
		dbg.Print("Testing stdout")
	}, true, false))
	dbg.ErrFatal(checkOutput(func() {
		dbg.Warn("Testing stdout")
	}, false, true))
	dbg.ErrFatal(checkOutput(func() {
		dbg.Error("Testing errout")
	}, false, true))
}

func checkOutput(f func(), wantsStd, wantsErr bool) error {
	oldStd := os.Stdout
	oldErr := os.Stderr
	rStd, wStd, err := os.Pipe()
	dbg.ErrFatal(err)
	rErr, wErr, err := os.Pipe()
	dbg.ErrFatal(err)
	os.Stdout = wStd
	os.Stderr = wErr

	chanStd := make(chan string, 1024)
	go func() {
		var buf bytes.Buffer
		n, err := io.Copy(&buf, rStd)
		if n == 0 || err != nil {
			return
		}
		chanStd <- buf.String()
	}()
	chanErr := make(chan string, 1024)
	go func() {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, rErr)
		if err != nil {
			return
		}
		chanErr <- buf.String()
	}()

	f()
	// Flush buffers
	wStd.Close()
	wErr.Close()
	time.Sleep(time.Millisecond * 100)
	stdStr := ""
	for len(chanStd) > 0 {
		stdStr += <-chanStd
	}
	errStr := ""
	for len(chanErr) > 0 {
		errStr += <-chanErr
	}
	// back to normal state
	os.Stdout = oldStd
	os.Stderr = oldErr
	if wantsStd {
		if len(stdStr) == 0 {
			return errors.New("Stdout was empty")
		}
	} else {
		if len(stdStr) > 0 {
			return errors.New("Stdout was full")
		}
	}
	if wantsErr {
		if len(errStr) == 0 {
			return errors.New("Stderr was empty")
		}
	} else {
		if len(errStr) > 0 {
			return errors.New("Stderr was full")
		}
	}
	return err
}

func ExampleLevel2() {
	dbg.SetDebugVisible(2)
	dbg.Lvl1("Level1")
	dbg.Lvl2("Level2")
	dbg.Lvl3("Level3")
	dbg.Lvl4("Level4")
	dbg.Lvl5("Level5")

	// Output:
	// 1 : (                  dbg_test.ExampleLevel2:   0) - Level1
	// 2 : (                  dbg_test.ExampleLevel2:   0) - Level2
}

func ExampleMultiParams() {
	dbg.Lvl1("Multiple", "parameters")

	// Output:
	// 1 : (             dbg_test.ExampleMultiParams:   0) - Multiple parameters
}

func ExampleLLvl() {
	dbg.Lvl1("Lvl output")
	dbg.LLvl1("LLvl output")
	dbg.Lvlf1("Lvlf output")
	dbg.LLvlf1("LLvlf output")

	// Output:
	// 1 : (                    dbg_test.ExampleLLvl:   0) - Lvl output
	// 1!: (                    dbg_test.ExampleLLvl:   0) - LLvl output
	// 1 : (                    dbg_test.ExampleLLvl:   0) - Lvlf output
	// 1!: (                    dbg_test.ExampleLLvl:   0) - LLvlf output
}

func thisIsAVeryLongFunctionNameThatWillOverflow() {
	dbg.Lvl1("Overflow")
}

func ExampleLongFunctions() {
	dbg.Lvl1("Before")
	thisIsAVeryLongFunctionNameThatWillOverflow()
	dbg.Lvl1("After")

	// Output:
	// 1 : (           dbg_test.ExampleLongFunctions:   0) - Before
	// 1 : (dbg_test.thisIsAVeryLongFunctionNameThatWillOverflow:   0) - Overflow
	// 1 : (                       dbg_test.ExampleLongFunctions:   0) - After
}

func ExampleLongFunctionsLimit() {
	dbg.NamePadding = -1
	dbg.Lvl1("Before")
	thisIsAVeryLongFunctionNameThatWillOverflow()
	dbg.Lvl1("After")

	// Output:
	// 1 : (dbg_test.ExampleLongFunctionsLimit:   0) - Before
	// 1 : (dbg_test.thisIsAVeryLongFunctionNameThatWillOverflow:   0) - Overflow
	// 1 : (dbg_test.ExampleLongFunctionsLimit:   0) - After
}
