package main
import (
	"io/ioutil"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
"testing"
	"fmt"
)

var testfile = `Machines = 8
App = "sign"

Hpn, Bf, Rate, Rounds, Failures
2, 8, 30, 20, 0
4, 8, 30, 20, 0`

func TestReadRunfile(t *testing.T) {
	dbg.DebugVisible = 5

	tmpfile := "/tmp/testrun.toml"
	err := ioutil.WriteFile(tmpfile, []byte(testfile), 0666)
	if err != nil {
		dbg.Fatal("Couldn't create file:", err)
	}

	tests := ReadRunfile( tmpfile )
	dbg.Lvl1(deter)
	fmt.Printf("%+v\n", tests[0])
	if deter.App != "sign"{
		dbg.Fatal("App should be 'sign'")
	}
	if len(tests) != 2{
		dbg.Fatal("There should be 2 tests")
	}
}