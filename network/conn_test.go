package network

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// test the creation of a new conn by opening a golang
// listener and making a TCPConn connect to it,then close it.
func TestTCPConn(t *testing.T) {
	addr := make(chan string)
	done := make(chan bool)
	go func() {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		assert.Nil(t, err)
		addr <- ln.Addr().String()
		_, err = ln.Accept()
		assert.Nil(t, err)
		// wait until it can be closed
		<-done
		assert.Nil(t, ln.Close())
		done <- true
	}()

	// get addr
	listeningAddr := <-addr
	c, err := NewTCPConn(listeningAddr)
	assert.Nil(t, err)
	assert.Nil(t, c.Close())
	// tell the listener to close
	done <- true
	// wait until it is closed
	<-done
}

func TestTCPConnWithListener(t *testing.T) {
	ln := NewTCPListener()
	ready := make(chan bool)
	stop := make(chan bool)
	connStat := make(chan uint64)

	addr := Address("tcp://127.0.0.1:5678")
	assert.True(t, addr.Valid())
	connFn := func(c Conn) {
		connStat <- c.Rx()
		c.Receive(context.TODO())
		connStat <- c.Rx()
	}
	go func() {
		ready <- true
		err := ln.Listen(addr, connFn)
		assert.Nil(t, err, "Listener stop incorrectly")
		stop <- true
	}()

	<-ready
	c, err := NewTCPConn(addr.NetworkAddress())
	assert.Nil(t, err, "Could not open connection")
	// Test bandwitdth measurements also
	rx1 := <-connStat
	tx1 := c.Tx()
	assert.Nil(t, c.Send(context.TODO(), &SimpleMessage{3}))
	tx2 := c.Tx()
	rx2 := <-connStat

	if (tx2 - tx1) != (rx2 - rx1) {
		t.Error("Connections did see same bytes? %d tx vs %d rx", (tx2 - tx1), (rx2 - rx1))
	}

	assert.Nil(t, ln.Stop(), "Error stopping listener")
	select {
	case <-stop:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Could not stop listener")

	}
}
