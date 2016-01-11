/*
Implementation of the Secure Distributed API - main module

Node takes care about
* the network
* pre-parsing incoming packets
* instantiating ProtocolInstances
* passing packets to ProtocolInstances

Basically you can open connections to some addresses, and listen on some address.
Each time a packet is received is passed through ProcessMessages()
When it is a SDAMessage, that means it is destined to a ProtocolInstance, and that dispatching is done in processSDAMessages
To register a new protocol instance just call AddProtocolInstance()
For the dispatching to work, the packet sent must address the right protocolID, the right protocolInstanceID
and the right topologyID it relies on.
*/

package sda

import (
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	. "github.com/satori/go.uuid"
	"golang.org/x/net/context"
	"sync"
	"time"
)

/*
Host is the structure responsible for holding information about the current
 state
*/
type Host struct {
	// Our identity
	Identity *network.Identity
	// The TCPHost which uses identity
	host network.SecureHost
	// and the locks
	networkLock *sync.Mutex
	// The database of identities this host knows
	identities map[UUID]*network.Identity
	// connections maintained by this host
	connections map[UUID]network.SecureConn
	// Our private-key
	private abstract.Secret
	// The suite used for this Host
	suite abstract.Suite
	// slice of received messages - testmode
	networkChan chan network.ApplicationMessage
	// instances linked to their ID and their ProtocolID
	instances map[UUID]ProtocolInstance
	// all trees known to this Host
	trees map[UUID]Tree
	// closed channel to notifiy the connections that we close
	closed chan bool
}

// NewHost starts a new Host that will listen on the network for incoming
// messages. It will store the private-key.
func NewHost(id *network.Identity, pkey abstract.Secret, host network.SecureHost) *Host {
	n := &Host{
		Identity:    id,
		networkLock: &sync.Mutex{},
		connections: make(map[UUID]network.SecureConn),
		identities:  make(map[UUID]*network.Identity),
		host:        host,
		private:     pkey,
		suite:       network.Suite,
		networkChan: make(chan network.ApplicationMessage, 1),
		instances:   make(map[UUID]ProtocolInstance),
		closed:      make(chan bool),
	}
	return n
}

// Start listening for messages coming from parent(up)
// each time a connection request is made, we receive first its identity then
// we handle the message using HandleConn
// NOTE Listen will try each address in the Host identity until one works ;)
func (n *Host) Listen() {
	fn := func(c network.SecureConn) {
		n.registerConnection(c)
		n.handleConn(c)
	}
	errChan := make(chan error)
	var stop bool
	// Try every addresses
	for _, addr := range n.Identity.Addresses {
		if stop {
			break
		}
		go func() {
			err := n.host.Listen(fn)
			if err != nil {
				errChan <- err
			}
		}()
		select {
		// error while listening on this address
		case e := <-errChan:
			dbg.Lvl2("Unable to listen with address", addr, "=>err", e)
			// no error, we listen it's ok !
		case <-time.After(2 * time.Second):
			dbg.Lvl2(addr, "is listening !")
			stop = true
		}
	}
}

// Connect takes an identity where the next Host is
// It will try every addresses in the Identity
func (n *Host) Connect(id *network.Identity) (network.Conn, error) {
	c, err := n.host.Open(*id)
	if err != nil {
		return nil, err
	}
	n.registerConnection(c)
	dbg.Lvl2("Host", "connected to", id.First())
	go n.handleConn(c)
	return c, nil
}

// Close shuts down the listener
func (n *Host) Close() error {
	n.networkLock.Lock()
	err := n.host.Close()
	n.connections = make(map[UUID]network.SecureConn)
	close(n.closed)
	n.networkLock.Unlock()
	return err
}

// SendToIdentity sends to an Identity by trying the differents addresses tied
// to this id
// XXX SHould use that function in public and put SendTo in private
func (n *Host) SendTo(id *network.Identity, msg network.ProtocolMessage) error {
	if _, ok := n.identities[id.ID()]; !ok {
		return fmt.Errorf("SendToIdentity received an non-saved identity")
	}
	var c network.SecureConn
	var ok bool
	// additionnal verification - might be skipped
	n.networkLock.Lock()
	c, ok = n.connections[id.ID()]
	if !ok {
		n.networkLock.Unlock()
		return fmt.Errorf("No connection to this address!", n.connections)
	}
	// we got the connection
	n.networkLock.Unlock()
	sdaMsg := &SDAMessage{
		ProtoID:    Nil,
		InstanceID: Nil,
	}
	b, err := network.MarshalRegisteredType(msg)
	if err != nil {
		return fmt.Errorf("Error marshaling  message: %s", err.Error())
	}
	sdaMsg.DataSlice = b
	sdaMsg.MsgType = network.TypeFromData(msg)
	dbg.Lvl3("Sending data", sdaMsg, "to", c.Remote())
	return c.Send(context.TODO(), sdaMsg)
}

// Receive will return the value of the communication-channel or an error
// if there has been nothing received during 2 seconds.
// XXX Should we do that? Consider a node that takes time to compute or to
// aggregate information from its children, 2 secs is low. Maybe set a higher
// timeout. And also, this is a general timeout, for every connections, do we
// suppose that we must receive something at least from someone every 30 sec ?
// If the Host is in stand-by mode, where it does not participate yet in any
// protocols... ?
// Could we merge that function with handleConn maybe ? so it timeout only on
// opened connections.
func (n *Host) Receive() network.ApplicationMessage {
	data := <-n.networkChan
	if data.MsgType == SDAMessageType {
		sda := data.Msg.(SDAMessage)
		t, msg, _ := network.UnmarshalRegisteredType(sda.DataSlice, data.Constructors)
		sda.MsgType = t
		sda.Data = msg
		data.Msg = sda
		dbg.Lvl3("SDA-Message is:", sda)
	}
	return data
}

// ProcessMessage checks if it is one of the messages for us or dispatch it
// to the corresponding instance.
// Our messages are:
// * SDAMessage - used to communicate between the Hosts
// * RequestTreeID - ask the parent for a given tree
// * SendTree - send the tree to the child
// * RequestPeerListID - ask the parent for a given peerList
// * SendPeerListID - send the tree to the child
func (n *Host) ProcessMessages() {
	for {
		data := <-n.networkChan
		dbg.Lvl3("Message Received:", data)
		switch data.MsgType {
		case SDAMessageType:
			n.processSDAMessage(&data)
		case RequestTreeType:
			tt := data.Msg.(RequestTree).TreeID
			n.SendTo(&data.Identity, tt)
		case SendTreeType:
		case RequestIdentityListType:
		case SendIdentityListType:
		default:
			dbg.Error("Didn't recognize message", data.MsgType)
		}
	}
}

// NetworkChan returns the channel where all messages received go. You can use
// it for testing, or if you want to make your own processing of the messages
func (n *Host) NetworkChan() chan network.ApplicationMessage {
	return n.networkChan
}

// AddProtocolInstance takes a UUID and a ProtocolInstance to be added
// to the map
func (n *Host) AddProtocolInstance(pi ProtocolInstance) {
	n.instances[pi.Id()] = pi
}

var timeOut = 30 * time.Second

// Handle a connection => giving messages to the MsgChans
func (n *Host) handleConn(c network.SecureConn) {
	msgChan := make(chan network.ApplicationMessage)
	errorChan := make(chan error)
	doneChan := make(chan bool)
	go func() {
		for {
			select {
			case <-doneChan:
				return
			default:
				ctx := context.TODO()
				am, err := c.Receive(ctx)
				// So the receiver can know about the error
				am.SetError(err)
				if err != nil {
					errorChan <- err
				} else {
					msgChan <- am
				}
			}
		}
	}()
	id := c.Identity()
	for {
		select {
		case <-n.closed:
			doneChan <- true
		case am := <-msgChan:
			n.networkChan <- am
		case e := <-errorChan:
			if e == network.ErrClosed {
				return
			}
			dbg.Error("Error with connection", id.First(), "=> error", e)
		case <-time.After(timeOut):
			dbg.Error("Timeout with connection", id.First())
		}
	}
}

// Dispatch SDA message looks if we have all the info to rightly dispatch the
// packet such as the protocol id and the topology id and the protocol instance
// id
func (n *Host) processSDAMessage(am *network.ApplicationMessage) error {
	sda := am.Msg.(SDAMessage)
	t, msg, err := network.UnmarshalRegisteredType(sda.DataSlice, network.DefaultConstructors(n.Suite()))
	if err != nil {
		dbg.Error("Error unmarshaling embedded msg in SDAMessage", err)
	}
	// Set the right type and msg
	sda.MsgType = t
	sda.Data = msg
	dbg.Lvl3("Processing SDA-message", sda)
	if !ProtocolExists(sda.ProtoID) {
		return fmt.Errorf("Protocol does not exists")
	}
	ip, ok := n.instances[sda.InstanceID]
	if !ok {
		// XXX What to do here ? create a new instance or just drop ?
		return fmt.Errorf("Instance Protocol not existing YET")
	}
	// Dispatch the message to the right instance !
	ip.Dispatch(&sda)
	return nil
}

// registerConnection registers a Identity for a new connection, mapped with the
// real physical address of the connection and the connection itself
func (n *Host) registerConnection(c network.SecureConn) {
	n.networkLock.Lock()
	id := c.Identity()
	n.connections[id.ID()] = c
	n.identities[id.ID()] = &id
	n.networkLock.Unlock()
}

// Suite returns the suite used by the host
// NOTE for the moment the suite is fixed for the host and any protocols
// instance.
func (n *Host) Suite() abstract.Suite {
	return n.suite
}
