package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounterSafe(t *testing.T) {
	cs := counterSafe{}
	assert.Equal(t, uint64(0), cs.Rx())
	assert.Equal(t, uint64(0), cs.Tx())

	cs.updateRx(14)
	assert.Equal(t, uint64(14), cs.Rx())

	cs.updateTx(16)
	assert.Equal(t, uint64(16), cs.Tx())
}
