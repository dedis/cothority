package eventlog

import (
	"github.com/dedis/protobuf"
	"github.com/dedis/student_18_omniledger/omniledger/darc"
	omniledger "github.com/dedis/student_18_omniledger/omniledger/service"

	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/onet.v2"
)

// Client is a structure to communicate with the CoSi
// service
type Client struct {
	*onet.Client
}

// Init initialises an event logging service. On successful initialisation, it
// will respond with a skipchain ID which the client must use when logging
// events.
func (c *Client) Init(r *onet.Roster, msg *InitRequest) (*InitResponse, error) {
	reply := &InitResponse{}
	if err := c.SendProtobuf(r.List[0], msg, reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// LogOne asks the service to log one event.
func (c *Client) LogOne(r *onet.Roster, scID skipchain.SkipBlockID, msg Event, darcID darc.ID, signers ...*darc.Signer) (*LogResponse, error) {
	return c.Log(r, scID, []Event{msg}, darcID, signers...)
}

// Log asks the service to log events.
func (c *Client) Log(r *onet.Roster, scID skipchain.SkipBlockID, msgs []Event, darcID darc.ID, signers ...*darc.Signer) (*LogResponse, error) {
	reply := &LogResponse{}
	tx, err := makeTx(msgs, darcID, signers...)
	if err != nil {
		return nil, err
	}
	if err := c.SendProtobuf(r.List[0], tx, reply); err != nil {
		return nil, err
	}
	return reply, nil
}

func makeTx(msgs []Event, darcID darc.ID, signers ...*darc.Signer) (*omniledger.ClientTransaction, error) {
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
		}
	}
	for i := range tx.Instructions {
		darcSigs := make([]darc.Signature, len(signers))
		for j, signer := range signers {
			sig, err := signer.Sign(tx.Instructions[i].Hash())
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
