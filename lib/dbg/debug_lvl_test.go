package dbg_test

import (
	"github.com/dedis/cothority/lib/dbg"
)

func init(){
	dbg.Testing = true
}

func ExampleLevel2() {
	dbg.DebugVisible = 2
	dbg.Lvl1("Level1")
	dbg.Lvl2("Level2")
	dbg.Lvl3("Level3")
	dbg.Lvl4("Level4")
	dbg.Lvl5("Level5")

	// Output:
	// 1: (            debug_lvl_test.ExampleLevel2:   0) - Level1
	// 2: (            debug_lvl_test.ExampleLevel2:   0) - Level2
}

func thisIsAVeryLongFunctionNameThatWillOverflow(){
	dbg.Lvl1("Overflow")
}

func ExampleLongFunctions() {
	dbg.Lvl1("Before")
	thisIsAVeryLongFunctionNameThatWillOverflow()
	dbg.Lvl1("After")

	// Output:
	// 1: (     debug_lvl_test.ExampleLongFunctions:   0) - Before
	// 1: (debug_lvl_test.thisIsAVeryLongFunctionNameThatWillOverflow:   0) - Overflow
	// 1: (                       debug_lvl_test.ExampleLongFunctions:   0) - After
}

func ExampleLongFunctionsLimit() {
	dbg.NamePadding = -1
	dbg.Lvl1("Before")
	thisIsAVeryLongFunctionNameThatWillOverflow()
	dbg.Lvl1("After")

	// Output:
	// 1: (debug_lvl_test.ExampleLongFunctionsLimit:   0) - Before
	// 1: (debug_lvl_test.thisIsAVeryLongFunctionNameThatWillOverflow:   0) - Overflow
	// 1: (debug_lvl_test.ExampleLongFunctionsLimit:   0) - After
}
