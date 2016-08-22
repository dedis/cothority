package network

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// will create a TCPListener & open a golang net.TCPConn to it
func TestTCPListener(t *testing.T) {
	ln := NewTCPListener()
	ready := make(chan bool)
	stop := make(chan bool)
	connReceived := make(chan bool)

	addr := Address("tcp://127.0.0.1:5678")
	assert.True(t, addr.Valid())
	assert.Equal(t, "127.0.0.1:5678", addr.NetworkAddress())
	connFn := func(c Conn) {
		connReceived <- true
		c.Close()
	}
	go func() {
		ready <- true
		err := ln.Listen(addr, connFn)
		assert.Nil(t, err, "Listener stop incorrectly")
		stop <- true
	}()

	<-ready
	_, err := net.Dial("tcp", addr.NetworkAddress())
	assert.Nil(t, err, "Could not open connection")
	<-connReceived
	assert.Nil(t, ln.Stop(), "Error stopping listener")
	select {
	case <-stop:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Could not stop listener")
	}
}
