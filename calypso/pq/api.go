package pq

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
	"sync"
)

type Client struct {
	bcClient *byzcoin.Client
	c        *onet.Client
}

func NewClient(byzcoin *byzcoin.Client) *Client {
	return &Client{bcClient: byzcoin, c: onet.NewClient(
		cothority.Suite, ServiceName)}
}

func (c *Client) VerifyWriteAll(roster *onet.Roster, write *Write,
	shares []*share.PriShare, rands [][]byte) []*VerifyWriteReply {
	var wg sync.WaitGroup
	replies := make([]*VerifyWriteReply, len(shares))
	for i, who := range roster.List {
		wg.Add(1)
		go func(c *Client, who *network.ServerIdentity, w *Write,
			sh *share.PriShare, r []byte, idx int, rps []*VerifyWriteReply) {
			defer wg.Done()
			reply, _ := c.VerifyWrite(who, w, sh, r, idx)
			rps[idx] = reply
		}(c, who, write, shares[i], rands[i], i, replies)
	}
	wg.Wait()
	return replies
}

func (c *Client) VerifyWrite(who *network.ServerIdentity, write *Write,
	share *share.PriShare, rand []byte, idx int) (reply *VerifyWriteReply,
	err error) {
	reply = &VerifyWriteReply{}
	err = c.c.SendProtobuf(who, &VerifyWrite{
		Idx:   idx,
		Write: write,
		Share: share,
		Rand:  rand,
	}, reply)
	return reply, cothority.ErrorOrNil(err, "Sending VerifyWrite request")
}

func (c *Client) AddWrite(write *Write, sigs map[int][]byte, t int,
	signer darc.Signer, signerCtr uint64, darc darc.Darc,
	wait int) (reply *WriteReply, err error) {
	reply = &WriteReply{}
	req := WriteRequest{
		Threshold: t,
		Write:     *write,
		Sigs:      sigs,
	}
	reqBuf, err := protobuf.Encode(&req)
	if err != nil {
		return nil, xerrors.Errorf("encoding write request: %v", err)
	}
	ctx := byzcoin.NewClientTransaction(byzcoin.CurrentVersion,
		byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(darc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: ContractPQWriteID,
				Args: byzcoin.Arguments{{
					Name: "writereq", Value: reqBuf}},
			},
			SignerCounter: []uint64{signerCtr},
		},
	)
	//Sign the transaction
	err = ctx.FillSignersAndSignWith(signer)
	if err != nil {
		return nil, xerrors.Errorf("signing txn: %v", err)
	}
	reply.InstanceID = ctx.Instructions[0].DeriveID("")
	//Delegate the work to the byzcoin client
	reply.AddTxResponse, err = c.bcClient.AddTransactionAndWait(ctx, wait)
	if err != nil {
		return nil, xerrors.Errorf("adding txn: %v", err)
	}
	return reply, err
}

func (c *Client) SpawnDarc(signer darc.Signer, signerCtr uint64,
	controlDarc darc.Darc, spawnDarc darc.Darc, wait int) (
	reply *byzcoin.AddTxResponse, err error) {
	darcBuf, err := spawnDarc.ToProto()
	if err != nil {
		return nil, xerrors.Errorf("serializing darc to protobuf: %v", err)
	}

	ctx := byzcoin.NewClientTransaction(byzcoin.CurrentVersion,
		byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(controlDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: byzcoin.ContractDarcID,
				Args: []byzcoin.Argument{{
					Name:  "darc",
					Value: darcBuf,
				}},
			},
			SignerCounter: []uint64{signerCtr},
		},
	)
	err = ctx.FillSignersAndSignWith(signer)
	if err != nil {
		return nil, xerrors.Errorf("signing txn: %v", err)
	}

	reply, err = c.bcClient.AddTransactionAndWait(ctx, wait)
	return reply, cothority.ErrorOrNil(err, "adding txn")
}
