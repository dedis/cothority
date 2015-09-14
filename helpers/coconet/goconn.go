package coconet

import (
	"errors"
	"math/rand"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/dedis/crypto/abstract"
)

// GoDirectory is a testing structure for the goConn. It allows us to simulate
// tcp network connections locally (and is easily adaptable for network
// connections). A single directory should be shared between all goConns's that
// are operating in the same network 'space'. If they are on the same tree they
// should share a directory.
type GoDirectory struct {
	// protects accesses to channel and nameToPeer
	sync.RWMutex
	// channel is a map from each peer-to-peer connection to channel.
	channel map[string]chan []byte
	// nameToPeer maps each connection (from-to) to a GoConn
	nameToPeer map[string]*GoConn
	// closed
	closed map[string]bool
}

// NewGoDirectory creates a new directory for registering GoConns.
func NewGoDirectory() *GoDirectory {
	return &GoDirectory{
		channel:    make(map[string]chan []byte),
		nameToPeer: make(map[string]*GoConn),
		closed:     make(map[string]bool)}
}

func (d *GoDirectory) Close() {
	d.Lock()
	for k := range d.closed {
		d.closed[k] = true
	}
	for k := range d.channel {
		d.closed[k] = true
	}
	d.Unlock()
}

// GoConn implements the Conn interface.
// It emulates connections by using channels.
// uses channels for communication.
type GoConn struct {
	// The directory maps each (from,to) pair to a channel for sending (from,to).
	// When receiving one reads from the channel (to, from).
	// This corresponds to the channel the sender would write to.
	// Thus the sender "owns" the channel.
	dir    *GoDirectory
	from   string
	to     string
	fromto string
	tofrom string

	// mupk guards the public key
	mupk   sync.RWMutex
	pubkey abstract.Point

	closed bool
}

// ErrPeerExists is an ignorable error that says that this peer has already been
// registered to this directory.
var ErrPeerExists = errors.New("peer already exists in given directory")

// NewGoConn creates a GoConn registered in the given directory with the given
// hostname. It returns an ignorable ErrPeerExists error if this peer already
// exists.
func NewGoConn(dir *GoDirectory, from, to string) (*GoConn, error) {
	gc := &GoConn{dir, from, to, from + "::::" + to, to + "::::" + from, sync.RWMutex{}, nil, false}
	dir.Lock()
	defer dir.Unlock()
	fromto := gc.fromto
	tofrom := gc.tofrom
	if c, ok := dir.nameToPeer[fromto]; ok {
		// return the already existant peer\
		return c, ErrPeerExists
	}
	dir.nameToPeer[fromto] = gc
	if _, ok := dir.channel[fromto]; !ok {
		dir.channel[fromto] = make(chan []byte, 100)
		dir.closed[fromto] = false
	}
	if _, ok := dir.channel[tofrom]; !ok {
		dir.channel[tofrom] = make(chan []byte, 100)
		dir.closed[tofrom] = false
	}
	return gc, nil
}

// Name implements the Conn Name interface.
// It returns the To end of the connection.
func (c *GoConn) Name() string {
	return c.to
}

// Connect implements the Conn Connect interface.
// For GoConn's it is a no-op.
func (c *GoConn) Connect() error {
	return nil
}

func (c *GoConn) Closed() bool {
	c.dir.Lock()
	closed := c.closed
	c.dir.Unlock()
	return closed
}

// Close implements the Conn Close interface.
func (c *GoConn) Close() {
	c.dir.Lock()
	c.closed = true
	c.dir.closed[c.fromto] = true
	c.dir.closed[c.tofrom] = true
	c.dir.Unlock()

}

// SetPubKey sets the public key of the connection.
func (c *GoConn) SetPubKey(pk abstract.Point) {
	c.mupk.Lock()
	c.pubkey = pk
	c.mupk.Unlock()
}

// PubKey returns the public key of the connection.
func (c *GoConn) PubKey() abstract.Point {
	c.mupk.RLock()
	pl := c.pubkey
	c.mupk.RUnlock()
	return pl
}

// Put puts data on the connection.
func (c *GoConn) Put(data BinaryMarshaler) error {
	if c.Closed() {
		return ErrClosed
	}
	fromto := c.fromto
	c.dir.RLock()
	ch := c.dir.channel[fromto]
	closed := c.dir.closed[fromto]
	c.dir.RUnlock()

	if closed {
		return ErrClosed
	}

	b, err := data.MarshalBinary()
	if err != nil {
		return err
	}
	ticker := time.Tick(1000 * time.Millisecond)

retry:
	select {
	case ch <- b:
	case <-ticker:
		c.dir.RLock()
		closed, ok := c.dir.closed[fromto]
		c.dir.RUnlock()

		if closed {
			log.Println("detected closed channel: putting:", fromto, ok)
			return ErrClosed
		}
		log.Println("retry")
		goto retry
	}
	return nil
}

// Get receives data from the sender.
func (c *GoConn) Get(bum BinaryUnmarshaler) error {
	if c.Closed() {
		return ErrClosed
	}
	tofrom := c.tofrom
	c.dir.RLock()
	ch := c.dir.channel[tofrom]
	closed, ok := c.dir.closed[tofrom]
	c.dir.RUnlock()

	if closed || !ok {
		return ErrClosed
	}

	var data []byte
	if Latency != 0 {
		time.Sleep(time.Duration(rand.Intn(Latency)) * time.Millisecond)
	}

	ticker := time.Tick(1000 * time.Millisecond)
retry:
	select {
	case data = <-ch:
	case <-ticker:
		c.dir.RLock()
		closed, ok := c.dir.closed[tofrom]
		c.dir.RUnlock()

		// if the channel has been closed then exit
		if closed {
			log.Println("detected closed channl: getting:", tofrom, ok)
			return ErrClosed
		}
		goto retry
	}
	err := bum.UnmarshalBinary(data)
	return err
}
