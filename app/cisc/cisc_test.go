package main

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/services/identity"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestGetKeys(t *testing.T) {
	clientApp := setupCA()
	res := clientApp.kvGetKeys()
	assert.Equal(t, []string{"ssh", "web"}, res)
	res = clientApp.kvGetKeys("web")
	assert.Equal(t, []string{"one", "two"}, res)
	res = clientApp.kvGetKeys("ssh", "mbp")
	assert.Equal(t, []string{"dl", "gh"}, res)
}

func TestSortUniq(t *testing.T) {
	slice := []string{"one", "two", "one"}
	assert.Equal(t, []string{"one", "two"}, sortUniq(slice))
}

func TestKvGetIntKeys(t *testing.T) {
	clientApp := setupCA()
	s1, s2 := "ssh", "gh"
	assert.Equal(t, []string{"mba", "mbp"}, clientApp.kvGetIntKeys(s1, s2))
	assert.Equal(t, "ssh", s1)
	assert.Equal(t, "gh", s2)
}

func setupCA() *CA {
	return &CA{&identity.Identity{
		Config: &identity.Config{
			Data: map[string]string{
				"web:one":     "1",
				"web:one:one": "2",
				"web:two":     "3",
				"ssh:mbp:gh":  "4",
				"ssh:mbp:dl":  "5",
				"ssh:mba:gh":  "6",
			},
		},
	}}
}
