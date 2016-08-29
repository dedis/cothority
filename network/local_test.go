package network

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLocalConnDiffAddress(t *testing.T) {
	testLocalConn(t, NewLocalAddress("127.0.0.1:2000"), NewLocalAddress("127.0.0.1:2001"))
}

func TestLocalConnSameAddress(t *testing.T) {
	testLocalConn(t, NewLocalAddress("127.0.0.1:2000"), NewLocalAddress("127.0.0.1:2000"))
}

func testLocalConn(t *testing.T, a1, a2 Address) {
	addr1 := a1
	addr2 := a2

	listener, err := NewLocalListener(addr1)
	if err != nil {
		t.Fatal("Could not listen", err)
	}

	var ready = make(chan bool)
	var incomingConn = make(chan bool)
	var outgoingConn = make(chan bool)
	go func() {
		ready <- true
		listener.Listen(func(c Conn) {
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
	if err != nil {
		listener.Stop()
		<-ready
		if addr1 == addr2 {
			return // all is good as we should not be able to connect
		}
		t.Fatal("erro NewLocalConn:", err)
	}

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
	addr := NewLocalAddress("127.0.0.1:2000")
	listener, err := NewLocalListener(addr)
	if err != nil {
		t.Fatal("Could not setup listener:", err)
	}
	var wg sync.WaitGroup
	go func() {
		listener.Listen(func(c Conn) {
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
			a := NewLocalAddress("127.0.0.1:" + strconv.Itoa(2000+j))
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

func waitListeningUp(addr Address) bool {
	for i := 0; i < 5; i++ {
		if localConnStore.IsListening(addr) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func NewTestLocalHost(port int) (*LocalHost, error) {
	addr := NewLocalAddress("127.0.0.1:" + strconv.Itoa(port))
	return NewLocalHost(addr)
}
