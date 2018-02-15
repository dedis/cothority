package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNonce(t *testing.T) {
	n1, n2, n3 := nonce(10), nonce(10), nonce(10)
	assert.Equal(t, 10, len(n1), len(n2), len(n3))
	assert.NotEqual(t, n1, n2, n3)
}

func TestSchedule(t *testing.T) {
	s := state{}
	s.log.Store("u", &stamp{0, false, 4})

	stop := s.schedule(time.Second)

	_, found := s.log.Load("u")
	assert.True(t, found)

	<-time.After(3 * time.Second)

	_, found = s.log.Load("u")
	assert.False(t, found)

	stop <- true
}

func TestRegister(t *testing.T) {
	s := state{}
	t1 := s.register(123, true)
	t2 := s.register(456, false)

	assert.NotEqual(t, t1, t2)

	_, found := s.log.Load(t1)
	assert.True(t, found)
	_, found = s.log.Load(t2)
	assert.True(t, found)
}
