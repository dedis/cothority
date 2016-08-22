package network

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// Listener is responsible for listening for incoming Conn on a particular
// address.It can only accept one type of incoming Conn.
type Listener interface {
	// Listen will start listening for incoming connections on the given
	// address. Each time there is an incoming Conn, it will call the given
	// function in a go routine with the incoming Conn as parameter.
	// The call is BLOCKING.
	Listen(Address, func(Conn)) error
	// Stop will stop the listening. It is a blocking call,i.e. it returns
	// when the Listener really has stopped listening.
	Stop() error
	// Type returns which type of connections does this listener accept as
	// incoming connection.
	IncomingType() ConnType
}

// TCPListener is the underlying implementation of
// Host using Tcp as a communication channel
type TCPListener struct {
	// the underlying golang/net listener
	listener net.Listener
	// the close channel used to indicate to the listener we want to quit
	quit chan bool
	// quitListener is a channel to indicate to the closing function that the
	// listener has actually really quit
	quitListener  chan bool
	listeningLock sync.Mutex
	listening     bool

	connType ConnType
}

// NewTCPLIstener returns a Listener that listens on a TCP port
func NewTCPListener() *TCPListener {
	return &TCPListener{
		quit:         make(chan bool),
		quitListener: make(chan bool),
		connType:     PlainTCP,
	}
}

// Listen implements the Listener interface
func (t *TCPListener) Listen(addr Address, fn func(Conn)) error {
	if addr.ConnType() != t.connType {
		return fmt.Errorf("Wrong ConnType: %s (actual) vs %s (expected)", addr.ConnType(), t.connType)
	}
	receiver := func(tc *TCPConn) {
		go fn(tc)
	}
	return t.listen(addr.NetworkAddress(), receiver)
}

// listen is the private function that takes a function that takes a TCPConn.
// That way we can control what to do of the TCPConn before returning it to the
// function given by the user. fn is called in the same routine.
func (t *TCPListener) listen(addr string, fn func(*TCPConn)) error {
	t.listeningLock.Lock()
	t.listening = true
	global, _ := GlobalBind(addr)
	for i := 0; i < MaxRetry; i++ {
		ln, err := net.Listen("tcp", global)
		if err == nil {
			t.listener = ln
			break
		} else if i == MaxRetry-1 {
			t.listeningLock.Unlock()
			return errors.New("Error opening listener: " + err.Error())
		}
		time.Sleep(WaitRetry)
	}

	t.listeningLock.Unlock()
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.quit:
				t.quitListener <- true
				return nil
			default:
			}
			continue
		}
		fmt.Println("Works!", conn)
		c := TCPConn{
			endpoint: conn.RemoteAddr().String(),
			conn:     conn,
		}
		fn(&c)
	}
}

// Stop will stop the listener. It is a blocking call.
func (t *TCPListener) Stop() error {
	// lets see if we launched a listening routing
	var listening bool
	t.listeningLock.Lock()
	listening = t.listening
	defer t.listeningLock.Unlock()
	// we are NOT listening
	if !listening {
		return nil
	}

	close(t.quit)

	var stop bool
	for !stop {
		if t.listener != nil {
			if err := t.listener.Close(); err != nil {
				return err
			}
		}
		select {
		case <-t.quitListener:
			stop = true
		case <-time.After(time.Millisecond * 50):
			continue
		}
	}
	return nil
}

func (t *TCPListener) IncomingType() ConnType {
	return t.connType
}
