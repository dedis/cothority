package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNonce(t *testing.T) {
	n1, n2, n3 := nonce(128), nonce(128), nonce(128)
	assert.Equal(t, 32, len(n1))
	assert.NotEqual(t, n1, n2)
	assert.NotEqual(t, n1, n3)
	assert.NotEqual(t, n2, n3)
}

func TestRegister(t *testing.T) {
	timeLimit = 2 * time.Second
	s := state{log: make(map[string]*stamp)}
	token := s.register(0, false)

	stamp := s.get("")
	assert.Nil(t, stamp)
	stamp = s.get(token)
	assert.NotNil(t, stamp)

	<-time.After(3 * time.Second)
	stamp = s.get(token)
	assert.Nil(t, stamp)
}
