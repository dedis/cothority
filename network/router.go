package network

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dedis/cothority/log"
)

// Router handles all networking operations such as:
// * it listens to incoming connections using a host.Listener methods
// * open up new connections using  host.Connect's method
// * dispatch incoming message using a Dispatcher
// * dispatch outgoing message maintaining a translation
//   between ServerIdentity <-> address
// * manage the reconnections of non-working Conn,
// Most caller should use the creation function like NewTCPRouter(...),
// NewLocalRouter(...) then use the Host such as:
// `router.Start() // will listen for incoming Conn and block`
// `router.Stop() // will stop the listening and the managing of all Conn`
type Router struct {
	// id is our own ServerIdentity
	id *ServerIdentity
	// address is the real-actual address used by the listener.
	address Address
	// Dispatcher is used to dispatch incoming message to the right recipient
	Dispatcher

	host Host
	// connections keeps track of all active connections with the translation
	// use of an array because it happens that some connections are opened at
	// the same time on both endpoints, and thus registered one after the other,
	// erasing the first conn.
	connections map[ServerIdentityID][]Conn
	connsMut    sync.Mutex

	// boolean flag indicating that the router is already clos{ing,ed}
	isClosed  bool
	closedMut sync.Mutex

	// we wait that all handleConn routines are done
	wg    sync.WaitGroup
	wgMut sync.Mutex
}

// NewRouter returns a fresh Router giving its identity, and the host we want to
// use.
func NewRouter(own *ServerIdentity, h Host) *Router {
	r := &Router{
		id:          own,
		connections: make(map[ServerIdentityID][]Conn),
		host:        h,
		Dispatcher:  NewBlockingDispatcher(),
	}
	r.address = h.Address()
	r.host.Listening()
	h.Listening()
	return r
}

// Start will start the listening routine of the underlying Host. It is a
// blocking call until the listening is done (by calling r.Stop()).
func (r *Router) Start() {
	// The function given to the listener  does the exchange of ServerIdentity
	// and pass the connection along to the router.
	err := r.host.Listen(func(c Conn) {
		dst, err := r.exchangeServerIdentity(c)
		if err != nil {
			log.Error("ExchangeServerIdentity failed:", err)
			if err := c.Close(); err != nil {
				log.Error("Couldn't close secure connection:",
					err)
			}
			return
		}
		// pass it along
		r.launchHandleRoutine(dst, c)
	})
	if err != nil {
		log.Error("Error listening:", err)
	}
}

// Stop will stop the listening routine, and stop any routine of handling
// connections. Calling r.Start(), then r.Stop() then r.Start() again leads to
// an undefined behaviour. Callers should most of the time re-create a fresh
// Router.
func (r *Router) Stop() error {
	var err error
	if r.host.Listening() {
		err = r.host.Stop()
	}
	// set the isClosed to true
	r.closedMut.Lock()
	r.isClosed = true
	r.closedMut.Unlock()

	// then close all connections
	r.connsMut.Lock()
	for _, arr := range r.connections {
		// take all connections to close
		for _, c := range arr {
			if err := c.Close(); err != nil {
				log.Error(err)
			}
		}
	}
	r.connsMut.Unlock()

	// wait for all handleConn to finish
	r.wg.Wait()

	r.closedMut.Lock()
	r.isClosed = false
	r.closedMut.Unlock()
	if err != nil {
		return err
	}
	return nil
}

// Send sends to an ServerIdentity without wrapping the msg into a SDAMessage
func (r *Router) Send(e *ServerIdentity, msg Body) error {
	if msg == nil {
		return errors.New("Can't send nil-packet")
	}

	c := r.connection(e.ID)
	if c == nil {
		var err error
		c, err = r.connect(e)
		if err != nil {
			return err
		}
	}

	log.Lvlf4("%s sends to %s msg: %+v", r.address, e, msg)
	var err error
	err = c.Send(msg)
	if err != nil {
		log.Lvl2(r.address, "Couldn't send to", e, ":", err, "trying again")
		c, err := r.connect(e)
		if err != nil {
			return err
		}
		err = c.Send(msg)
		if err != nil {
			return err
		}
	}
	log.Lvl5("Message sent")
	return nil
}

func (r *Router) connect(si *ServerIdentity) (Conn, error) {
	c, err := r.host.Connect(si.Address)
	if err != nil {
		return nil, err
	}
	if err := r.negotiateOpen(si, c); err != nil {
		return nil, err
	}

	r.launchHandleRoutine(si, c)
	return c, nil

}

// handleConn is the main routine for a connection to wait for incoming
// messages.
func (r *Router) handleConn(remote *ServerIdentity, c Conn) {
	defer func() {
		// when leaving, unregister the connection
		if err := c.Close(); err != nil {
			log.Error(r.address, "having error closing conn to ", remote.Address, ":", err)
		}
		// and release one on the waitgroup
		r.wg.Done()
	}()
	address := c.Remote()
	log.Lvl3(r.address, "Handling new connection to ", remote.Address)
	for {
		packet, err := c.Receive()
		// So the receiver can know about the error
		packet.SetError(err)
		packet.From = address
		packet.ServerIdentity = remote

		// whether the router is closed
		if r.Closed() {
			// signal we are done with this go routine.
			return
		}

		if err != nil {
			// something went wrong on this connection
			log.Lvlf4("%+v got error (%+s) while receiving message", r.id.String(), err)

			if err == ErrClosed || err == ErrEOF || err == ErrTemp {
				// remote connection closed
				log.Lvl3(r.address, "handleConn with closed connection: stop (dst=", remote.Address, ")")
				return
			}
			// weird error let's try again
			log.Lvl3(r.id, "Error with connection", address, "=>", err)
			continue
		}

		if err := r.Dispatch(&packet); err != nil {
			log.Lvl3("Error dispatching:", err)
		}
	}
}

// connection returns the connection associated with this ServerIdentity. Nil if
// nothing found. It always return the first connection associated.
func (r *Router) connection(sid ServerIdentityID) Conn {
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	arr := r.connections[sid]
	if len(arr) == 0 {
		return nil
	}
	return arr[0]
}

// registerConnection registers an ServerIdentity for a new connection, mapped with the
// real physical address of the connection and the connection itself
// it locks (and unlocks when done):  networkLock
func (r *Router) registerConnection(remote *ServerIdentity, c Conn) {
	log.Lvl4(r.address, "Registers", remote.Address)
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	_, okc := r.connections[remote.ID]
	if okc {
		log.Lvl5("Connection already registered. Appending new connection to same identity.")
	}
	r.connections[remote.ID] = append(r.connections[remote.ID], c)
}

func (r *Router) launchHandleRoutine(dst *ServerIdentity, c Conn) {
	r.wg.Add(1)
	go r.handleConn(dst, c)
}

// Close shuts down all network connections and returns once all processing go
// routines are done.
func (r *Router) stopHandling() error {
	return nil
}

// Closed returns true if the router is closed (or is closing). For a router
// to be closed means that a call to Stop() must have been made.
func (r *Router) Closed() bool {
	r.closedMut.Lock()
	defer r.closedMut.Unlock()
	return r.isClosed
}

// close set the isClosed variable to true, any subsequent call to Closed()
// will return true.
func (r *Router) close() {
}

// Tx implements monitor/CounterIO
// It returns the Tx for all connections managed by this router
func (r *Router) Tx() uint64 {
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	var tx uint64
	for _, arr := range r.connections {
		for _, c := range arr {
			tx += c.Tx()
		}
	}
	return tx
}

// Rx implements monitor/CounterIO
// It returns the Rx for all connections managed by this router
func (r *Router) Rx() uint64 {
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	var rx uint64
	for _, arr := range r.connections {
		for _, c := range arr {
			rx += c.Rx()
		}
	}
	return rx
}

// Listening returns true if this router is listening or not.
func (r *Router) Listening() bool {
	return r.host.Listening()
}

// exchangeServerIdentity takes a fresh new conn issued by the listener and
// proceed to the exchanges of the server identities of both parties. It returns
// the ServerIdentity of the remote party and register the connection.
func (r *Router) exchangeServerIdentity(c Conn) (*ServerIdentity, error) {
	dst, err := exchangeServerIdentity(r.id, c)
	if err != nil {
		return nil, err
	}
	r.registerConnection(dst, c)
	return dst, nil
}

// negotiateOpen takes a fresh issued new Conn and the supposed destination's
// ServerIdentity. It proceeds to the exchange of identity and verifies that
// we are correctly dealing with the right remote party. It then registers the
// connection
// NOTE: This version is non secure at all, it's just a simple verification.
// Later will come the signing part,etc.
func (r *Router) negotiateOpen(si *ServerIdentity, c Conn) error {
	err := negotiateOpen(r.id, si, c)
	if err != nil {
		return err
	}
	r.registerConnection(si, c)
	return nil
}

func exchangeServerIdentity(own *ServerIdentity, c Conn) (*ServerIdentity, error) {
	// Send our ServerIdentity to the remote endpoint
	log.Lvl4(own.Address, "Sending our identity to", c.Remote())
	if err := c.Send(own); err != nil {
		return nil, fmt.Errorf("Error while sending out identity during negotiation:%s", err)
	}
	// Receive the other ServerIdentity
	nm, err := c.Receive()
	if err != nil {
		return nil, fmt.Errorf("Error while receiving ServerIdentity during negotiation %s", err)
	}
	// Check if it is correct
	if nm.MsgType != ServerIdentityType {
		return nil, fmt.Errorf("Received wrong type during negotiation %s", nm.MsgType.String())
	}

	// Set the ServerIdentity for this connection
	e := nm.Msg.(ServerIdentity)
	log.Lvl4(own.Address, "Identity exchange complete with ", e.Address)
	return &e, nil

}

func negotiateOpen(own, remote *ServerIdentity, c Conn) error {
	var err error
	var dst *ServerIdentity
	if dst, err = exchangeServerIdentity(own, c); err != nil {
		return err
	}
	// verify the ServerIdentity if its the same we are supposed to connect
	if dst.ID != remote.ID {
		log.Lvl4("IDs not the same", log.Stack())
		return errors.New("Warning: ServerIdentity received during negotiation is wrong.")
	}
	return nil
}
