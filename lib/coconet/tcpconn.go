package coconet

import (
	"encoding/json"
	"errors"
	"net"
	"sync"
	//"runtime/debug"

	"github.com/dedis/cothority/lib/dbg"

	"github.com/dedis/crypto/abstract"
	"io"
	"strings"
)

var Latency = 100

// TCPConn is an implementation of the Conn interface for TCP network connections.
type TCPConn struct {
	// encLock guards the encoder and decoder and underlying conn.
	encLock sync.Mutex
	name    string
	conn    net.Conn
	enc     *json.Encoder
	dec     *json.Decoder

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
		enc:  json.NewEncoder(conn),
		dec:  json.NewDecoder(conn)}

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
	tc.enc = json.NewEncoder(conn)
	tc.dec = json.NewDecoder(conn)
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
func (tc *TCPConn) PutData(bm BinaryMarshaler) error {
	if tc.Closed() {
		dbg.Lvl3("tcpconn: put: connection closed")
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
// Returns io.EOF on an irrecoverable error.
// Returns given error if it is Temporary.
func (tc *TCPConn) GetData(bum BinaryUnmarshaler) error {
	if tc.Closed() {
		dbg.Lvl3("tcpconn: get: connection closed")
		return ErrClosed
	}
	tc.encLock.Lock()
	for tc.dec == nil {
		tc.encLock.Unlock()
		return ErrNotEstablished
	}
	dec := tc.dec
	tc.encLock.Unlock()

	//if Latency != 0 {
	//	time.Sleep(time.Duration(rand.Intn(Latency)) * time.Millisecond)
	//}
	err := dec.Decode(bum)
	if err != nil {
		if IsTemporary(err) {
			dbg.Lvl2("Temporary error")
			return err
		}
		// if it is an irrecoverable error
		// close the channel and return that it has been closed
		if err == io.EOF || err.Error() == "read tcp4" {
			dbg.Lvl3("Closing connection by EOF:", err)
		} else {
			if !strings.Contains(err.Error(), "use of closed") {
				dbg.Lvl1("Couldn't decode packet at", tc.name, "error:", err)
				dbg.Lvlf1("Packet was: %+v", bum)
			}
		}
		tc.Close()
		return ErrClosed
	}
	return err
}

// Close closes the connection.
func (tc *TCPConn) Close() {
	dbg.Lvl3("tcpconn: closing connection")
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
