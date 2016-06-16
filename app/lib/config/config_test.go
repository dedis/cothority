package config

import (
	"strings"
	"testing"

	"github.com/dedis/cothority/lib/dbg"
)

func TestMain(m *testing.M) {
	dbg.MainTest(m)
}

var serverGroup string = `Description = "Default Dedis Cosi servers"

[[servers]]
Addresses = ["5.135.161.91:2000"]
Public = "lLglU3nhHfUWe4p647hffn618TiUq+6FvTGzJw8eTGU="
Description = "Nikkolasg's server: spreading the love of signing"

[[servers]]
Addresses = ["185.26.156.40:61117"]
Public = "apIWOKSt6JcOvNnjcVcPCNcaJJh/kPEjkbn2xSW+W+Q="
Description = "Ismail's server"`

func TestReadGroupDescToml(t *testing.T) {
	group, err := ReadGroupDescToml(strings.NewReader(serverGroup))
	dbg.ErrFatal(err)

	if len(group.Roster.List) != 2 {
		t.Fatal("Should have 2 Entities")
	}
	if len(group.description) != 2 {
		t.Fatal("Should have 2 descriptions")
	}
	if group.description[group.Roster.List[1]] != "Ismail's server" {
		t.Fatal("This should be Ismail's server")
	}
}
