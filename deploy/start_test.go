package main
import (
	"io/ioutil"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
)

var testfile = `Machines = 8
Args = "Hpn Bf Rate Rounds Failures App"
Runs = "2 8 30 20 0 sign"`

func ReadRunfileTest() {
	tmpfile := "/tmp/testrun.toml"
	err := ioutil.WriteFile(tmpfile, []byte(testfile), 0666)
	if err != nil {
		dbg.Fatal("Couldn't create file:", err)
	}

	tests := ReadRunfile( tmpfile )
	if tests[0].Hpn() != 3{
		dbg.Fatal("HPN should be 2")
	}
}