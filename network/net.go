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

	"strconv"

	"strings"

	"github.com/dedis/cothority/log"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

// Network part //

// NewTCPHost returns a Fresh TCP Host
// If constructors == nil, it will take an empty one.
func NewTCPHost() *TCPHost {
	return &TCPHost{
		listeningPort: make(chan int, 1),
		peers:         make(map[string]Conn),
		quit:          make(chan bool),
		constructors:  DefaultConstructors(Suite),
		quitListener:  make(chan bool),
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
		// log.Lvl4("Closing peer", c)
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
// Needed so TcpHost implements the CounterIO interface from monitor
func (t *TCPHost) Rx() uint64 {
	var size uint64
	for _, c := range t.peers {
		size += c.Rx()
	}
	return size
}

// Tx returns the number of bytes written by all its connection
// Needed so TcpHost implements the CounterIO interface from monitor
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
	for i := 0; i < MaxRetryConnect; i++ {
		conn, err = net.Dial("tcp", name)
		if err != nil {
			//log.Lvl5("(", i, "/", maxRetry, ") Error opening connection to", name)
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
	var ln net.Listener
	for i := 0; i < MaxRetryConnect; i++ {
		var err error
		ln, err = net.Listen("tcp", global)
		if err == nil {
			t.listener = ln
			break
		} else if i == MaxRetryConnect-1 {
			t.listeningLock.Unlock()
			return errors.New("Error opening listener: " + err.Error())
		}
		time.Sleep(WaitRetry)
	}

	// Send the actual listening port through the channel, in case
	// it was a ":0"-address where the system choses its own
	// port.
	_, p, _ := net.SplitHostPort(ln.Addr().String())
	port, err := strconv.Atoi(p)
	if err != nil {
		return errors.New("Couldn't find port: " + err.Error())
	}
	if len(t.listeningPort) == 0 {
		// If the channel is empty, else we'd block.
		log.Lvl3("Sending port", port, "over", t.listeningPort)
		t.listeningPort <- port
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
func NewSecureTCPHost(private abstract.Scalar, si *ServerIdentity) *SecureTCPHost {
	addr := ""
	if si != nil {
		addr = si.First()
	}
	return &SecureTCPHost{
		private:        private,
		serverIdentity: si,
		TCPHost:        NewTCPHost(),
		workingAddress: addr,
	}
}

// Listen will try each addresses it the host ServerIdentity.
// Returns an error if it can't listen on any of the addresses.
func (st *SecureTCPHost) Listen(fn func(SecureConn)) error {
	receiver := func(c *TCPConn) {
		stc := &SecureTCPConn{
			TCPConn:       c,
			SecureTCPHost: st,
		}
		// if negotiation fails we drop the connection
		if err := stc.exchangeServerIdentity(); err != nil {
			log.Warn("Negotiation failed:", err)
			if err := stc.Close(); err != nil {
				log.Warn("Couldn't close secure connection:",
					err)
			}
			return
		}
		st.connMutex.Lock()
		st.conns = append(st.conns, stc)
		st.connMutex.Unlock()
		go fn(stc)
	}
	var err error
	if st.serverIdentity == nil {
		return errors.New("Can't listen without ServerIdentity")
	}
	log.Lvl3("Addresses are", st.serverIdentity.Addresses)
	for i, addr := range st.serverIdentity.Addresses {
		log.Lvl3("Starting to listen on", addr)
		go func() {
			err = st.TCPHost.listen(addr, receiver)
			// The listening is over
			if err == nil || err == ErrClosed || err == ErrEOF {
				return
			}
			st.TCPHost.listeningPort <- -1
		}()
		port := <-st.TCPHost.listeningPort
		if port > 0 {
			// If the port we asked for is '0', we need to
			// update the address.
			if strings.HasSuffix(addr, ":0") {
				log.Lvl3("Got port", port)
				addr = strings.TrimRight(addr, "0") +
					strconv.Itoa(port)
				st.serverIdentity.Addresses[i] = addr
				st.lockAddress.Lock()
				st.workingAddress = addr
				st.lockAddress.Unlock()
			}
			return nil
		}
		err = fmt.Errorf("Couldn't open address %s", addr)
	}
	return fmt.Errorf("No address worked for listening on this host %+s.",
		err.Error())
}

// Open will try any address that is in the ServerIdentity and connect to the first
// one that works. Then it exchanges the ServerIdentity to verify it is talking with the
// right host.
func (st *SecureTCPHost) Open(si *ServerIdentity) (SecureConn, error) {
	var secure SecureTCPConn
	var success bool
	// try all names
	for _, addr := range si.Addresses {
		// try to connect with this name
		log.Lvl4("Trying address", addr)
		c, err := st.TCPHost.openTCPConn(addr)
		if err != nil {
			log.Lvl3("Address didn't accept connection:", addr, "=>", err)
			continue
		}
		// create the secure connection
		secure = SecureTCPConn{
			TCPConn:        c,
			SecureTCPHost:  st,
			serverIdentity: si,
		}
		success = true
		break
	}
	if !success {
		return nil, errors.New("Could not connect to any address tied to this ServerIdentity")
	}
	// Exchange and verify entities
	err := secure.negotiateOpen(si)
	if err == nil {
		st.connMutex.Lock()
		st.conns = append(st.conns, &secure)
		st.connMutex.Unlock()
	}
	log.Lvl3(secure.TCPConn.Local(), ": successfully connected and identified",
		secure.TCPConn.Remote())
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

// WorkingAddress returns the working address
func (st *SecureTCPHost) WorkingAddress() string {
	st.lockAddress.Lock()
	defer st.lockAddress.Unlock()
	return st.workingAddress
}

// Remote returns the name of the peer at the end point of
// the connection
func (c *TCPConn) Remote() string {
	return c.Endpoint
}

// Local returns the local address and port
func (c *TCPConn) Local() string {
	return c.conn.LocalAddr().String()
}

// Receive waits for any input on the connection and returns
// the ApplicationMessage **decoded** and an error if something
// wrong occured
func (c *TCPConn) Receive(ctx context.Context) (nm Packet, e error) {
	c.receiveMutex.Lock()
	defer c.receiveMutex.Unlock()
	var am Packet
	am.Constructors = c.host.constructors
	var err error
	//c.Conn.SetReadDeadline(time.Now().Add(timeOut))
	// First read the size
	var total Size
	defer func() {
		if err := recover(); err != nil {
			nm = EmptyApplicationPacket
			e = fmt.Errorf("Error Received message (size=%d): %v", total, err)
		}
	}()
	log.Lvl5("Starting to receive on", c.Local(), "from", c.Remote())
	if err = binary.Read(c.conn, globalOrder, &total); err != nil {
		return EmptyApplicationPacket, handleError(err)
	}
	log.Lvl5("Received some bytes", total)
	b := make([]byte, total)
	var read Size
	var buffer bytes.Buffer
	for read < total {
		// read the size of the next packet
		n, err := c.conn.Read(b)
		// if error then quit
		if err != nil {
			e := handleError(err)
			return EmptyApplicationPacket, e
		}
		// put it in the longterm buffer
		if _, err := buffer.Write(b[:n]); err != nil {
			log.Error("Couldn't write to buffer:", err)
		}
		read += Size(n)
		b = b[n:]
		log.Lvl5("Read", read, "out of", total, "bytes")
	}

	err = am.UnmarshalBinary(buffer.Bytes())
	if err != nil {
		log.Errorf("Read %d out of %d - buffer is %x", read, total, buffer.Bytes())
		DumpTypes()
		return EmptyApplicationPacket, fmt.Errorf("Error unmarshaling message type %s: %s", am.MsgType.String(), err.Error())
	}
	am.From = c.Remote()
	// set the size read
	c.addReadBytes(uint64(total))
	return am, nil
}

// how many bytes do we write at once on the socket
// 1400 seems a safe choice regarding the size of a ethernet packet.
// https://stackoverflow.com/questions/2613734/maximum-packet-size-for-a-tcp-connection
const maxChunkSize Size = 1400

// Send will convert the NetworkMessage into an ApplicationMessage
// and send it with the size through the network.
// Returns an error if anything was wrong
func (c *TCPConn) Send(ctx context.Context, obj Body) error {
	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()
	am, err := NewNetworkPacket(obj)
	if err != nil {
		return fmt.Errorf("Error converting packet: %v\n", err)
	}
	log.Lvlf5("%s->%s: Message SEND => %+v", c.Local(), c.Remote(), am)
	var b []byte
	b, err = am.MarshalBinary()
	if err != nil {
		return fmt.Errorf("Error marshaling  message: %s", err.Error())
	}
	// First write the size
	packetSize := Size(len(b))
	if err := binary.Write(c.conn, globalOrder, packetSize); err != nil {
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
		log.Lvl4("Sending from", c.conn.LocalAddr(), "to", c.conn.RemoteAddr())
		n, err := c.conn.Write(b[:length])
		if err != nil {
			log.Error("Couldn't write chunk starting at", sent, "size", length, err)
			log.Error(log.Stack())
			return handleError(err)
		}
		sent += Size(n)
		log.Lvl5("Sent", sent, "out of", packetSize)

		// bytes left to send
		b = b[n:]
	}
	log.Lvl5(c.Endpoint, "Sent a total of", sent, "bytes")
	// update stats on the connection
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
// Needed so TCPConn implements the CounterIO interface from monitor
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
// Needed so TCPConn implements the CounterIO interface from monitor
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

// Receive is analog to Conn.Receive but also set the right ServerIdentity in the
// message
func (sc *SecureTCPConn) Receive(ctx context.Context) (Packet, error) {
	nm, err := sc.TCPConn.Receive(ctx)
	nm.ServerIdentity = sc.serverIdentity
	return nm, err
}

// ServerIdentity returns the underlying entity tied to this connection
func (sc *SecureTCPConn) ServerIdentity() *ServerIdentity {
	return sc.serverIdentity
}

// exchangeServerIdentity is made to exchange the ServerIdentity between the two parties.
// when a connection request is made during listening
func (sc *SecureTCPConn) exchangeServerIdentity() error {
	ourEnt := sc.SecureTCPHost.serverIdentity
	if ourEnt == nil {
		ourEnt = NewServerIdentity(config.NewKeyPair(Suite).Public, "")
	}
	// Send our ServerIdentity to the remote endpoint
	log.Lvlf4("Sending our identity %x to %s", ourEnt.ID,
		sc.TCPConn.conn.RemoteAddr().String())
	if err := sc.TCPConn.Send(context.TODO(), ourEnt); err != nil {
		log.Error(err)
		return fmt.Errorf("Error while sending indentity during negotiation: %s", err)
	}

	log.Lvl4(sc.workingAddress, "waiting for identity")
	// Wait for a packet to arrive
	// Receive the other ServerIdentity
	nm, err := sc.TCPConn.Receive(context.TODO())
	if err != nil {
		return fmt.Errorf("%s: Error while receiving ServerIdentity during negotiation: %s",
			sc.workingAddress, err)
	}
	// Check if it is correct
	if nm.MsgType != ServerIdentityType {
		return fmt.Errorf("Received wrong type during negotiation: %s", nm.MsgType.String())
	}

	// Set the ServerIdentity for this connection
	e := nm.Msg.(ServerIdentity)
	log.Lvlf4("%x: Received identity %x", ourEnt.ID, e.ID)

	sc.serverIdentity = &e
	log.Lvl4("Identity exchange complete")
	return nil
}

// negotiateOpen is called when Open a connection is called. Plus
// negotiateListen it also verify the ServerIdentity.
func (sc *SecureTCPConn) negotiateOpen(si *ServerIdentity) error {
	if err := sc.exchangeServerIdentity(); err != nil {
		return err
	}
	if sc.SecureTCPHost.serverIdentity == nil {
		return nil
	}
	// verify the ServerIdentity if its the same we are supposed to connect
	if sc.ServerIdentity().ID != si.ID {
		log.Lvl3("Wanted to connect to", si, si.ID, "but got", sc.ServerIdentity(), sc.ServerIdentity().ID)
		log.Lvl3(si.Public, sc.ServerIdentity().Public)
		log.Lvl4("IDs not the same", log.Stack())
		return errors.New("Warning: ServerIdentity received during negotiation is wrong.")
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
