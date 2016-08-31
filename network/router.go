package network

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
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
	// Dispatcher is used to dispatch incoming message to the right recipient
	Dispatcher

	host Host
	// connections keeps track of all active connections with the translation
	connections map[ServerIdentityID]Conn
	connsMut    sync.Mutex

	// boolean flag indicating that the router is already clos{ing,ed}
	isClosed  bool
	closedMut sync.Mutex

	// we wait that all handleConn routines are done
	handleConnQuit chan bool
}

// NewRouter returns a fresh Router giving its identity, and the host we want to
// use.
func NewRouter(own *ServerIdentity, h Host) *Router {
	r := &Router{
		id:             own,
		connections:    make(map[ServerIdentityID]Conn),
		host:           h,
		Dispatcher:     NewBlockingDispatcher(),
		handleConnQuit: make(chan bool),
	}
	own.Address = h.Address()
	return r
}

func (r *Router) Start() {
	// The function given to the listener  does the exchange of ServerIdentity
	// and pass the connection along to the router.
	err := r.host.Listen(func(c Conn) {
		dst, err := r.exchangeServerIdentity(c)
		if err != nil {
			log.Error("ExchangeServerIdentity failed:", err)
			debug.PrintStack()
			if err := c.Close(); err != nil {
				log.Error("Couldn't close secure connection:",
					err)
			}
			return
		}
		// pass it along
		r.registerConnection(dst, c)
		r.handleConn(dst, c)
	})
	if err != nil {
		log.Error("Error listening:", err)
	}
}

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
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	// connect function to connect + exchange + register + handle
	var connect = func() (Conn, error) {
		c, err := r.host.Connect(e.Address)

		if err != nil {
			return nil, err
		}
		if err := r.negotiateOpen(e, c); err != nil {
			return nil, err
		}

		r.connections[e.ID] = c
		go r.handleConn(e, c)
		return c, nil
	}

	if msg == nil {
		return errors.New("Can't send nil-packet")
	}

	c, ok := r.connections[e.ID]
	if !ok {
		var err error
		c, err = connect()
		if err != nil {
			return err
		}
	}

	log.Lvlf4("%s sends to %s msg: %+v", r.id.Address, e, msg)
	var err error
	err = c.Send(context.TODO(), msg)
	if err != nil {
		log.Lvl2("Couldn't send to", e, ":", err, "trying again")
		c, err := connect()
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

// handleConn is the main routine for a connection to wait for incoming
// messages.
func (r *Router) handleConn(remote *ServerIdentity, c Conn) {
	address := c.Remote()
	log.Lvl3(r.id.Address, "Handling new connection to ", remote.Address)
	for {
		ctx := context.TODO()
		packet, err := c.Receive(ctx)
		// So the receiver can know about the error
		packet.SetError(err)
		packet.From = address
		packet.ServerIdentity = remote
		log.Lvl5(r.id.Address, "Got message", packet)
		if err != nil {
			// something went wrong on this connection
			r.closedMut.Lock()
			log.Lvlf4("%+v got error (%+s) while receiving message (isClosed=%+v)",
				r.id.String(), err, r.isClosed)

			if r.isClosed {
				// request to finish handling conn
				log.Lvl3(r.id.Address, "handleConn is asked to stop for", remote.Address)
				r.closedMut.Unlock()
				r.handleConnQuit <- true
			} else if err == ErrClosed || err == ErrEOF || err == ErrTemp {
				// remote connection closed
				r.closedMut.Unlock()
				r.closeConnection(remote, c)
				log.Lvl3(r.id.Address, "handleConn with closed connection: stop (dst=", remote.Address, ")")
			} else {
				// weird error let's try again
				r.closedMut.Unlock()
				log.Error(r.id, "Error with connection", address, "=>", err)
				continue
			}
			return
		}

		r.closedMut.Lock()
		if !r.isClosed {
			// XXX Let's get rid of this processMessages: It's a blocking
			// channel, so only one connection is dispatching message at a time.
			// The layer of indirection does not bring any advantages at all...
			//log.Lvl5(t.workingAddress, "Send message to networkChan", len(t.networkChan))
			//t.networkChan <- packet
			if err := r.Dispatch(&packet); err != nil {
				log.Lvl3("Error dispatching:", err)
			}
		} else {
			// signal we are done with this go routine.
			r.handleConnQuit <- true
			r.closedMut.Unlock()
			log.Lvl3(r.id.Address, "leaving out handleConn for", remote.Address)
			return
		}
		r.closedMut.Unlock()
	}
}

// registerConnection registers an ServerIdentity for a new connection, mapped with the
// real physical address of the connection and the connection itself
// it locks (and unlocks when done):  networkLock
func (r *Router) registerConnection(remote *ServerIdentity, c Conn) {
	log.Lvl4("Registers", remote.Address)
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	_, okc := r.connections[remote.ID]
	if okc {
		log.Lvl3("Connection already registered", okc)
	}
	r.connections[remote.ID] = c
}

// closeConnection closes a connection and removes it from the connections-map
// Calling this method will borrow the connsMut lock.
func (r *Router) closeConnection(remote *ServerIdentity, c Conn) error {
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	log.Lvl4("Closing connection", c, c.Remote(), c.Local())
	err := c.Close()
	delete(r.connections, remote.ID)
	return err
}

func (r *Router) connection(id *ServerIdentity) Conn {
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	return r.connections[id.ID]
}

func (r *Router) reset() {
	r.closedMut.Lock()
	r.isClosed = false
	r.closedMut.Unlock()
}

// Close shuts down all network connections and returns once all processing go
// routines are done.
func (r *Router) stopHandling() error {
	r.closedMut.Lock()
	if r.isClosed {
		r.closedMut.Unlock()
		return errors.New("Already closing")
	}
	r.isClosed = true
	r.closedMut.Unlock()

	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	lenConn := len(r.connections)
	for id, c := range r.connections {
		delete(r.connections, id)
		if err := c.Close(); err != nil {
			return err
		}
	}
	// wait for all handleConn to finish
	var finished int
	for finished < lenConn {
		<-r.handleConnQuit
		finished++
	}
	return nil
}

// Tx implements monitor/CounterIO
// It returns the Tx for all connections managed by this router
func (r *Router) Tx() uint64 {
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	var tx uint64
	for _, c := range r.connections {
		tx += c.Tx()
	}
	return tx
}

// Rx implements monitor/CounterIO
// It returns the Rx for all connections managed by this router
func (r *Router) Rx() uint64 {
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	var rx uint64
	for _, c := range r.connections {
		rx += c.Rx()
	}
	return rx
}

func (r *Router) Listening() bool {
	return r.host.Listening()
}

// exchangeServerIdentity takes a fresh new conn issued by the listener and
// proceed to the exchanges of the server identities of both parties. It returns
// the ServerIdentity of the remote party.
func (h *Router) exchangeServerIdentity(c Conn) (*ServerIdentity, error) {
	return exchangeServerIdentity(h.id, c)
}

// negotiateOpen takes a fresh issued new Conn and the supposed destination's
// ServerIdentity. It proceeds to the exchange of identity and verifies that
// we are correctly dealing with the right remote party.
// NOTE: This version is non secure at all, it's just a simple verification.
// Later will come the signing part,etc.
func (h *Router) negotiateOpen(e *ServerIdentity, c Conn) error {
	return negotiateOpen(h.id, e, c)
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
