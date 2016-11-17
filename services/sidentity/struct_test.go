package sidentity

/*
import (
	"testing"

	"github.com/stretchr/testify/assert"
)


func TestGetKeys(t *testing.T) {
	cfg := setupConfig()
	res := cfg.GetSuffixColumn()
	assert.Equal(t, []string{"ssh", "web"}, res)
	res = cfg.GetSuffixColumn("web")
	assert.Equal(t, []string{"one", "two"}, res)
	res = cfg.GetSuffixColumn("ssh", "mbp")
	assert.Equal(t, []string{"dl", "gh"}, res)
}

func TestSortUniq(t *testing.T) {
	slice := []string{"one", "two", "one"}
	assert.Equal(t, []string{"one", "two"}, sortUniq(slice))
}

func TestKvGetIntKeys(t *testing.T) {
	cfg := setupConfig()
	s1, s2 := "ssh", "gh"
	assert.Equal(t, []string{"mba", "mbp"}, cfg.GetIntermediateColumn(s1, s2))
	assert.Equal(t, "ssh", s1)
	assert.Equal(t, "gh", s2)
}

func setupConfig() *Config {
	return &Config{
		Data: map[string]string{
			"web:one":     "1",
			"web:one:one": "2",
			"web:two":     "3",
			"ssh:mbp:gh":  "4",
			"ssh:mbp:dl":  "5",
			"ssh:mba:gh":  "6",
		},
	}
}
*/
