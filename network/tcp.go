package network

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dedis/cothority/log"
)

// NewTCPRouter returns a fresh Router using TCPHost as the underlying Host.
func NewTCPRouter(sid *ServerIdentity) (*Router, error) {
	h, err := NewTCPHost(sid.Address)
	if err != nil {
		return nil, err
	}
	return NewRouter(sid, h), nil
}

// TCPConn is the underlying implementation of
// Conn using plain Tcp.
type TCPConn struct {
	// The name of the endpoint we are connected to.
	endpoint Address

	// The connection used
	conn net.Conn

	// closed indicator
	closed    bool
	closedMut sync.Mutex
	// So we only handle one receiving packet at a time
	receiveMutex sync.Mutex
	// So we only handle one sending packet at a time
	sendMutex sync.Mutex

	counterSafe
}

// NewTCPConn will open a TCPConn to the given address.
func NewTCPConn(addr Address) (*TCPConn, error) {
	var err error
	var conn net.Conn
	netAddr := addr.NetworkAddress()
	for i := 0; i < MaxRetryConnect; i++ {
		conn, err = net.Dial("tcp", netAddr)
		if err != nil {
			time.Sleep(WaitRetry)
		} else {
			break
		}
		time.Sleep(WaitRetry)
	}
	if conn == nil {
		return nil, fmt.Errorf("Could not connect to %s: %s", addr, err)
	}
	c := TCPConn{
		endpoint: addr,
		conn:     conn,
	}
	return &c, err
}

// Receive calls the receive routine to get the bytes from the connection then
// it tries to decode the buffer. Returns the Packet with the Msg field decoded
// or EmptyApplicationPacket and an error if something wrong occured.
func (c *TCPConn) Receive() (nm Packet, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("Error Received message: %v", err)
			nm = EmptyApplicationPacket
		}
	}()

	var am Packet
	buff, err := c.receiveRaw()
	if err != nil {
		return EmptyApplicationPacket, err
	}

	err = am.UnmarshalBinary(buff)
	if err != nil {
		return EmptyApplicationPacket, fmt.Errorf("Error unmarshaling message type %s: %s", am.MsgType.String(), err.Error())
	}
	am.From = c.Remote()
	return am, nil
}

// receiveRaw is responsible for getting first the size of the message, then the
// whole message. It returns the raw message as slice of bytes.
func (c *TCPConn) receiveRaw() ([]byte, error) {
	c.receiveMutex.Lock()
	defer c.receiveMutex.Unlock()
	// First read the size
	var total Size
	if err := binary.Read(c.conn, globalOrder, &total); err != nil {
		return nil, handleError(err)
	}
	b := make([]byte, total)
	var read Size
	var buffer bytes.Buffer
	for read < total {
		// read the size of the next packet
		n, err := c.conn.Read(b)
		// if error then quit
		if err != nil {

			return nil, handleError(err)
		}
		// put it in the longterm buffer
		if _, err := buffer.Write(b[:n]); err != nil {
			log.Error("Couldn't write to buffer:", err)
		}
		read += Size(n)
		b = b[n:]
	}

	// set the size read
	c.updateRx(uint64(read))
	return buffer.Bytes(), nil
}

// Send will convert the NetworkMessage into an ApplicationMessage
// and send it with send()
// Returns an error if anything was wrong
func (c *TCPConn) Send(obj Body) error {
	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()
	am, err := NewNetworkPacket(obj)
	if err != nil {
		return fmt.Errorf("Error converting packet: %v\n", err)
	}
	log.Lvlf5("Message SEND => %+v", am)
	var b []byte
	b, err = am.MarshalBinary()
	if err != nil {
		return fmt.Errorf("Error marshaling  message: %s", err.Error())
	}
	return c.sendRaw(b)
}

// sendRaw takes care of sending this slice of bytes FULLY to the connection
func (c *TCPConn) sendRaw(b []byte) error {
	// First write the size
	packetSize := Size(len(b))
	if err := binary.Write(c.conn, globalOrder, packetSize); err != nil {
		return err
	}
	// Then send everything through the connection
	// Send chunk by chunk
	log.Lvl5("Sending from", c.conn.LocalAddr(), "to", c.conn.RemoteAddr())
	var sent Size
	for sent < packetSize {
		n, err := c.conn.Write(b[sent:])
		if err != nil {
			return handleError(err)
		}
		sent += Size(n)
	}
	// update stats on the connection
	c.updateTx(uint64(packetSize))
	return nil
}

// Remote returns the name of the peer at the end point of
// the connection
func (c *TCPConn) Remote() Address {
	return c.endpoint
}

// Local returns the local address and port
func (c *TCPConn) Local() Address {
	return NewTCPAddress(c.conn.LocalAddr().String())
}

// Type implements the Conn interface
func (c *TCPConn) Type() ConnType {
	return PlainTCP
}

// Close ... closes the connection
func (c *TCPConn) Close() error {
	c.closedMut.Lock()
	defer c.closedMut.Unlock()
	if c.closed == true {
		return nil
	}
	err := c.conn.Close()
	c.closed = true
	if err != nil {
		return handleError(err)
	}
	return nil
}

// handleError produces the higher layer error depending on the type
// so user of the package can know what is the cause of the problem
func handleError(err error) error {

	if strings.Contains(err.Error(), "use of closed") || strings.Contains(err.Error(), "broken pipe") {
		return ErrClosed
	} else if strings.Contains(err.Error(), "canceled") {
		return ErrCanceled
	} else if err == io.EOF || strings.Contains(err.Error(), "EOF") {
		return ErrEOF
	}

	netErr, ok := err.(net.Error)
	if !ok {
		return ErrUnknown
	}
	if netErr.Temporary() {
		return ErrTemp
	} else if netErr.Timeout() {
		return ErrTimeout
	}
	return ErrUnknown
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

	// closed is used to tell the listen routine to return immediatly if a
	// Stop() have been made (concurrency into place here)
	closed bool

	// listening addr (actual). Useful for listening on :0 port
	addr net.Addr
}

// NewTCPListener returns a Listener. This function tries to bind to the given
// address already.It returns the listener and an error if one occured during
// the binding. A subsequent call to Address() will give the real listening
// address (useful if you set it to port :0).
func NewTCPListener(addr Address) (*TCPListener, error) {
	l := &TCPListener{
		quit:         make(chan bool),
		quitListener: make(chan bool),
	}
	return l, l.bind(addr)
}

func (t *TCPListener) bind(addr Address) error {
	t.listeningLock.Lock()
	defer t.listeningLock.Unlock()
	if t.listening == true {
		return errors.New("Already listening")
	}
	global, _ := GlobalBind(addr.NetworkAddress())
	for i := 0; i < MaxRetryConnect; i++ {
		ln, err := net.Listen("tcp", global)
		if err == nil {
			t.listener = ln
			break
		} else if i == MaxRetryConnect-1 {
			return errors.New("Error opening listener: " + err.Error())
		}
		time.Sleep(WaitRetry)
	}
	t.addr = t.listener.Addr()
	return nil
}

// Listen implements the Listener interface
func (t *TCPListener) Listen(fn func(Conn)) error {
	receiver := func(tc *TCPConn) {
		go fn(tc)
	}
	return t.listen(receiver)
}

// listen is the private function that takes a function that takes a TCPConn.
// That way we can control what to do of the TCPConn before returning it to the
// function given by the user. fn is called in the same routine.
func (t *TCPListener) listen(fn func(*TCPConn)) error {
	t.listeningLock.Lock()
	if t.closed == true {
		t.listeningLock.Unlock()
		return nil
	}
	t.listening = true
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
		c := TCPConn{
			endpoint: NewTCPAddress(conn.RemoteAddr().String()),
			conn:     conn,
		}
		fn(&c)
	}
}

// Stop will stop the listener. It is a blocking call.
func (t *TCPListener) Stop() error {
	// lets see if we launched a listening routing
	t.listeningLock.Lock()
	defer t.listeningLock.Unlock()

	close(t.quit)

	if t.listener != nil {
		if err := t.listener.Close(); err != nil {
			if handleError(err) != ErrClosed {
				return err
			}
		}
	}
	var stop bool
	if t.listening {
		for !stop {
			select {
			case <-t.quitListener:
				stop = true
			case <-time.After(time.Millisecond * 50):
				continue
			}
		}
	}

	t.quit = make(chan bool)
	t.listening = false
	t.closed = true
	return nil
}

// Address implements the Listener interface
func (t *TCPListener) Address() Address {
	t.listeningLock.Lock()
	defer t.listeningLock.Unlock()
	return NewAddress(PlainTCP, t.addr.String())
}

// Listening implements the Listener interface
func (t *TCPListener) Listening() bool {
	t.listeningLock.Lock()
	defer t.listeningLock.Unlock()
	return t.listening
}

// TCPHost implements the Host interface using TCP connections.
type TCPHost struct {
	addr Address
	*TCPListener
}

// NewTCPHost returns a fresh Host using TCP connection based type
func NewTCPHost(addr Address) (*TCPHost, error) {
	h := &TCPHost{
		addr: addr,
	}
	var err error
	h.TCPListener, err = NewTCPListener(addr)
	return h, err
}

// Connect implements the Host interface
func (t *TCPHost) Connect(addr Address) (Conn, error) {
	switch addr.ConnType() {
	case PlainTCP:
		c, err := NewTCPConn(addr)
		return c, err
	}
	return nil, fmt.Errorf("TCPHost %s can't handle this type of connection: %s", addr, addr.ConnType())
}

// NewTCPClient returns a fresh client using the TCP network communication
// layer.
func NewTCPClient() *Client {
	fn := func(own, remote *ServerIdentity) (Conn, error) {
		return NewTCPConn(remote.Address)
	}
	return newClient(fn)
}

// NewTCPAddress returns a new Address that has type PlainTCP with the given
// addr
func NewTCPAddress(addr string) Address {
	return NewAddress(PlainTCP, addr)
}
