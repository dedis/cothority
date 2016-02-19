// This package is a networking library. You have Hosts which can
// issue connections to others hosts, and Conn which are the connections itself.
// Hosts and Conns are interfaces and can be of type Tcp, or Chans, or Udp or
// whatever protocols you think might implement this interface.
// In this library we also provide a way to encode / decode any kind of packet /
// structs. When you want to send a struct to a conn, you first register
// (one-time operation) this packet to the library, and then directly pass the
// struct itself to the conn that will recognize its type. When decoding,
// it will automatically detect the underlying type of struct given, and decode
// it accordingly. You can provide your own decode / encode methods if for
// example, you have a variable length packet structure. For this, just
// implements MarshalBinary or UnmarshalBinary.

package network

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/net/context"

	"errors"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
	"runtime/debug"
)

// Network part //

// NewTcpHost returns a Fresh TCP Host
// If constructors == nil, it will take an empty one.
func NewTcpHost() *TcpHost {
	return &TcpHost{
		peers:         make(map[string]Conn),
		quit:          make(chan bool),
		constructors:  DefaultConstructors(Suite),
		quitListener:  make(chan bool),
		listeningLock: &sync.Mutex{},
		closedLock:    &sync.Mutex{},
	}
}

// Open will create a new connection between this host
// and the remote host named "name". This is a TcpConn.
// If anything went wrong, Conn will be nil.
func (t *TcpHost) Open(name string) (Conn, error) {
	c, err := t.openTcpConn(name)
	if err != nil {
		return nil, err
	}
	t.peers[name] = c
	return c, nil
}

// Listen for any host trying to contact him.
// Will launch in a goroutine the srv function once a connection is established
func (t *TcpHost) Listen(addr string, fn func(Conn)) error {
	receiver := func(tc *TcpConn) {
		go fn(tc)
	}
	return t.listen(addr, receiver)
}

// Close will close every connection this host has opened
func (t *TcpHost) Close() error {
	for _, c := range t.peers {
		dbg.Lvl4("Closing peer", c)
		if err := c.Close(); err != nil {
			dbg.Error("Error while closing peers")
			return handleError(err)
		}
	}
	if !t.closed {
		close(t.quit)
	}
	t.closed = true
	// lets see if we launched a listening routing
	var listening bool
	t.listeningLock.Lock()
	listening = t.listening
	t.listeningLock.Unlock()
	// we are NOT listening
	if !listening {
		//	fmt.Println("tcphost.Close() without listening")
		return nil
	}
	var stop bool
	for !stop {
		if t.listener != nil {
			t.listener.Close()
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

// Remote returns the name of the peer at the end point of
// the connection
func (c *TcpConn) Remote() string {
	return c.Endpoint
}

// Receive waits for any input on the connection and returns
// the ApplicationMessage **decoded** and an error if something
// wrong occured
func (c *TcpConn) Receive(ctx context.Context) (NetworkMessage, error) {
	var am NetworkMessage
	am.Constructors = c.host.constructors
	var err error
	//c.Conn.SetReadDeadline(time.Now().Add(timeOut))
	// First read the size
	var s Size
	defer func() {
		if e := recover(); e != nil {
			dbg.Print("ERROR SIZE:", s, " => ", e)
			panic(e)
		}
	}()
	if err = binary.Read(c.Conn, globalOrder, &s); err != nil {
		return EmptyApplicationMessage, handleError(err)
	}
	b := make([]byte, s)
	var read Size
	var buffer bytes.Buffer
	for Size(buffer.Len()) < s {
		// read the size of the next packet
		n, err := c.Conn.Read(b)
		// if error then quit
		if err != nil {
			e := handleError(err)
			return EmptyApplicationMessage, e
		}
		// put it in the longterm buffer
		buffer.Write(b[:n])
		// if we could not read everything yet
		if Size(buffer.Len()) < s {
			// make b size = bytes that we still need to read (no more no less)
			b = b[:s-read]
		}
	}
	defer func() {
		if e := recover(); e != nil {
			debug.PrintStack()
			dbg.Errorf("Error Unmarshalling %s: %d bytes : %v\n", am.MsgType, len(buffer.Bytes()), e)
			dbg.Error(hex.Dump(buffer.Bytes()))
		}
	}()

	err = am.UnmarshalBinary(buffer.Bytes())
	if err != nil {
		return EmptyApplicationMessage, fmt.Errorf("Error unmarshaling message type %s: %s", am.MsgType.String(), err.Error())
	}
	am.From = c.Remote()
	return am, nil
}

// how many bytes do we write at once on the socket
var maxChunkSize Size = 4096

// Send will convert the NetworkMessage into an ApplicationMessage
// and send it with the size through the network.
// Returns an error if anything was wrong
func (c *TcpConn) Send(ctx context.Context, obj ProtocolMessage) error {
	am, err := newNetworkMessage(obj)
	if err != nil {
		return fmt.Errorf("Error converting packet: %v\n", err)
	}
	dbg.Lvl5("Message SEND =>", fmt.Sprintf("%+v", am))
	var b []byte
	b, err = am.MarshalBinary()
	if err != nil {
		return fmt.Errorf("Error marshaling  message: %s", err.Error())
	}
	//c.Conn.SetWriteDeadline(time.Now().Add(timeOut))
	// First write the size
	packetSize := Size(len(b))
	if err := binary.Write(c.Conn, globalOrder, packetSize); err != nil {
		return err
	}
	// Then send everything through the connection
	// Send chunk by chunk
	var sent Size
	for sent < packetSize {
		var offset Size
		// if we have yet more than "chunk" byte to write
		if packetSize-sent > maxChunkSize {
			offset = maxChunkSize
		} else {
			// take only the rest
			offset = packetSize - sent
		}

		// if one write went wrong or stg
		if offset < Size(len(b)) {
			offset = Size(len(b))
		}

		// chunk to send
		chunk := b[:offset]
		// bytes left to send
		b = b[:offset]
		n, err := c.Conn.Write(chunk)
		if err != nil {
			return handleError(err)
		}
		sent += Size(n)
	}

	return nil
}

// Close ... closes the connection
func (c *TcpConn) Close() error {
	if c.closed == true {
		return nil
	}
	err := c.Conn.Close()
	c.closed = true
	if err != nil {
		return handleError(err)
	}
	return nil
}

// OpenTcpCOnn is private method that opens a TcpConn to the given name
func (t *TcpHost) openTcpConn(name string) (*TcpConn, error) {
	var err error
	var conn net.Conn
	for i := 0; i < maxRetry; i++ {
		conn, err = net.Dial("tcp", name)
		if err != nil {
			//dbg.Lvl5("(", i, "/", maxRetry, ") Error opening connection to", name)
			time.Sleep(waitRetry)
		} else {
			break
		}
		time.Sleep(waitRetry)
	}
	if conn == nil {
		return nil, fmt.Errorf("Could not connect to %s.", name)
	}
	c := TcpConn{
		Endpoint: name,
		Conn:     conn,
		host:     t,
	}

	return &c, err
}

// listen is the private function that takes a function that takes a TcpConn.
// That way we can control what to do of the TcpConn before returning it to the
// function given by the user. Used by SecureTcpHost
func (t *TcpHost) listen(addr string, fn func(*TcpConn)) error {
	t.listeningLock.Lock()
	t.listening = true
	global, _ := cliutils.GlobalBind(addr)
	ln, err := net.Listen("tcp", global)
	if err != nil {
		return errors.New("Error opening listener: " + err.Error())
	}
	t.listener = ln
	var unlocked bool
	for {
		if !unlocked {
			unlocked = true
			t.listeningLock.Unlock()
		}
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
		c := TcpConn{
			Endpoint: conn.RemoteAddr().String(),
			Conn:     conn,
			host:     t,
		}
		t.peers[conn.RemoteAddr().String()] = &c
		fn(&c)
	}
	return nil
}

// NewSecureTcpHost returns a Secure Tcp Host
func NewSecureTcpHost(private abstract.Secret, e *Entity) *SecureTcpHost {
	return &SecureTcpHost{
		private:        private,
		entity:         e,
		EntityToAddr:   make(map[uuid.UUID]string),
		TcpHost:        NewTcpHost(),
		workingAddress: e.First(),
	}
}

// Listen will try each addresses it the host Entity.
// Returns an error if it can listen on any address
func (st *SecureTcpHost) Listen(fn func(SecureConn)) error {
	receiver := func(c *TcpConn) {
		dbg.Lvl3("Listening on", c.Remote())
		stc := &SecureTcpConn{
			TcpConn:       c,
			SecureTcpHost: st,
		}
		// if negociation fails we drop the connection
		if err := stc.negotiateListen(); err != nil {
			dbg.Error("Negociation failed:", err)
			stc.Close()
			return
		}
		go fn(stc)
	}
	var addr string
	var err error
	dbg.Lvl3("Addresses are", st.entity.Addresses)
	for _, addr = range st.entity.Addresses {
		dbg.Lvl3("Starting to listen on", addr)
		st.workingAddress = addr
		if err = st.TcpHost.listen(addr, receiver); err != nil {
			// The listening is over
			if err == ErrClosed || err == ErrEOF {
				return nil
			}
		} else {
			return nil
		}
	}
	return fmt.Errorf("No address worked for listening on this host", err)
}

// Open will try any address that is in the Entity and connect to the first
// one that works. Then it exchanges the Entity to verify.
func (st *SecureTcpHost) Open(e *Entity) (SecureConn, error) {
	var secure SecureTcpConn
	var success bool
	// try all names
	for _, addr := range e.Addresses {
		// try to connect with this name
		dbg.Lvl3("Trying address", addr)
		c, err := st.TcpHost.openTcpConn(addr)
		if err != nil {
			dbg.Lvl3("Address didn't accept connection:", addr, "=>", err)
			continue
		}
		// create the secure connection
		secure = SecureTcpConn{
			TcpConn:       c,
			SecureTcpHost: st,
			entity:        e,
		}
		success = true
		break
	}
	if !success {
		return nil, fmt.Errorf("Could not connect to any address tied to this Entity")
	}
	// Exchange and verify entities
	return &secure, secure.negotiateOpen(e)
}

// Receive is analog to Conn.Receive but also set the right Entity in the
// message
func (sc *SecureTcpConn) Receive(ctx context.Context) (NetworkMessage, error) {
	nm, err := sc.TcpConn.Receive(ctx)
	nm.Entity = sc.entity
	return nm, err
}

func (sc *SecureTcpConn) Entity() *Entity {
	return sc.entity
}

// negotitateListen is made to exchange the Entity between the two parties.
// when a connection request is made during listening
func (sc *SecureTcpConn) negotiateListen() error {
	// Send our Entity to the remote endpoint
	dbg.Lvl4("Sending our identity")
	if err := sc.TcpConn.Send(context.TODO(), sc.SecureTcpHost.entity); err != nil {
		return fmt.Errorf("Error while sending indentity during negotiation:%s", err)
	}
	// Receive the other Entity
	nm, err := sc.TcpConn.Receive(context.TODO())
	if err != nil {
		return fmt.Errorf("Error while receiving Entity during negotiation %s", err)
	}
	dbg.Lvl4("Received our identity")
	// Check if it is correct
	if nm.MsgType != EntityType {
		return fmt.Errorf("Received wrong type during negotiation %s", nm.MsgType.String())
	}

	// Set the Entity for this connection
	e := nm.Msg.(Entity)
	sc.entity = &e
	dbg.Lvl4("Identity exchange complete")
	return nil
}

// negotiateOpen is called when Open a connection is called. Plus
// negotiateListen it also verifiy the Entity.
func (sc *SecureTcpConn) negotiateOpen(e *Entity) error {
	if err := sc.negotiateListen(); err != nil {
		return err
	}

	// verify the Entity if its the same we are supposed to connect
	if sc.Entity().Id != e.Id {
		return fmt.Errorf("Entity received during negotiation is wrong. WARNING")
	}

	return nil
}
