package calypso

import (
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/onet"
	"github.com/dedis/protobuf"
)

// calypso.Client is a class to communicate to the calypso service.
type Client struct {
	byzcoin  *byzcoin.Client
	onet     *onet.Client
	ltsReply *CreateLTSReply
}

// NewClient instantiates a new calypso.Client.
// It takes as input an "initialized" byzcoin client
func NewClient(byzcoin *byzcoin.Client) *Client {
	return &Client{byzcoin: byzcoin, onet: onet.NewClient(
		cothority.Suite, ServiceName)}
}

// CreateLTS creates a random LTSID that can be used to reference
// the LTS group created.
func (c *Client) CreateLTS() (reply *CreateLTSReply, err error) {
	reply = &CreateLTSReply{}
	err = c.onet.SendProtobuf(c.byzcoin.Roster.List[0], &CreateLTS{
		Roster: c.byzcoin.Roster,
		BCID:   c.byzcoin.ID,
	}, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// CreateLTS creates a random LTSID that can be used to reference
// the LTS group created.
func (c *Client) DecryptKey(dkr *DecryptKey) (reply *DecryptKeyReply, err error) {
	reply = &DecryptKeyReply{}
	err = c.onet.SendProtobuf(c.byzcoin.Roster.List[0], dkr, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// AddWrite creates a Write Instance by adding a transaction on the byzcoin client
func (c *Client) AddWrite(data []byte, signer darc.Signer) (
	reply *WriteReply, err error) {
	reply = &WriteReply{}
	gDarc, err := c.byzcoin.GetGenDarc()
	if err != nil {
		return nil, err
	}
	write := NewWrite(cothority.Suite, c.ltsReply.LTSID, gDarc.GetBaseID(),
		c.ltsReply.X, data)
	writeBuf, err := protobuf.Encode(write)
	if err != nil {
		return nil, err
	}
	ctx := byzcoin.ClientTransaction{
		Instructions: byzcoin.Instructions{{
			InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
			Nonce:      byzcoin.Nonce{},
			Index:      0,
			Length:     1,
			Spawn: &byzcoin.Spawn{
				ContractID: ContractWriteID,
				Args: byzcoin.Arguments{{
					Name: "write", Value: writeBuf}},
			},
		}},
	}
	//Sign the transaction
	err = ctx.Instructions[0].SignBy(gDarc.GetID(), signer)
	if err != nil {
		return nil, err
	}
	reply.InstanceID = ctx.Instructions[0].DeriveID("")
	//Delegate the work to the byzcoin client
	reply.AddTxResponse, err = c.byzcoin.AddTransaction(ctx)
	if err != nil {
		return nil, err
	}
	return reply, err
}

func (c *Client) AddRead(write *byzcoin.Proof, signer darc.Signer) (
	reply *ReadReply, err error) {
	var readBuf []byte
	read := &Read{
		Write: byzcoin.NewInstanceID(write.InclusionProof.Key),
		Xc:    signer.Ed25519.Point,
	}
	reply = &ReadReply{}
	readBuf, err = protobuf.Encode(read)
	if err != nil {
		return nil, err
	}
	gDarc, err := c.byzcoin.GetGenDarc()
	if err != nil {
		return nil, err
	}
	ctx := byzcoin.ClientTransaction{
		Instructions: byzcoin.Instructions{{
			InstanceID: byzcoin.NewInstanceID(write.InclusionProof.Key),
			Nonce:      byzcoin.Nonce{},
			Index:      0,
			Length:     1,
			Spawn: &byzcoin.Spawn{
				ContractID: ContractReadID,
				Args:       byzcoin.Arguments{{Name: "read", Value: readBuf}},
			},
		}},
	}
	err = ctx.Instructions[0].SignBy(gDarc.GetID(), signer)
	reply.InstanceID = ctx.Instructions[0].DeriveID("")
	if err != nil {
		return nil, err
	}
	reply.AddTxResponse, err = c.byzcoin.AddTransaction(ctx)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
