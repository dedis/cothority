package network

import (
	"errors"
	"fmt"
	"sync"

	"golang.org/x/net/context"

	"github.com/dedis/cothority/log"
)

// Router handles all networking operations such as:
//   * listening to incoming connections using a host.Listener method
//   * opening up new connections using host.Connect method
//   * dispatching incoming message using a Dispatcher
//   * dispatching outgoing message maintaining a translation
//   between ServerIdentity <-> address
//   * managing the re-connections of non-working Conn
// Most caller should use the creation function like NewTCPRouter(...),
// NewLocalRouter(...) then use the Host such as:
//
//   router.Start() // will listen for incoming Conn and block
//   router.Stop() // will stop the listening and the managing of all Conn
type Router struct {
	// id is our own ServerIdentity
	id *ServerIdentity
	// Dispatcher is used to dispatch incoming message to the right recipient
	Dispatcher
	// Host listens for new connections
	host Host
	// connections keeps track of all active connections. Because a connection
	// can be opened at the same time on both endpoints, there can be more
	// than one connection per ServerIdentityID.
	connections map[ServerIdentityID][]Conn
	connsMut    sync.Mutex

	// boolean flag indicating that the router is already clos{ing,ed}.
	isClosed  bool
	closedMut sync.Mutex

	// wg waits for all handleConn routines to be done.
	wg    sync.WaitGroup
	wgMut sync.Mutex
}

// NewRouter returns a new Router attached to a ServerIdentity and the host we want to
// use.
func NewRouter(own *ServerIdentity, h Host) *Router {
	r := &Router{
		id:          own,
		connections: make(map[ServerIdentityID][]Conn),
		host:        h,
		Dispatcher:  NewBlockingDispatcher(),
	}
	own.Address = h.Address()
	return r
}

// Start the listening routine of the underlying Host. This is a
// blocking call until r.Stop() is called.
func (r *Router) Start() {
	// The function given to the listener exchanges the ServerIdentities
	// and passes the connection to the router.
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
		// start handleConn that waits for incoming messages and
		// dispatches them.
		r.launchHandleRoutine(dst, c)
	})
	if err != nil {
		log.Error("Error listening:", err)
	}
}

// Stop the listening routine, and stop any routine of handling
// connections. Calling r.Start(), then r.Stop() then r.Start() again leads to
// an undefined behaviour. Callers should most of the time re-create a fresh
// Router.
func (r *Router) Stop() error {
	err := r.host.Stop()
	err2 := r.stopHandling()
	r.reset()
	if err != nil {
		return err
	} else if err2 != nil {
		return err2
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

	log.Lvlf4("%s sends to %s msg: %+v", r.id.Address, e, msg)
	var err error
	err = c.Send(context.TODO(), msg)
	if err != nil {
		log.Lvl2(r.id.Address, "Couldn't send to", e, ":", err, "trying again")
		c, err := r.connect(e)
		if err != nil {
			return err
		}
		err = c.Send(context.TODO(), msg)
		if err != nil {
			return err
		}
	}
	log.Lvl5("Message sent")
	return nil
}

// connect starts a new connection and launches the listener for incoming
// messages.
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

// handleConn waits for incoming messages and calls the dispatcher for
// each new message. It only quits if the connection is closed or another
// unrecoverable error in the connection appears.
func (r *Router) handleConn(remote *ServerIdentity, c Conn) {
	defer func() {
		// Clean up the connection by making sure it's closed.
		if err := c.Close(); err != nil {
			log.Error(r.id.Address, "having error closing conn to ", remote.Address, ":", err)
		}
		r.wg.Done()
	}()
	address := c.Remote()
	log.Lvl3(r.id.Address, "Handling new connection to ", remote.Address)
	for {
		packet, err := c.Receive()
		// Writes the error in the packet sent to the dispatcher.
		packet.SetError(err)
		packet.From = address
		packet.ServerIdentity = remote

		if r.Closed() {
			return
		}

		if err != nil {
			log.Lvlf4("%+v got error (%+s) while receiving message", r.id.String(), err)

			if err == ErrClosed || err == ErrEOF || err == ErrTemp {
				// Connection got closed.
				log.Lvl3(r.id.Address, "handleConn with closed connection: stop (dst=", remote.Address, ")")
				return
			}
			// Temporary error, continue.
			log.Lvl3(r.id, "Error with connection", address, "=>", err)
			continue
		}

		if err := r.Dispatch(&packet); err != nil {
			log.Lvl3("Error dispatching:", err)
		}
	}
}

// connection returns the first connection associated with this ServerIdentity.
// If no connection is found, it returns nil.
func (r *Router) connection(sid ServerIdentityID) Conn {
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	arr := r.connections[sid]
	if len(arr) == 0 {
		return nil
	}
	return arr[0]
}

// registerConnection registers a ServerIdentity for a new connection, mapped with the
// real physical address of the connection and the connection itself.
// It uses the networkLock mutex.
func (r *Router) registerConnection(remote *ServerIdentity, c Conn) {
	log.Lvl4(r.address, "Registers", remote.Address)
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	_, okc := r.connections[remote.ID]
	if okc {
		log.Lvl2("Connection already registered", okc)
	}
	r.connections[remote.ID] = append(r.connections[remote.ID], c)
}

func (r *Router) reset() {
	r.closedMut.Lock()
	r.isClosed = false
	r.closedMut.Unlock()
}

func (r *Router) launchHandleRoutine(dst *ServerIdentity, c Conn) {
	r.wg.Add(1)
	go r.handleConn(dst, c)
}

// Close shuts down all network connections and returns once all processing go
// routines are done.
func (r *Router) stopHandling() error {
	// set the isClosed to true
	r.close()

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
	r.closedMut.Lock()
	defer r.closedMut.Unlock()
	r.isClosed = true
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

// Listening returns true if this router is started.
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
	if err := c.Send(context.TODO(), own); err != nil {
		return nil, fmt.Errorf("Error while sending out identity during negotiation:%s", err)
	}
	// Receive the other ServerIdentity
	nm, err := c.Receive(context.TODO())
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

/*// GetStatus is a function that returns the status of all connections managed:*/
//// Connections: ip address of remote host
//// Total: total number of managed connections
//// Packets_Received: #bytes received from all connections
//// Packets_Sent: #bytes sent from all connections
//func (r *router) GetStatus() Status {
//r.connsMut.Lock()
//defer r.connsMut.Unlock()
//m := make(map[string]string)
//nbr := len(r.connections)
//remote := make([]string, nbr)
//iter := 0
//var rx uint64
//var tx uint64
//for _, c := range r.connections {
//remote[iter] = c.Remote()
//rx += c.Rx()
//tx += c.Tx()
//iter = iter + 1
//}
//m["Connections"] = strings.Join(remote, "\n")
//m["Total"] = strconv.Itoa(nbr)
//m["Packets_Received"] = strconv.FormatUint(rx, 10)
//m["Packets_Sent"] = strconv.FormatUint(tx, 10)
//return m
