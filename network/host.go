package network

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/dedis/cothority/log"
)

// Host is the central part of the network library. It listens to incoming
// connections, open up new connections, and dispatch incoming/outgoing
//messages to the right destination.
type Host interface {
	// Start is used to start the listening of incoming connections and any
	// others operations the Host might need (queuing message routine,etc)
	// XXX Blocking or not ?
	// All implementation must make sure that Start() is a BLOCKING call.
	Start()
	// Stop is used to stop all operations done by this Host. It will stop the
	// listening and all active communications.
	Stop() error

	// Send takes a destination server identity and a message. The host is
	// responsible to translate from the ServerIdentity to the actual network
	// connection and send the message through it.
	Send(sid *ServerIdentity, msg Body)
	// Dispatcher makes possible the dispatching of any messages coming from
	// the network to the right destination.
	Dispatcher

	// newConn is called each time a message is destined to a remote host where
	// no connections have been established already. It is also used in case of
	// reconnections.
	//newConn(sid *ServerIdentity) (Conn, error)

	// XXX See what can be done to reduce all of these methods
	//GetStatus()
	Tx() uint64
	Rx() uint64
}

// TCPHost implements the Host interface. It uses a TCPListener and a
// BlockingDispatcher. For each incoming or outgoing connections, it also
// proceeds to the server identity exchange with the remote
type TCPHost struct {
	// What is our id
	id *ServerIdentity

	// TCPListener to accept new incoming connections of type PlainTCP
	*TCPListener

	// router handles all the connection management
	*router
}

// NewTCPHost takes a server identity and returns a Host implementation using
// plain tcp connections.
func NewTCPHost(own *ServerIdentity) *TCPHost {
	tcpHost := &TCPHost{
		id:          own,
		TCPListener: NewTCPListener(),
	}
	tcpHost.router = newRouter(own, tcpHost.newConn)
	return tcpHost
}

func (t *TCPHost) Start() {
	// The function given to the listener  does the exchange of ServerIdentity
	// and pass the connection along to the router.
	t.TCPListener.Listen(t.id.Address.NetworkAddress(), func(c Conn) {
		dst, err := t.exchangeServerIdentity(c)
		if err != nil {
			debug.PrintStack()
			if err := c.Close(); err != nil {
				log.Error("Couldn't close secure connection:",
					err)
			}
			return
		}
		// pass it along the router
		t.router.registerConnection(dst, c)
		t.router.handleConn(dst, c)
	})
}

func (t *TCPHost) Stop() error {
	err := t.TCPListener.Stop()
	err2 := t.router.close()
	t.router.reset()
	if err != nil {
		return err
	} else if err2 != nil {
		return err2
	}
	return nil
}

// newConn implements the Host interface by opening a PlainTCP connection to the
// destionation, then proceeds to the exchange of identity. If all is correct,
// it returns the connection.
func (t *TCPHost) newConn(sid *ServerIdentity) (Conn, error) {
	switch sid.Address.ConnType() {
	case PlainTCP:
		c, err := NewTCPConn(sid.Address.NetworkAddress())
		if err != nil {
			return nil, err
		}
		if err = t.negotiateOpen(c, sid); err != nil {
			return nil, err
		}
		return c, nil
	}
	return nil, fmt.Errorf("TCPHost can't handle this type of connection: %s", sid.Address.ConnType())
}

// exchangeServerIdentity takes a fresh new conn issued by the listener and
// proceed to the exchanges of the server identities of both parties. It returns
// the ServerIdentity of the remote party.
func (t *TCPHost) exchangeServerIdentity(c Conn) (*ServerIdentity, error) {
	// Send our ServerIdentity to the remote endpoint
	log.Lvl4(t.id.Address, "Sending our identity to", c.Remote())

	if err := c.Send(context.TODO(), t.id); err != nil {
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
	log.Lvl4(t.id.Address, "Identity exchange complete with ", e.Address)
	return &e, nil
}

// negotiateOpen takes a fresh issued new Conn and the supposed destination's
// ServerIdentity. It proceeds to the exchange of identity and verifies that we
// are correctly dealing with the right one.
// NOTE: This version is non secure at all, it's just a simple verification.
// Later will come the signing part,etc.
func (t *TCPHost) negotiateOpen(c Conn, e *ServerIdentity) error {
	var err error
	var dst *ServerIdentity
	if dst, err = t.exchangeServerIdentity(c); err != nil {
		return err
	}
	// verify the ServerIdentity if its the same we are supposed to connect
	if dst.ID != e.ID {
		log.Lvl3(fmt.Sprintf("Wanted to connect to %s (%x) but got %s (%x)", e.Address, e.ID, dst.Address, dst.ID))
		log.Lvl4("IDs not the same", log.Stack())
		return errors.New("Warning: ServerIdentity received during negotiation is wrong.")
	}
	return nil
}
