package calypso

import (
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/protobuf"
)

// Client is a class to communicate to the calypso service.
type Client struct {
	bcClient *byzcoin.Client
	c        *onet.Client
	ltsReply *CreateLTSReply
}

// WriteReply is returned upon successfully spawning a Write instance.
type WriteReply struct {
	*byzcoin.AddTxResponse
	byzcoin.InstanceID
}

// ReadReply is is returned upon successfully spawning a Read instance.
type ReadReply struct {
	*byzcoin.AddTxResponse
	byzcoin.InstanceID
}

// NewClient instantiates a new Client.
// It takes as input an "initialized" byzcoin client
// with an already created ledger
func NewClient(byzcoin *byzcoin.Client) *Client {
	return &Client{bcClient: byzcoin, c: onet.NewClient(
		cothority.Suite, ServiceName)}
}

// CreateLTS creates a random LTSID that can be used to reference
// the LTS group created.
func (c *Client) CreateLTS() (reply *CreateLTSReply, err error) {
	reply = &CreateLTSReply{}
	err = c.c.SendProtobuf(c.bcClient.Roster.List[0], &CreateLTS{
		Roster: c.bcClient.Roster,
		BCID:   c.bcClient.ID,
	}, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// DecryptKey takes as input Read- and Write- Proofs. It verifies that
// the read/write requests match and then re-encrypts the secret
// given the public key information of the reader.
func (c *Client) DecryptKey(dkr *DecryptKey) (reply *DecryptKeyReply, err error) {
	reply = &DecryptKeyReply{}
	err = c.c.SendProtobuf(c.bcClient.Roster.List[0], dkr, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (c *Client) DecryptKeyBatch(dkb *DKBatch) (reply *DKBatchReply, err error) {
	reply = &DKBatchReply{}
	err = c.c.SendProtobuf(c.bcClient.Roster.List[0], dkb, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// WaitProof calls the byzcoin client's wait proof
func (c *Client) WaitProof(id byzcoin.InstanceID, interval time.Duration,
	value []byte) (*byzcoin.Proof, error) {
	return c.bcClient.WaitProof(id, interval, value)
}

// AddWrite creates a Write Instance by adding a transaction on the byzcoin client.
// Input:
//   - write - A Write structure
//   - signer - The data owner who will sign the transaction
//   - darc - The darc governing this instance
//   - wait - The number of blocks to wait -- 0 means no wait
//
// Output:
//   - reply - WriteReply containing the transaction response and instance id
//	 - err - Error if any, nil otherwise.
func (c *Client) AddWrite(write *Write, signer darc.Signer, darc darc.Darc,
	wait int) (
	reply *WriteReply, err error) {
	reply = &WriteReply{}
	if err != nil {
		return nil, err
	}
	writeBuf, err := protobuf.Encode(write)
	if err != nil {
		return nil, err
	}
	ctx := byzcoin.ClientTransaction{
		Instructions: byzcoin.Instructions{{
			InstanceID: byzcoin.NewInstanceID(darc.GetBaseID()),
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
	err = ctx.Instructions[0].SignBy(darc.GetID(), signer)
	if err != nil {
		return nil, err
	}
	reply.InstanceID = ctx.Instructions[0].DeriveID("")
	//Delegate the work to the byzcoin client
	reply.AddTxResponse, err = c.bcClient.AddTransactionAndWait(ctx, wait)
	if err != nil {
		return nil, err
	}
	return reply, err
}

// AddRead creates a Read Instance by adding a transaction on the byzcoin client.
// Input:
//   - proof - A ByzCoin proof of the Write Operation.
//   - signer - The data owner who will sign the transaction
//   - darc - The darc governing this instance
//   - wait - The number of blocks to wait -- 0 means no wait
//
// Output:
//   - reply - ReadReply containing the transaction response and instance id
//	 - err - Error if any, nil otherwise.
func (c *Client) AddRead(proof *byzcoin.Proof, signer darc.Signer,
	darc darc.Darc, wait int) (
	reply *ReadReply, err error) {
	var readBuf []byte
	read := &Read{
		Write: byzcoin.NewInstanceID(proof.InclusionProof.Key),
		Xc:    signer.Ed25519.Point,
	}
	reply = &ReadReply{}
	readBuf, err = protobuf.Encode(read)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	ctx := byzcoin.ClientTransaction{
		Instructions: byzcoin.Instructions{{
			InstanceID: byzcoin.NewInstanceID(proof.InclusionProof.Key),
			Nonce:      byzcoin.Nonce{},
			Index:      0,
			Length:     1,
			Spawn: &byzcoin.Spawn{
				ContractID: ContractReadID,
				Args:       byzcoin.Arguments{{Name: "read", Value: readBuf}},
			},
		}},
	}
	err = ctx.Instructions[0].SignBy(darc.GetID(), signer)
	reply.InstanceID = ctx.Instructions[0].DeriveID("")
	if err != nil {
		return nil, err
	}
	reply.AddTxResponse, err = c.bcClient.AddTransactionAndWait(ctx, wait)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

//func (c *Client) AddReadBatch(proofs []*byzcoin.Proof, signer []darc.Signer,
//darc []darc.Darc, wait int) (
//reply *BatchReadReply, err error) {
func (c *Client) AddReadBatch(batchData []*BatchData, wait int) (
	reply *BatchReadReply, err error) {
	w := 0
	sz := len(batchData)
	replies := make([]BRReply, sz)
	for i, bd := range batchData {
		if bd != nil {
			var readBuf []byte
			read := &Read{
				Write: byzcoin.NewInstanceID(bd.Proof.InclusionProof.Key),
				Xc:    bd.Signer.Ed25519.Point,
			}
			readBuf, err = protobuf.Encode(read)
			if err != nil {
				return nil, err
			}
			ctx := byzcoin.ClientTransaction{
				Instructions: byzcoin.Instructions{{
					InstanceID: byzcoin.NewInstanceID(bd.Proof.InclusionProof.Key),
					Nonce:      byzcoin.Nonce{},
					Index:      0,
					Length:     1,
					Spawn: &byzcoin.Spawn{
						ContractID: ContractReadID,
						Args:       byzcoin.Arguments{{Name: "read", Value: readBuf}},
					},
				}},
			}
			err = ctx.Instructions[0].SignBy(bd.Darc.GetID(), bd.Signer)
			if err != nil {
				log.Errorf("Cannot sign transaction: %v", err)
			} else {
				if i == sz-1 {
					w = wait
				}
				_, err := c.bcClient.AddTransactionAndWait(ctx, w)
				if err != nil {
					log.Errorf("Cannot add transaction: %v", err)
				} else {
					replies[i] = BRReply{ID: ctx.Instructions[0].DeriveID(""), Valid: true}
				}
			}
		}
	}
	reply = &BatchReadReply{
		Replies: replies,
	}
	return reply, nil
}

// SpawnDarc spawns a Darc Instance by adding a transaction on the byzcoin client.
// Input:
//   - signer - The signer authorizing the spawn of this darc (calypso "admin")
//   - controlDarc - The darc governing this spawning
//	 - spawnDarc - The darc to be spawned
//   - wait - The number of blocks to wait -- 0 means no wait
//
// Output:
//   - reply - AddTxResponse containing the transaction response
//	 - err - Error if any, nil otherwise.
func (c *Client) SpawnDarc(signer darc.Signer,
	controlDarc darc.Darc, spawnDarc darc.Darc, wait int) (
	reply *byzcoin.AddTxResponse, err error) {
	reply = &byzcoin.AddTxResponse{}
	if err != nil {
		return nil, err
	}
	darcBuf, err := spawnDarc.ToProto()
	if err != nil {
		return nil, err
	}

	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: byzcoin.NewInstanceID(controlDarc.GetBaseID()),
			Nonce:      byzcoin.GenNonce(),
			Index:      0,
			Length:     1,
			Spawn: &byzcoin.Spawn{
				ContractID: byzcoin.ContractDarcID,
				Args: []byzcoin.Argument{{
					Name:  "darc",
					Value: darcBuf,
				}},
			},
		}},
	}
	err = ctx.Instructions[0].SignBy(controlDarc.GetBaseID(), signer)
	if err != nil {
		return nil, err
	}
	return c.bcClient.AddTransactionAndWait(ctx, wait)
}
