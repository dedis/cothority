package coconet

import (
	"encoding/gob"
	"errors"
	"math/rand"
	"net"
	"sync"
	"time"
	//"runtime/debug"

	log "github.com/Sirupsen/logrus"

	"github.com/dedis/crypto/abstract"
)

var Latency = 100

// TCPConn is an implementation of the Conn interface for TCP network connections.
type TCPConn struct {
	// encLock guards the encoder and decoder and underlying conn.
	encLock sync.Mutex
	name    string
	conn    net.Conn
	enc     *gob.Encoder
	dec     *gob.Decoder

	// pkLock guards the public key
	pkLock sync.Mutex
	pubkey abstract.Point

	closed bool
}

// NewTCPConnFromNet wraps a net.Conn creating a new TCPConn using conn as the
// underlying connection.
// After creating a TCPConn in this fashion, it might be necessary to call SetName,
// in order to give it an understandable name.
func NewTCPConnFromNet(conn net.Conn) *TCPConn {
	return &TCPConn{
		name: conn.RemoteAddr().String(),
		conn: conn,
		enc:  gob.NewEncoder(conn),
		dec:  gob.NewDecoder(conn)}

}

// NewTCPConn takes a hostname and creates TCPConn.
// Before calling Get or Put Connect must first be called to establish the connection.
func NewTCPConn(hostname string) *TCPConn {
	tp := &TCPConn{}
	tp.name = hostname
	return tp
}

func (tc *TCPConn) Closed() bool {
	tc.encLock.Lock()
	closed := tc.closed
	tc.encLock.Unlock()
	return closed
}

// Connect connects to the endpoint specified.
func (tc *TCPConn) Connect() error {
	conn, err := net.Dial("tcp", tc.name)
	if err != nil {
		return err
	}
	tc.encLock.Lock()
	tc.conn = conn
	tc.enc = gob.NewEncoder(conn)
	tc.dec = gob.NewDecoder(conn)
	tc.encLock.Unlock()
	return nil
}

// SetName sets the name of the connection.
func (tc *TCPConn) SetName(name string) {
	tc.name = name
}

// Name returns the name of the connection.
func (tc *TCPConn) Name() string {
	return tc.name
}

// SetPubKey sets the public key.
func (tc *TCPConn) SetPubKey(pk abstract.Point) {
	tc.pkLock.Lock()
	tc.pubkey = pk
	tc.pkLock.Unlock()
}

// PubKey returns the public key of this peer.
func (tc *TCPConn) PubKey() abstract.Point {
	tc.pkLock.Lock()
	pl := tc.pubkey
	tc.pkLock.Unlock()
	return pl
}

// ErrNotEstablished indicates that the connection has not been successfully established
// through a call to Connect yet. It does not indicate whether the failure was permanent or
// temporary.
var ErrNotEstablished = errors.New("connection not established")

type temporary interface {
	Temporary() bool
}

// IsTemporary returns true if it is a temporary error.
func IsTemporary(err error) bool {
	t, ok := err.(temporary)
	return ok && t.Temporary()
}

// Put puts data to the connection.
// Returns io.EOF on an irrecoverable error.
// Returns actual error if it is Temporary.
func (tc *TCPConn) Put(bm BinaryMarshaler) error {
	if tc.Closed() {
		log.Errorln("tcpconn: put: connection closed")
		return ErrClosed
	}
	tc.encLock.Lock()
	if tc.enc == nil {
		tc.encLock.Unlock()
		return ErrNotEstablished
	}
	enc := tc.enc
	tc.encLock.Unlock()

	err := enc.Encode(bm)
	if err != nil {
		if IsTemporary(err) {
			return err
		}
		tc.Close()
		return ErrClosed
	}
	return err
}

// Get gets data from the connection.
// Returns io.EOF on an irrecoveralbe error.
// Returns given error if it is Temporary.
func (tc *TCPConn) Get(bum BinaryUnmarshaler) error {
	log.Println("TCPConn-Get", tc.name)
	if tc.Closed() {
		log.Errorln("tcpconn: get: connection closed")
		return ErrClosed
	}
	tc.encLock.Lock()
	for tc.dec == nil {
		tc.encLock.Unlock()
		return ErrNotEstablished
	}
	dec := tc.dec
	tc.encLock.Unlock()

	if Latency != 0 {
		time.Sleep(time.Duration(rand.Intn(Latency)) * time.Millisecond)
	}
	err := dec.Decode(bum)
	if err != nil {
		if IsTemporary(err) {
			return err
		}
		// if it is an irrecoverable error
		// close the channel and return that it has been closed
		log.Errorln("Couldn't decode packet:", err)
		tc.Close()
		return ErrClosed
	}
	return err
}

// Close closes the connection.
func (tc *TCPConn) Close() {
	log.Errorln("tcpconn: closing connection")
	tc.encLock.Lock()
	defer tc.encLock.Unlock()
	if tc.conn != nil {
		// ignore error because only other possibility was an invalid
		// connection. but we don't care if we close a connection twice.
		tc.conn.Close()
	}
	tc.closed = true
	tc.conn = nil
	tc.enc = nil
	tc.dec = nil
}
