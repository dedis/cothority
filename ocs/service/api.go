package service

/*
The api.go defines the methods that can be called from the outside. Most
of the methods will take a roster so that the service knows which nodes
it should work with.

This part of the service runs on the client or the app.
*/

import (
	"errors"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/ocs/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

// Client is a structure to communicate with the OCS service
// service. It can handle connections to different nodes, and
// will re-use existing connections transparently. To force
// closing of all connections, use Client.Close()
type Client struct {
	*onet.Client
	sbc *skipchain.Client
}

// NewClient instantiates a new ocs.Client
func NewClient() *Client {
	return &Client{
		Client: onet.NewClient(cothority.Suite, ServiceName),
		sbc:    skipchain.NewClient(),
	}
}

// CreateSkipchain creates a new OCS-skipchain using the roster r. The OCS-service
// will create a new skipchain with an empty first genesis-block. You can create more
// than one skipchain at the same time.
//
// Input:
//  - r [*onet.Roster] - the roster of the nodes holding the new skipchain
//  - admin [*darc.Darc] - the administrator of the ocs-skipchain
//
// Returns:
//  - ocs [*SkipChainURL] - the identity of that new skipchain
//  - err - an error if something went wrong, or nil
func (c *Client) CreateSkipchain(r *onet.Roster, admin *darc.Darc) (ocs *SkipChainURL,
	err error) {
	req := &CreateSkipchainsRequest{
		Roster:  *r,
		Writers: *admin,
	}
	reply := &CreateSkipchainsReply{}
	err = c.SendProtobuf(r.List[0], req, reply)
	if err != nil {
		return nil, err
	}
	ocs = NewSkipChainURL(reply.OCS)
	return
}

// EditAccount creates a new account on the skipchain. If the account-ID already exists,
// there must be a valid signature provided in the Darc-structure, and all elements
// must be valid: Version_new = Version_old + 1, Threshold_new = Threshold_old and the
// different Darc-changes must follow the rules.
func (c *Client) EditAccount(ocs *SkipChainURL, d *darc.Darc) (sb *skipchain.SkipBlock,
	err error) {
	req := &UpdateDarc{
		Darc: *d,
		OCS:  ocs.Genesis,
	}
	reply := &UpdateDarcReply{}
	err = c.SendProtobuf(ocs.Roster.List[0], req, reply)
	if err != nil {
		return
	}
	return reply.SB, nil
}

// WriteRequest contacts the ocs-service and requests the addition of a new write-
// block with the given encData. The encData has already to be encrypted using the symmetric
// symKey. This method will encrypt the symKey using the public shared key of the
// ocs-service and only send this encrypted key over the network. The block will also
// contain the list of readers that are allowed to request the key.
//
// Input:
//  - ocs [*SkipChainURL] - the url of the skipchain to use
//  - encData [[]byte] - the data - already encrypted using symKey
//  - symKey [[]byte] - the symmetric key - it will be encrypted using the shared public key
//  - adminKey [kyber.Scalar] - the private key of an admin
//  - acl [Darc] - the access control list of public keys that are allowed to access
//    that resource
//
// Output:
//  - sb [*skipchain.SkipBlock] - the actual block written in the skipchain. The
//    Data-field of the block contains the actual write request.
//  - err - an error if something went wrong, or nil
func (c *Client) WriteRequest(ocs *SkipChainURL, encData []byte, symKey []byte,
	sig *darc.Signature, acl *darc.Darc) (sb *skipchain.SkipBlock,
	err error) {
	if len(encData) > 1e7 {
		return nil, errors.New("Cannot store data bigger than 10MB")
	}

	requestShared := &SharedPublicRequest{Genesis: ocs.Genesis}
	shared := &SharedPublicReply{}
	err = c.SendProtobuf(ocs.Roster.List[0], requestShared, shared)
	if err != nil {
		return
	}

	write := NewWrite(cothority.Suite, ocs.Genesis, shared.X, acl, symKey)
	write.Data = encData
	wr := &WriteRequest{
		Write:     *write,
		Readers:   acl,
		OCS:       ocs.Genesis,
		Signature: *sig,
	}
	reply := &WriteReply{}
	err = c.SendProtobuf(ocs.Roster.List[0], wr, reply)
	sb = reply.SB
	return
}

// ReadRequest is used to request a re-encryption of the symmetric key of the
// given data. The ocs-skipchain will verify if the signature corresponds to
// one of the public keys given in the write-request, and only if this is valid,
// it will add the block to the skipchain.
//
// Input:
//  - ocs [*SkipChainURL] - the url of the skipchain to use
//  - data [skipchain.SkipBlockID] - the hash of the write-request where the
//    data is stored
//  - reader [kyber.Scalar] - the private key of the reader. It is used to
//    sign the request to authenticate to the skipchain.
//
// Output:
//  - sb [*skipchain.SkipBlock] - the read-request that has been added to the
//    skipchain if it accepted the signature.
//  - err - an error if something went wrong, or nil
func (c *Client) ReadRequest(ocs *SkipChainURL, dataID skipchain.SkipBlockID,
	reader kyber.Scalar) (sb *skipchain.SkipBlock, err error) {
	sig, err := schnorr.Sign(cothority.Suite, reader, dataID)
	if err != nil {
		return nil, err
	}

	request := &ReadRequest{
		Read: Read{
			DataID:    dataID,
			Signature: darc.Signature{Signature: sig},
		},
		OCS: ocs.Genesis,
	}
	reply := &ReadReply{}
	err = c.SendProtobuf(ocs.Roster.List[0], request, reply)
	if err != nil {
		return nil, err
	}
	return reply.SB, nil
}

// DecryptKeyRequest takes the id of a successful read-request and asks the cothority
// to re-encrypt the symmetric key under the reader's public key. The cothority
// does a distributed re-encryption, so that the actual symmetric key is never revealed
// to any of the nodes.
//
// Input:
//  - ocs [*SkipChainURL] - the url of the skipchain to use
//  - readID [skipchain.SkipBlockID] - the ID of the successful read-request
//  - reader [kyber.Scalar] - the private key of the reader. It will be used to
//    decrypt the symmetric key.
//
// Output:
//  - sym [[]byte] - the decrypted symmetric key
//  - err - an error if something went wrong, or nil
func (c *Client) DecryptKeyRequest(ocs *SkipChainURL, readID skipchain.SkipBlockID, reader kyber.Scalar) (sym []byte,
	err error) {
	request := &DecryptKeyRequest{
		Read: readID,
	}
	reply := &DecryptKeyReply{}
	err = c.SendProtobuf(ocs.Roster.List[0], request, reply)
	if err != nil {
		return
	}

	log.LLvl2("Got decryption key")

	sym, err = DecodeKey(cothority.Suite, reply.X,
		reply.Cs, reply.XhatEnc, reader)
	if err != nil {
		return nil, errors.New("could not decode sym: " + err.Error())
	}
	return
}

// DecryptKeyRequestEphemeral works similar to DecryptKeyRequest but generates
// an ephemeral keypair that is used in the decryption. It still needs the
// reader to be able to sign the ephemeral keypair, to make sure that the read-
// request is valid.
//
// Input:
//  - ocs [*SkipChainURL] - the url of the skipchain to use
//  - readID [skipchain.SkipBlockID] - the ID of the successful read-request
//  - reader [*darc.Signer] - the reader that has requested the read
//
// Output:
//  - sym [[]byte] - the decrypted symmetric key
//  - cerr [ClientError] - an eventual error if something went wrong, or nil
func (c *Client) DecryptKeyRequestEphemeral(ocs *SkipChainURL, readID skipchain.SkipBlockID, readerDarc *darc.Darc, reader *darc.Signer) (sym []byte,
	err error) {
	kp := key.NewKeyPair(cothority.Suite)
	id := darc.NewIdentityEd25519(reader.Ed25519.Point)
	path := darc.NewSignaturePath([]*darc.Darc{readerDarc}, *id, darc.User)
	msg, err := kp.Public.MarshalBinary()
	if err != nil {
		return
	}
	sig, err := darc.NewDarcSignature(msg, path, reader)
	if err != nil {
		return
	}
	request := &DecryptKeyRequest{
		Read:      readID,
		Ephemeral: kp.Public,
		Signature: sig,
	}
	reply := &DecryptKeyReply{}
	err = c.SendProtobuf(ocs.Roster.List[0], request, reply)
	if err != nil {
		return
	}

	log.LLvl2("Got decryption key")
	sym, err = DecodeKey(cothority.Suite, reply.X,
		reply.Cs, reply.XhatEnc, kp.Private)
	if err != nil {
		return
	}
	return
}

// GetData returns the encrypted data from a write-request given its id. It requests
// the data from the skipchain. To decode the data, the caller has to have a
// decrypted symmetric key, then he can decrypt the data with:
//
//   cipher := cothority.Suite.Cipher(key)
//   data, err := cipher.Open(nil, encData)
//
// Input:
//  - ocs [*SkipChainURL] - the url of the skipchain to use
//  - dataID [skipchain.SkipBlockID] - the hash of the skipblock where the data
//    is stored
//
// Output:
//  - err - an error if something went wrong, or nil
func (c *Client) GetData(ocs *SkipChainURL, dataID skipchain.SkipBlockID) (encData []byte,
	err error) {
	cl := skipchain.NewClient()
	sb, err := cl.GetSingleBlock(ocs.Roster, dataID)
	if err != nil {
		return nil, err
	}
	ocsData := NewOCS(sb.Data)
	if ocsData == nil || ocsData.Write == nil {
		return nil, errors.New("not correct type of data")
	}
	return ocsData.Write.Data, nil
}

// GetReadRequests searches the skipchain starting at 'start' for requests and returns all found
// requests. A maximum of 'count' requests are returned. If 'count' == 0, 'start'
// must point to a write-block, and all read-requests for that write-block will
// be returned.
//
// Input:
//  - ocs [*SkipChainURL] - the url of the skipchain to use
//
// Output:
//  - err - an error if something went wrong, or nil
func (c *Client) GetReadRequests(ocs *SkipChainURL, start skipchain.SkipBlockID, count int) ([]*ReadDoc, error) {
	request := &GetReadRequests{start, count}
	reply := &GetReadRequestsReply{}
	err := c.SendProtobuf(ocs.Roster.List[0], request, reply)
	if err != nil {
		return nil, err
	}
	return reply.Documents, nil
}

// GetLatestDarc looks for an update path to the latest valid
// darc given either a genesis-darc and nil, or a later darc
// and its base-darc.
func (c *Client) GetLatestDarc(ocs *SkipChainURL, darcID darc.ID) (path *[]*darc.Darc, err error) {
	request := &GetLatestDarc{
		OCS:    ocs.Genesis,
		DarcID: darcID,
	}
	reply := &GetLatestDarcReply{}
	err = c.SendProtobuf(ocs.Roster.List[0], request, reply)
	if err != nil {
		return
	}
	return reply.Darcs, nil
}
