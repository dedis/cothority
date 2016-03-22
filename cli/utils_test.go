package cli

import (
	"strings"
	"testing"
)

// sample toml with two participants in toml:
var testToml = `title = "test-toml.toml"
[[servers]]
  Address = "192.168.210.8:7770"
  PubKey = "5ThA/lW6WgZNtb+WY1HnoxHWgZlR4dFy/AFNJ5jgmU4="
[[servers]]
  Adress = "192.168.210.9:7771"
  PubKey = "ECpQAgvJhn/mN9QWCG2WLMBd9OEKIp0FtNvZyh++NQ4="
`

func TestCreateEntityList(t *testing.T) {
	el, err := ReadGroupToml(strings.NewReader(testToml))
	if err != nil {
		t.Fatal("Failed to read group toml.", err)
	}
	want := 2
	got := len(el.List)
	if got != want {
		t.Fatal("Wanted " + want + " number of entities," +
			"but got " + got)
	}
	if el.List[0].Id == el.List[1].Id {
		t.Fatal("To different entities have the same ID")
	}
	want = "5ThA/lW6WgZNtb+WY1HnoxHWgZlR4dFy/AFNJ5jgmU4="
	got = el.List[0].Public.String()
	if want != got {
		t.Fatal("First entity's public key" + got +
			"doesn't match with the one from the input file" + want)
	}
}
