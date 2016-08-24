package network

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

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
	nm, err := outgoing.Receive(context.TODO())
	assert.Nil(t, err)
	assert.Equal(t, 3, nm.Msg.(SimpleMessage).I)
	outgoingConn <- true

	// close the incoming conn, so Receive here should return an error
	nm, err = outgoing.Receive(context.TODO())
	if err != ErrClosed {
		t.Error("Receive should have returned an error")
	}
	assert.Nil(t, outgoing.Close())

	// close the listener
	assert.Nil(t, listener.Stop())
	<-ready
}

func TestLocalManyConn(t *testing.T) {
	nbrConn := 3
	addr := "127.0.0.1:2000"
	listener := NewLocalListener()
	var wg sync.WaitGroup
	go func() {
		listener.Listen(addr, func(c Conn) {
			_, err := c.Receive(context.TODO())
			assert.Nil(t, err)

			assert.Nil(t, c.Send(context.TODO(), &SimpleMessage{3}))
		})
	}()

	if !waitListeningUp(addr) {
		t.Fatal("Can't get listener up")
	}
	wg.Add(nbrConn)
	for i := 1; i <= nbrConn; i++ {
		go func(j int) {
			a := "127.0.0.1:" + strconv.Itoa(2000+j)
			c, err := NewLocalConn(a, addr)
			if err != nil {
				t.Fatal(err)
			}
			assert.Nil(t, c.Send(context.TODO(), &SimpleMessage{3}))
			nm, err := c.Receive(context.TODO())
			assert.Nil(t, err)
			assert.Equal(t, 3, nm.Msg.(SimpleMessage).I)
			assert.Nil(t, c.Close())
			wg.Done()
		}(i)
	}

	wg.Wait()
	listener.Stop()
}

func waitListeningUp(addr string) bool {
	for i := 0; i < 5; i++ {
		if localConnStore.IsListening(addr) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
