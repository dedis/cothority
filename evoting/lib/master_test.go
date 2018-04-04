package lib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAdmin(t *testing.T) {
	m := &Master{Admins: []uint32{0}}
	assert.True(t, m.IsAdmin(0))
	assert.False(t, m.IsAdmin(1))
}
