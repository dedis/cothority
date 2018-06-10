package eventlog

import (
	"time"

	"github.com/dedis/cothority/omniledger/darc"
	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/protobuf"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
)

// Client is a structure to communicate with the eventlog service
type Client struct {
	*onet.Client
	roster *onet.Roster
	// ID is the skipchain where events will be logged.
	ID skipchain.SkipBlockID
	// Signers are the Darc signers that will sign events sent with this client.
	Signers []*darc.Signer
	// Darc is the current Darc associated with this skipchain. Use it as a base
	// in case you need to evolve the permissions on the EventLog.
	Darc *darc.Darc
}

// NewClient creates a new client to talk to the eventlog service.
func NewClient(r *onet.Roster) *Client {
	return &Client{
		Client: onet.NewClient(cothority.Suite, ServiceName),
		roster: r,
	}
}

// Init initialises an event logging skipchain. A sucessful call
// updates the ID, Signer and Darc fields of the Client. The new
// skipchain has a Darc that requires one signature from owner.
func (c *Client) Init(owner *darc.Signer, blockInterval time.Duration) error {
	d := darc.NewDarc([]*darc.Identity{owner.Identity()}, []*darc.Identity{}, "_evolve", "_sign", []byte("eventlog owner"))
	if err := d.AddRule("Spawn_eventlog", d.GetEvolutionExpr()); err != nil {
		return err
	}
	msg := &InitRequest{
		Owner:         *d,
		Roster:        *c.roster,
		BlockInterval: blockInterval,
	}
	reply := &InitResponse{}
	if err := c.SendProtobuf(c.roster.List[0], msg, reply); err != nil {
		return err
	}
	c.Darc = d
	c.Signers = []*darc.Signer{owner}
	c.ID = reply.ID
	return nil
}

// A LogID is an opaque unique identifier useful to find a given log message later.
type LogID []byte

// Log asks the service to log events.
func (c *Client) Log(ev ...Event) ([]LogID, error) {
	reply := &LogResponse{}
	tx, err := makeTx(ev, c.Darc.GetBaseID(), c.Signers)
	if err != nil {
		return nil, err
	}
	req := &LogRequest{
		SkipchainID: c.ID,
		Transaction: *tx,
	}
	if err := c.SendProtobuf(c.roster.List[0], req, reply); err != nil {
		return nil, err
	}
	out := make([]LogID, len(tx.Instructions))
	for i := range tx.Instructions {
		out[i] = tx.Instructions[i].ObjectID.Slice()
	}
	return out, nil
}

// GetEvent asks the service to retrieve an event.
func (c *Client) GetEvent(id []byte) (*Event, error) {
	reply := &GetEventResponse{}
	req := &GetEventRequest{
		SkipchainID: c.ID,
		Key:         id,
	}
	if err := c.SendProtobuf(c.roster.List[0], req, reply); err != nil {
		return nil, err
	}
	return &reply.Event, nil
}

func makeTx(msgs []Event, darcID darc.ID, signers []*darc.Signer) (*omniledger.ClientTransaction, error) {
	// We need the identity part of the signatures before
	// calling ToDarcRequest() below, because the identities
	// go into the message digest.
	sigs := make([]darc.Signature, len(signers))
	for i, x := range signers {
		sigs[i].Signer = *(x.Identity())
	}

	instrNonce := omniledger.GenNonce()
	tx := omniledger.ClientTransaction{
		Instructions: make([]omniledger.Instruction, len(msgs)),
	}
	for i, msg := range msgs {
		eventBuf, err := protobuf.Encode(&msg)
		if err != nil {
			return nil, err
		}
		arg := omniledger.Argument{
			Name:  "event",
			Value: eventBuf,
		}
		tx.Instructions[i] = omniledger.Instruction{
			ObjectID: omniledger.ObjectID{
				DarcID:     darcID,
				InstanceID: omniledger.GenNonce(), // TODO figure out how to do the nonce property
			},
			Nonce:  instrNonce,
			Index:  i,
			Length: len(msgs),
			Spawn: &omniledger.Spawn{
				Args:       []omniledger.Argument{arg},
				ContractID: contractName,
			},
			Signatures: append([]darc.Signature{}, sigs...),
		}
	}
	for i := range tx.Instructions {
		darcSigs := make([]darc.Signature, len(signers))
		for j, signer := range signers {
			dr, err := tx.Instructions[i].ToDarcRequest()
			if err != nil {
				return nil, err
			}

			sig, err := signer.Sign(dr.Hash())
			if err != nil {
				return nil, err
			}
			darcSigs[j] = darc.Signature{
				Signature: sig,
				Signer:    *signer.Identity(),
			}
		}
		tx.Instructions[i].Signatures = darcSigs
	}
	return &tx, nil
}
