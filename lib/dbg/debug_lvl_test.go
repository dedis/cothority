package dbg_test

import (
	"os"
	"strings"
	"testing"

	"github.com/dedis/cothority/lib/dbg"
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
