package debug_lvl_test

import (
	dbg "github.com/ineiti/cothorities/helpers/debug_lvl"
)

func init(){
	dbg.Testing = true
}

func ExampleLevel2() {
	dbg.Lvl1("Level1")
	dbg.Lvl2("Level2")
	dbg.Lvl3("Level3")
	dbg.Lvl4("Level4")
	dbg.Lvl5("Level5")

	// Output:
	// 1: (debug_lvl_test.go: 12) - Level1
	// 2: (debug_lvl_test.go: 13) - Level2
}
