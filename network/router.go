package network

import (
	"context"
	"errors"
	"sync"

	"github.com/dedis/cothority/log"
)

// router is responsible to Send and Dispatch incoming/outgoing messages
// to/from connections. It implements the Send and Dispatcher part of the
// Host interface since it is a common part between all implementations of Hosts.
// Usually, you embed a router in a Host implementation to not having to handle
// all the common part.
type router struct {
	// id is our own ServerIdentity, mostly there for logging/debugging purposes
	id *ServerIdentity
	Dispatcher
	// newConn is used to create connections if we don't have one already for
	// that ServerIdentity or to re-connect to a failed connection.
	newConn func(*ServerIdentity) (Conn, error)
	// connections keeps track of all active connections
	connections map[ServerIdentityID]Conn
	connsMut    sync.Mutex

	// boolean flag indicating that the router is already clos{ing,ed}
	isClosed  bool
	closedMut sync.Mutex
}

func newRouter(own *ServerIdentity, newConn func(sid *ServerIdentity) (Conn, error)) *router {
	return &router{
		id:          own,
		connections: make(map[ServerIdentityID]Conn),
		newConn:     newConn,
		Dispatcher:  NewBlockingDispatcher(),
	}
}

// Send sends to an ServerIdentity without wrapping the msg into a SDAMessage
func (r *router) Send(e *ServerIdentity, msg Body) error {
	r.connsMut.Lock()
	defer r.connsMut.Unlock()

	if msg == nil {
		return errors.New("Can't send nil-packet")
	}

	c, ok := r.connections[e.ID]
	if !ok {
		var err error
		c, err = r.newConn(e)
		if err != nil {
			return err
		}
		go r.handleConn(e, c)
	}

	log.Lvlf4("%s sends to %s msg: %+v", r.id.Address, e, msg)
	var err error
	err = c.Send(context.TODO(), msg)
	if err != nil {
		log.Lvl2("Couldn't send to", e, ":", err, "trying again")
		delete(r.connections, e.ID)
		c, err = r.newConn(e)
		if err != nil {
			return err
		}
		go r.handleConn(e, c)
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
func (r *router) handleConn(remote *ServerIdentity, c Conn) {
	r.registerConnection(remote, c)
	address := c.Remote()
	for {
		ctx := context.TODO()
		packet, err := c.Receive(ctx)
		// So the receiver can know about the error
		packet.SetError(err)
		packet.From = address
		log.Lvl5(r.id.Address, "Got message", packet)
		if err != nil {
			// something went wrong on this connection
			r.closedMut.Lock()
			log.Lvlf4("%+v got error (%+s) while receiving message (isClosed=%+v)",
				r.id.String(), err, r.isClosed)
			r.closedMut.Unlock()
			if err == ErrClosed || err == ErrEOF || err == ErrTemp {
				log.Lvl4(r.id, c.Remote(), "quitting handleConn for-loop", err)
				r.closeConnection(remote, c)
				return
			}
			log.Error(r.id, "Error with connection", address, "=>", err)
		}

		r.closedMut.Lock()
		if !r.isClosed {
			// XXX Let's get rid of this processMessages: It's a blocking
			// channel, so only one connection is dispatching message at a time.
			// The layer of indirection does not bring any advantages at all...
			//log.Lvl5(t.workingAddress, "Send message to networkChan", len(t.networkChan))
			//t.networkChan <- packet
			if err := r.Dispatch(&packet); err != nil {
				log.Print("Error dispatching:", err)
			}
		} else {
			r.closeConnection(remote, c)
			r.closedMut.Unlock()
			return
		}
		r.closedMut.Unlock()
	}
}

// closeConnection closes a connection and removes it from the connections-map
// Calling this method will borrow the connsMut lock.
func (r *router) closeConnection(remote *ServerIdentity, c Conn) error {
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	log.Lvl4("Closing connection", c, c.Remote(), c.Local())
	err := c.Close()
	delete(r.connections, remote.ID)
	return err
}

// registerConnection registers an ServerIdentity for a new connection, mapped with the
// real physical address of the connection and the connection itself
// it locks (and unlocks when done):  networkLock
func (r *router) registerConnection(remote *ServerIdentity, c Conn) {
	log.Lvl4("Registers", remote.Address)
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	_, okc := r.connections[remote.ID]
	if okc {
		log.Lvl3("Connection already registered", okc)
	}
	r.connections[remote.ID] = c
}

// Close shuts down all network connections
func (r *router) close() error {
	r.closedMut.Lock()
	defer r.closedMut.Unlock()
	if r.isClosed {
		return errors.New("Already closing")
	}
	r.isClosed = true
	for _, c := range r.connections {
		if err := c.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Tx implements monitor/CounterIO
// It returns the Tx for all connections managed by this router
func (r *router) Tx() uint64 {
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
func (r *router) Rx() uint64 {
	r.connsMut.Lock()
	defer r.connsMut.Unlock()
	var rx uint64
	for _, c := range r.connections {
		rx += c.Rx()
	}
	return rx
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
/*}*/
