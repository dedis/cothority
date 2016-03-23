package cli

import (
	"bytes"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/network"
	"strings"
	"testing"
)

// sample toml with two nodes:
var testToml = `Description = "test only toml"
[[servers]]
  Addresses = ["192.168.210.8:7770"]
  Public = "5ThA/lW6WgZNtb+WY1HnoxHWgZlR4dFy/AFNJ5jgmU4="
  Description = "EPFL's test server #1"
[[servers]]
  Addresses = ["192.168.210.9:7771"]
  Public = "ECpQAgvJhn/mN9QWCG2WLMBd9OEKIp0FtNvZyh++NQ4="
  Description = "EPFL's test server #2"
`

func TestCreateEntityList(t *testing.T) {
	el, err := ReadGroupToml(strings.NewReader(testToml))
	if err != nil {
		t.Fatal("Failed to read group toml.", err)
	}
	if el == nil {
		t.Fatal("Didn't parse entity list")
	}
	want := 2
	got := len(el.List)
	if got != want {
		t.Fatalf("Wanted %s number of entities, but got %s",
			want, got)
	}
	if el.List[0].Id == el.List[1].Id {
		t.Fatal("To different entities have the same ID")
	}
	wantKey := "5ThA/lW6WgZNtb+WY1HnoxHWgZlR4dFy/AFNJ5jgmU4="
	var buff bytes.Buffer
	if err := cliutils.WritePub64(network.Suite, &buff, el.List[0].Public); err != nil {
		t.Fatal("Could not convert key to base64", err)
	}
	gotKey := buff.String()
	if wantKey != gotKey {
		t.Fatalf("First entity's public key %s  doesn't "+
			"match with input: %s", gotKey, wantKey)
	}
}

func TestSignStatement(t *testing.T) {
	// See TestHackyCosi in sda_test
	// the *only* difference here would be that the entity list is created
	// from a toml file instead (this part is tested above)
}
