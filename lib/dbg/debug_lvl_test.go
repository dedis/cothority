package dbg_test

import (
	"github.com/dedis/cothority/lib/dbg"
)

func init() {
	dbg.Testing = true
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
	dbg.LLvl2("LLvl2 output")
	dbg.Lvlf1("Lvlf output")
	dbg.LLvlf1("LLvlf output")

	// Output:
	// 1 : (                    dbg_test.ExampleLLvl:   0) - Lvl output
	// 1!: (                    dbg_test.ExampleLLvl:   0) - LLvl output
	// 2!: (                    dbg_test.ExampleLLvl:   0) - LLvl2 output
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
