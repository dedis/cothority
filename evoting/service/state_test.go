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
	s := state{make(map[string]*stamp)}
	s.log["u"] = &stamp{0, false, 4}

	stop := s.schedule(time.Second)
	assert.Equal(t, 1, len(s.log))
	<-time.After(2500 * time.Millisecond)
	assert.Equal(t, 0, len(s.log))
	stop <- true
	<-time.After(500 * time.Millisecond)
}

func TestRegister(t *testing.T) {
	s := state{make(map[string]*stamp)}
	t1 := s.register(123, true)
	t2 := s.register(456, false)

	assert.NotEqual(t, t1, t2)
	assert.Equal(t, 2, len(s.log))
}
