package platform_test

import (
	"io/ioutil"
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/simul/platform"
)

var testfile = `Machines = 8
App = "sign"

Ppm, Rounds
2, 30
4, 30`

func TestReadRunfile(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 2)
	tplat := &TPlat{}

	tmpfile := "/tmp/testrun.toml"
	err := ioutil.WriteFile(tmpfile, []byte(testfile), 0666)
	if err != nil {
		dbg.Fatal("Couldn't create file:", err)
	}

	tests := platform.ReadRunFile(tplat, tmpfile)
	dbg.Lvl2(tplat)
	dbg.Lvlf2("%+v\n", tests[0])
	if tplat.App != "sign" {
		dbg.Fatal("App should be 'sign'")
	}
	if len(tests) != 2 {
		dbg.Fatal("There should be 2 tests")
	}
	if tests[0].Get("machines") != "8" {
		dbg.Fatal("Machines = 8 has not been copied into RunConfig")
	}
}

type TPlat struct {
	App      string
	Machines int
}

func (t *TPlat) Configure(pc *platform.Config)       {}
func (t *TPlat) Build(s string, arg ...string) error { return nil }
func (t *TPlat) Deploy(rc platform.RunConfig) error  { return nil }
func (t *TPlat) Start(...string) error               { return nil }
func (t *TPlat) Stop() error                         { return nil }
func (t *TPlat) Cleanup() error                      { return nil }
func (t *TPlat) Wait() error                         { return nil }
