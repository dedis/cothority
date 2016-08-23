package network

import (
	"context"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocalConn(t *testing.T) {
	addr1 := "127.0.0.1:2000"
	addr2 := "127.0.0.1:2001"
	c1 := NewLocalConn(addr1, addr2)
	c2 := NewLocalConn(addr2, addr1)
	assert.Equal(t, 2, localConnStore.Len())

	assert.Nil(t, c1.Send(context.TODO(), &SimpleMessage{3}))
	nm, err := c2.Receive(context.TODO())
	log.Print("Received message")
	assert.Nil(t, err)
	sm, ok := nm.Msg.(SimpleMessage)
	assert.True(t, ok)
	assert.Equal(t, 3, sm.I)

	assert.Nil(t, c1.Close())

	if c2.Send(context.TODO(), &SimpleMessage{3}) == nil {
		t.Error("Sending on a closed channel connection should have failed")
	}
	assert.Nil(t, c2.Close())
}
