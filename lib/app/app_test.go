package app_test

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/dbg"
	"io/ioutil"
	"testing"
)

var testFileApp = `Machines = 8
Debug = 1`
var testFileDeter = `Machines = 5`

func TestReadConfig(t *testing.T) {
	conf := app.ConfigColl{}

	dbg.SetDebugVisible(5)

	writeFile("/tmp/app.toml", testFileApp)
	writeFile("/tmp/deter.toml", testFileDeter)

	app.ReadConfig(&conf, "/tmp")

}

func writeFile(name string, content string) {
	err := ioutil.WriteFile(name, []byte(content), 0666)
	if err != nil {
		dbg.Fatal("Couldn't create file:", err)
	}
}
