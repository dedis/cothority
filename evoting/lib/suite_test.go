package lib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomKeyPair(t *testing.T) {
	x1, X1 := RandomKeyPair()
	x2, X2 := RandomKeyPair()
	assert.NotEqual(t, x1, x2)
	assert.NotEqual(t, X1, X2)
}
