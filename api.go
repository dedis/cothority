package ocs

/*
The api.go defines the methods that can be called from the outside. Most
of the methods will take a roster so that the service knows which nodes
it should work with.

This part of the service runs on the client or the app.
*/

import (
	"errors"

	"github.com/dedis/onchain-secrets/protocol"
	"gopkg.in/dedis/cothority.v1/skipchain"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/network"
)

// Client is a structure to communicate with the CoSi
// service
type Client struct {
	*onet.Client
	sbc *skipchain.Client
}

// NewClient instantiates a new cosi.Client
func NewClient() *Client {
	return &Client{
		Client: onet.NewClient(ServiceName),
		sbc:    skipchain.NewClient(),
	}
}

// CreateSkipchain creates a new OCS-skipchain using the roster r. It returns:
//  - ocs-skipchain, error
func (c *Client) CreateSkipchain(r *onet.Roster) (ocs *skipchain.SkipBlock,
	cerr onet.ClientError) {
	req := &CreateSkipchainsRequest{
		Roster: r,
	}
	reply := &CreateSkipchainsReply{}
	cerr = c.SendProtobuf(r.RandomServerIdentity(), req, reply)
	ocs = reply.OCS
	return
}

// WriteRequest pushes a new block on the skipchain with the encrypted file
// on it.
func (c *Client) WriteRequest(ocs *skipchain.SkipBlock, encData []byte, symKey []byte, readList []abstract.Point) (sb *skipchain.SkipBlock,
	cerr onet.ClientError) {
	if len(encData) > 1e7 {
		return nil, onet.NewClientErrorCode(ErrorParameter, "Cannot store files bigger than 10MB")
	}

	requestShared := &SharedPublicRequest{Genesis: ocs.SkipChainID()}
	shared := &SharedPublicReply{}
	cerr = c.SendProtobuf(ocs.Roster.RandomServerIdentity(), requestShared, shared)
	if cerr != nil {
		return
	}

	U, Cs := protocol.EncodeKey(network.Suite, shared.X, symKey)
	wr := &WriteRequest{
		Write: &DataOCSWrite{
			File:    encData,
			U:       U,
			Cs:      Cs,
			Readers: []byte{},
		},
		Readers: &DataOCSReaders{
			ID:      []byte{},
			Readers: readList,
		},
		OCS: ocs.SkipChainID(),
	}
	reply := &WriteReply{}
	cerr = c.SendProtobuf(ocs.Roster.RandomServerIdentity(), wr, reply)
	sb = reply.SB
	return
}

// ReadRequest asks the ocs-skipchain to add a block giving access to 'reader'
// for the file that references the skipblock it is stored in.
func (c *Client) ReadRequest(ocs *skipchain.SkipBlock, reader abstract.Scalar,
	file skipchain.SkipBlockID) (*skipchain.SkipBlock, onet.ClientError) {
	sig, err := crypto.SignSchnorr(network.Suite, reader, file)
	if err != nil {
		return nil, onet.NewClientErrorCode(ErrorParameter, err.Error())
	}

	request := &ReadRequest{
		Read: &DataOCSRead{
			Public:    network.Suite.Point().Mul(nil, reader),
			File:      file,
			Signature: &sig,
		},
		OCS: ocs.SkipChainID(),
	}
	reply := &ReadReply{}
	cerr := c.SendProtobuf(ocs.Roster.RandomServerIdentity(), request, reply)
	if cerr != nil {
		return nil, cerr
	}
	return reply.SB, nil
}

// DecryptKeyRequest has to retrieve the key from the skipchain.
func (c *Client) DecryptKeyRequest(readSB *skipchain.SkipBlock, reader abstract.Scalar) (key []byte,
	cerr onet.ClientError) {
	request := &DecryptKeyRequest{
		Read: readSB.Hash,
	}
	reply := &DecryptKeyReply{}
	cerr = c.SendProtobuf(readSB.Roster.RandomServerIdentity(), request, reply)
	if cerr != nil {
		return
	}
	key, err := protocol.DecodeKey(network.Suite, reply.X,
		reply.Cs, reply.XhatEnc, reader)
	if err != nil {
		return nil, onet.NewClientErrorCode(ErrorProtocol, "couldn't decode key: "+err.Error())
	}
	return
}

// GetFile returns the encrypted file with a given id. It takes the roster of the
// latest block and the id of the file to retrieve. It checks extensively if
// the block is correct and returns the encrypted data of the file.
func (c *Client) GetFile(roster *onet.Roster, file skipchain.SkipBlockID) ([]byte,
	onet.ClientError) {
	cl := skipchain.NewClient()
	sb, cerr := cl.GetSingleBlock(roster, file)
	if cerr != nil {
		return nil, cerr
	}
	_, ocsDataI, err := network.Unmarshal(sb.Data)
	if err != nil {
		return nil, onet.NewClientError(err)
	}
	ocsData, ok := ocsDataI.(*DataOCS)
	if !ok || ocsData.Write == nil {
		return nil, onet.NewClientError(errors.New("not correct type of data"))
	}
	return ocsData.Write.File, nil
}

// GetReadRequests searches the skipchain starting at 'start' for requests and returns all found
// requests. A maximum of 'count' requests are returned. If 'count' == 0, 'start'
// must point to a write-block, and all read-requests for that write-block will
// be returned.
func (c *Client) GetReadRequests(roster *onet.Roster, start skipchain.SkipBlockID, count int) ([]*ReadDoc, onet.ClientError) {
	request := &GetReadRequests{start, count}
	reply := &GetReadRequestsReply{}
	cerr := c.SendProtobuf(roster.RandomServerIdentity(), request, reply)
	if cerr != nil {
		return nil, cerr
	}
	return reply.Documents, nil
}
