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

	listener := NewLocalListener()

	var ready = make(chan bool)
	var incomingConn = make(chan bool)
	var outgoingConn = make(chan bool)
	go func() {
		ready <- true
		listener.Listen(addr1, func(c Conn) {
			incomingConn <- true
			nm, err := c.Receive(context.TODO())
			assert.Nil(t, err)
			assert.Equal(t, 3, nm.Msg.(SimpleMessage).I)
			// acknoledge the message
			incomingConn <- true
			err = c.Send(context.TODO(), &SimpleMessage{3})
			assert.Nil(t, err)
			//wait ack
			<-outgoingConn
			// close connection
			assert.Nil(t, c.Close())
		})
		ready <- true
	}()
	<-ready

	outgoing, err := NewLocalConn(addr2, addr1)
	assert.Nil(t, err)

	// check if connection is opened on the listener
	<-incomingConn
	// send stg and wait for ack
	assert.Nil(t, outgoing.Send(context.TODO(), &SimpleMessage{3}))
	<-incomingConn

	// receive stg and send ack
	log.Print("Waiting outgoing.Receive()")
	nm, err := outgoing.Receive(context.TODO())
	assert.Nil(t, err)
	assert.Equal(t, 3, nm.Msg.(SimpleMessage).I)
	outgoingConn <- true

	// close the incoming conn, so Receive here should return an error
	nm, err = outgoing.Receive(context.TODO())
	if err != ErrClosed {
		t.Error("Receive should have returned an error")
	}

	// close the listener
	listener.Stop()
	<-ready
}
