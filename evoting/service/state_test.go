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
	s := state{log: make(map[string]*stamp)}
	s.log["0"] = &stamp{0, false, 4}

	s.schedule(time.Second)

	stamp := s.get("0")
	assert.NotNil(t, stamp)

	<-time.After(3 * time.Second)

	stamp = s.get("0")
	assert.Nil(t, stamp)
}

func TestHandle(t *testing.T) {
	s := state{log: make(map[string]*stamp)}
	s.log["0"] = &stamp{0, false, 4}
	s.log["1"] = &stamp{0, false, 5}

	s.handle()

	_, found := s.log["1"]
	assert.False(t, found)
	stamp, _ := s.log["0"]
	assert.Equal(t, 5, stamp.time)
}

func TestRegister(t *testing.T) {
	s := state{log: make(map[string]*stamp)}
	token := s.register(0, false)

	stamp := s.get("")
	assert.Nil(t, stamp)
	stamp = s.get(token)
	assert.NotNil(t, stamp)
}
