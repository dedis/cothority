package main

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestGetKeys(t *testing.T) {
	myKVs := map[string]string{
		"web:one":     "1",
		"web:one:one": "2",
		"web:two":     "3",
		"ssh:mbp:gh":  "4",
		"ssh:mbp:dl":  "5",
		"ssh:mba:gh":  "6",
	}
	res := getKeys(myKVs)
	assert.Equal(t, []string{"ssh", "web"}, res)
	res = getKeys(myKVs, "web")
	assert.Equal(t, []string{"one", "two"}, res)
	res = getKeys(myKVs, "ssh", "mbp")
	assert.Equal(t, []string{"dl", "gh"}, res)
}
