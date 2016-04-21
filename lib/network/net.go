// Package network is a networking library used in the SDA. You have Hosts which can
// issue connections to others hosts, and Conn which are the connections itself.
// Hosts and Conns are interfaces and can be of type Tcp, or Chans, or Udp or
// whatever protocols you think might implement this interface.
// In this library we also provide a way to encode / decode any kind of packet /
// structs. When you want to send a struct to a conn, you first register
// (one-time operation) this packet to the library, and then directly pass the
// pointer to the struct itself to the conn that will recognize its type. When decoding,
// it will automatically detect the underlying type of struct given, and decode
// it accordingly. You can provide your own decode / encode methods if for
// example, you have a variable length packet structure. Since this library uses
// github.com/dedis/protobuf library for encoding, you just have to
// implement MarshalBinary or UnmarshalBinary.
package network

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"golang.org/x/net/context"

	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

// Network part //

// NewTCPHost returns a Fresh TCP Host
// If constructors == nil, it will take an empty one.
func NewTCPHost() *TCPHost {
	return &TCPHost{
		peers:        make(map[string]Conn),
		quit:         make(chan bool),
		constructors: DefaultConstructors(Suite),
		quitListener: make(chan bool),
	}
}

// Open will create a new connection between this host
// and the remote host named "name". This is a TCPConn.
// If anything went wrong, Conn will be nil.
func (t *TCPHost) Open(name string) (Conn, error) {
	c, err := t.openTCPConn(name)
	if err != nil {
		return nil, err
	}
	// XXX Are we sure we need this mutex here ?
	t.peersMut.Lock()
	defer t.peersMut.Unlock()
	t.peers[name] = c
	return c, nil
}

// Listen for any host trying to contact him.
// Will launch in a goroutine the srv function once a connection is established
func (t *TCPHost) Listen(addr string, fn func(Conn)) error {
	receiver := func(tc *TCPConn) {
		go fn(tc)
	}
	return t.listen(addr, receiver)
}

// Close will close every connection this host has opened
func (t *TCPHost) Close() error {
	t.peersMut.Lock()
	defer t.peersMut.Unlock()
	for _, c := range t.peers {
		// dbg.Lvl4("Closing peer", c)
		if err := c.Close(); err != nil {
			return handleError(err)
		}
	}

	t.closedLock.Lock()
	if !t.closed {
		close(t.quit)
	}
	t.closed = true
	t.closedLock.Unlock()

	// lets see if we launched a listening routing
	var listening bool
	t.listeningLock.Lock()
	listening = t.listening
	t.listeningLock.Unlock()
	// we are NOT listening
	if !listening {
		return nil
	}
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

// Rx returns the number of bytes read by all its connections
// Needed so TcpHost implements the CounterIO interface from lib/monitor
func (t *TCPHost) Rx() uint64 {
	var size uint64
	for _, c := range t.peers {
		size += c.Rx()
	}
	return size
}

// Tx returns the number of bytes written by all its connection
// Needed so TcpHost implements the CounterIO interface from lib/monitor
func (t *TCPHost) Tx() uint64 {
	var size uint64
	for _, c := range t.peers {
		size += c.Tx()
	}
	return size
}

// OpenTCPConn is private method that opens a TCPConn to the given name
func (t *TCPHost) openTCPConn(name string) (*TCPConn, error) {
	var err error
	var conn net.Conn
	for i := 0; i < MaxRetry; i++ {
		conn, err = net.Dial("tcp", name)
		if err != nil {
			//dbg.Lvl5("(", i, "/", maxRetry, ") Error opening connection to", name)
			time.Sleep(WaitRetry)
		} else {
			break
		}
		time.Sleep(WaitRetry)
	}
	if conn == nil {
		return nil, fmt.Errorf("Could not connect to %s: %s", name, err)
	}
	c := TCPConn{
		Endpoint: name,
		conn:     conn,
		host:     t,
	}

	return &c, err
}

// listen is the private function that takes a function that takes a TCPConn.
// That way we can control what to do of the TCPConn before returning it to the
// function given by the user. Used by SecureTCPHost
func (t *TCPHost) listen(addr string, fn func(*TCPConn)) error {
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
		c := TCPConn{
			Endpoint: conn.RemoteAddr().String(),
			conn:     conn,
			host:     t,
		}
		t.peersMut.Lock()
		t.peers[conn.RemoteAddr().String()] = &c
		t.peersMut.Unlock()
		fn(&c)
	}
}

// NewSecureTCPHost returns a Secure Tcp Host
// If the entity is nil, it will not verify the identity of the
// remote host
func NewSecureTCPHost(private abstract.Secret, e *Entity) *SecureTCPHost {
	addr := ""
	if e != nil {
		addr = e.First()
	}
	return &SecureTCPHost{
		private:        private,
		entity:         e,
		TCPHost:        NewTCPHost(),
		workingAddress: addr,
	}
}

// Listen will try each addresses it the host Entity.
// Returns an error if it can listen on any address
func (st *SecureTCPHost) Listen(fn func(SecureConn)) error {
	receiver := func(c *TCPConn) {
		dbg.Lvl3(st.workingAddress, "connected with", c.Remote())
		stc := &SecureTCPConn{
			TCPConn:       c,
			SecureTCPHost: st,
		}
		// if negotiation fails we drop the connection
		if err := stc.exchangeEntity(); err != nil {
			dbg.Error("Negotiation failed:", err)
			if err := stc.Close(); err != nil {
				dbg.Error("Couldn't close secure connection:",
					err)
			}
			return
		}
		st.connMutex.Lock()
		st.conns = append(st.conns, stc)
		st.connMutex.Unlock()
		go fn(stc)
	}
	var addr string
	var err error
	if st.entity == nil {
		return errors.New("Can't listen without Entity")
	}
	dbg.Lvl3("Addresses are", st.entity.Addresses)
	for _, addr = range st.entity.Addresses {
		dbg.Lvl3("Starting to listen on", addr)
		st.lockAddress.Lock()
		st.workingAddress = addr
		st.lockAddress.Unlock()
		if err = st.TCPHost.listen(addr, receiver); err != nil {
			// The listening is over
			if err == ErrClosed || err == ErrEOF {
				return nil
			}
		} else {
			return nil
		}
	}
	return fmt.Errorf("No address worked for listening on this host %+s.",
		err.Error())
}

// Open will try any address that is in the Entity and connect to the first
// one that works. Then it exchanges the Entity to verify it is talking with the
// right host.
func (st *SecureTCPHost) Open(e *Entity) (SecureConn, error) {
	var secure SecureTCPConn
	var success bool
	// try all names
	for _, addr := range e.Addresses {
		// try to connect with this name
		dbg.Lvl3("Trying address", addr)
		c, err := st.TCPHost.openTCPConn(addr)
		if err != nil {
			dbg.Lvl3("Address didn't accept connection:", addr, "=>", err)
			continue
		}
		// create the secure connection
		secure = SecureTCPConn{
			TCPConn:       c,
			SecureTCPHost: st,
			entity:        e,
		}
		success = true
		break
	}
	if !success {
		return nil, fmt.Errorf("Could not connect to any address tied to this Entity")
	}
	// Exchange and verify entities
	err := secure.negotiateOpen(e)
	if err == nil {
		st.connMutex.Lock()
		st.conns = append(st.conns, &secure)
		st.connMutex.Unlock()
	}
	return &secure, err
}

// String returns a string identifying that host
func (st *SecureTCPHost) String() string {
	st.lockAddress.Lock()
	defer st.lockAddress.Unlock()
	return st.workingAddress
}

// Tx implements the CounterIO interface
func (st *SecureTCPHost) Tx() uint64 {
	st.connMutex.Lock()
	defer st.connMutex.Unlock()
	var b uint64
	for _, c := range st.conns {
		b += c.Tx()
	}
	return b
}

// Rx implements the CounterIO interface
func (st *SecureTCPHost) Rx() uint64 {
	st.connMutex.Lock()
	defer st.connMutex.Unlock()
	var b uint64
	for _, c := range st.conns {
		b += c.Rx()
	}
	return b
}

// Remote returns the name of the peer at the end point of
// the connection
func (c *TCPConn) Remote() string {
	return c.Endpoint
}

// Receive waits for any input on the connection and returns
// the ApplicationMessage **decoded** and an error if something
// wrong occured
func (c *TCPConn) Receive(ctx context.Context) (nm Message, e error) {
	c.receiveMutex.Lock()
	defer c.receiveMutex.Unlock()
	var am Message
	am.Constructors = c.host.constructors
	var err error
	//c.Conn.SetReadDeadline(time.Now().Add(timeOut))
	// First read the size
	var s Size
	defer func() {
		if err := recover(); err != nil {
			nm = EmptyApplicationMessage
			e = fmt.Errorf("Error Received message (size=%d): %v", s, err)
		}
	}()
	if err = binary.Read(c.conn, globalOrder, &s); err != nil {
		return EmptyApplicationMessage, handleError(err)
	}
	b := make([]byte, s)
	var read Size
	var buffer bytes.Buffer
	for Size(buffer.Len()) < s {
		// read the size of the next packet
		n, err := c.conn.Read(b)
		// if error then quit
		if err != nil {
			e := handleError(err)
			return EmptyApplicationMessage, e
		}
		// put it in the longterm buffer
		if _, err := buffer.Write(b[:n]); err != nil {
			dbg.Error("Couldn't write to buffer:", err)
		}
		read += Size(n)
		// if we could not read everything yet
		if Size(buffer.Len()) < s {
			// make b size = bytes that we still need to read (no more no less)
			b = b[:s-read]
		}
	}

	err = am.UnmarshalBinary(buffer.Bytes())
	if err != nil {
		return EmptyApplicationMessage, fmt.Errorf("Error unmarshaling message type %s: %s", am.MsgType.String(), err.Error())
	}
	am.From = c.Remote()
	// set the size read
	c.addReadBytes(uint64(s))
	return am, nil
}

// how many bytes do we write at once on the socket
// 1400 seems a safe choice regarding the size of a ethernet packet.
// https://stackoverflow.com/questions/2613734/maximum-packet-size-for-a-tcp-connection
const maxChunkSize Size = 1400

// Send will convert the NetworkMessage into an ApplicationMessage
// and send it with the size through the network.
// Returns an error if anything was wrong
func (c *TCPConn) Send(ctx context.Context, obj ProtocolMessage) error {
	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()
	am, err := NewNetworkMessage(obj)
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
	if err := binary.Write(c.conn, globalOrder, packetSize); err != nil {
		dbg.Error("Couldn't write number of bytes")
		return err
	}
	// Then send everything through the connection
	// Send chunk by chunk
	var sent Size
	for sent < packetSize {
		length := packetSize - sent
		if length > maxChunkSize {
			length = maxChunkSize
		}

		// Sending 'length' bytes
		n, err := c.conn.Write(b[:length])
		if err != nil {
			dbg.Error("Couldn't write chunk starting at", sent, "size", length, err)
			return handleError(err)
		}
		sent += Size(n)

		// bytes left to send
		b = b[n:]
	}
	// update stats on the conn
	c.addWrittenBytes(uint64(packetSize))
	return nil
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

// Rx returns the number of bytes read by this connection
// Needed so TCPConn implements the CounterIO interface from lib/monitor
func (c *TCPConn) Rx() uint64 {
	c.bRxLock.Lock()
	defer c.bRxLock.Unlock()
	return c.bRx
}

// addReadBytes add b bytes to the total number of bytes read
func (c *TCPConn) addReadBytes(b uint64) {
	c.bRxLock.Lock()
	defer c.bRxLock.Unlock()
	c.bRx += b
}

// Tx returns the number of bytes written by this connection
// Needed so TCPConn implements the CounterIO interface from lib/monitor
func (c *TCPConn) Tx() uint64 {
	c.bTxLock.Lock()
	defer c.bTxLock.Unlock()
	return c.bTx
}

// addWrittenBytes add b bytes to the total number of bytes written
func (c *TCPConn) addWrittenBytes(b uint64) {
	c.bTxLock.Lock()
	defer c.bTxLock.Unlock()
	c.bTx += b
}

// Receive is analog to Conn.Receive but also set the right Entity in the
// message
func (sc *SecureTCPConn) Receive(ctx context.Context) (Message, error) {
	nm, err := sc.TCPConn.Receive(ctx)
	nm.Entity = sc.entity
	return nm, err
}

// Entity returns the underlying entity tied to this connection
func (sc *SecureTCPConn) Entity() *Entity {
	return sc.entity
}

// exchangeEntity is made to exchange the Entity between the two parties.
// when a connection request is made during listening
func (sc *SecureTCPConn) exchangeEntity() error {
	ourEnt := sc.SecureTCPHost.entity
	if ourEnt == nil {
		ourEnt = NewEntity(config.NewKeyPair(Suite).Public, "")
	}
	// Send our Entity to the remote endpoint
	dbg.Lvl4("Sending our identity", ourEnt.ID, "to",
		sc.TCPConn.conn.RemoteAddr().String())
	if err := sc.TCPConn.Send(context.TODO(), ourEnt); err != nil {
		return fmt.Errorf("Error while sending indentity during negotiation:%s", err)
	}
	// Receive the other Entity
	nm, err := sc.TCPConn.Receive(context.TODO())
	if err != nil {
		return fmt.Errorf("Error while receiving Entity during negotiation %s", err)
	}
	// Check if it is correct
	if nm.MsgType != EntityType {
		return fmt.Errorf("Received wrong type during negotiation %s", nm.MsgType.String())
	}

	// Set the Entity for this connection
	e := nm.Msg.(Entity)
	dbg.Lvl4(ourEnt.ID, "Received identity", e.ID)

	sc.entity = &e
	dbg.Lvl4("Identity exchange complete")
	return nil
}

// negotiateOpen is called when Open a connection is called. Plus
// negotiateListen it also verify the Entity.
func (sc *SecureTCPConn) negotiateOpen(e *Entity) error {
	if err := sc.exchangeEntity(); err != nil {
		return err
	}
	if sc.SecureTCPHost.entity == nil {
		return nil
	}
	// verify the Entity if its the same we are supposed to connect
	if sc.Entity().ID != e.ID {
		dbg.Lvl3("Wanted to connect to", e, e.ID, "but got", sc.Entity(), sc.Entity().ID)
		dbg.Lvl3(e.Public, sc.Entity().Public)
		dbg.Lvl4("IDs not the same", dbg.Stack())
		return errors.New("Warning: Entity received during negotiation is wrong.")
	}

	return nil
}

// Rx implements the CounterIO interface
func (sc *SecureTCPConn) Rx() uint64 {
	return sc.TCPConn.Rx()
}

// Tx implements the CounterIO interface
func (sc *SecureTCPConn) Tx() uint64 {
	return sc.TCPConn.Tx()
}
