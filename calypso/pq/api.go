package pq

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
	"sync"
)

type Client struct {
	//bcClient *byzcoin.Client
	c *onet.Client
}

func NewClient() *Client {
	return &Client{c: onet.NewClient(cothority.Suite, ServiceName)}
	//return &Client{bcClient: byzcoin, c: onet.NewClient(
	//	cothority.Suite, ServiceName)}
}

func (c *Client) VerifyWriteAll(roster *onet.Roster, write *Write,
	shares []*share.PriShare, rands [][]byte) []*VerifyWriteReply {
	var wg sync.WaitGroup
	replies := make([]*VerifyWriteReply, len(shares))
	for i, who := range roster.List {
		wg.Add(1)
		go func(c *Client, who *network.ServerIdentity, wr *Write,
			sh *share.PriShare, r []byte, idx int, rps []*VerifyWriteReply) {
			defer wg.Done()
			reply, _ := c.VerifyWrite(who, wr, sh, r, idx)
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
