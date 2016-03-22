package cli

import (
	"fmt"
	"strings"
	"testing"
)

// sample toml with two participants in toml:
var testToml = `Description = "test only toml"
[[servers]]
  Address = "192.168.210.8:7770"
  PubKey = "5ThA/lW6WgZNtb+WY1HnoxHWgZlR4dFy/AFNJ5jgmU4="
  Description = "EPFL's test server #1"
[[servers]]
  Adress = "192.168.210.9:7771"
  PubKey = "ECpQAgvJhn/mN9QWCG2WLMBd9OEKIp0FtNvZyh++NQ4="
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
		t.Fatal(fmt.Sprintf("Wanted %s number of entities, but got %s",
			want, got))
	}
	if el.List[0].Id == el.List[1].Id {
		t.Fatal("To different entities have the same ID")
	}
	wantKey := "5ThA/lW6WgZNtb+WY1HnoxHWgZlR4dFy/AFNJ5jgmU4="
	gotKey := el.List[0].Public.String()
	if wantKey != gotKey {
		t.Fatal(fmt.Sprintf("First entity's public key %s  doesn't "+
			"match with input: %s", gotKey, wantKey))
	}
}
