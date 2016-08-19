package network

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// test the creation of a new conn by opening a golang
// listenern and making a TCPConn connect to it,then close it.
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
