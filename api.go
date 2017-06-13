package logread

/*
The api.go defines the methods that can be called from the outside. Most
of the methods will take a roster so that the service knows which nodes
it should work with.

This part of the service runs on the client or the app.
*/

import (
	"errors"

	"github.com/dedis/cothority/skipchain"
	"github.com/satori/go.uuid"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/network"
)

// ServiceName is used for registration on the onet.
const ServiceName = "LogRead"

// VerifyLogreadACL makes sure that all necessary signatures are present when
// updating the ACL-skipchain.
var VerifyLogreadACL = skipchain.VerifierID(uuid.NewV5(uuid.NamespaceURL, "LogreadACL"))

// VerificationLogreadACL adds the VerifyBase to the VerifyLogreadACL for a complete
// skipchain.
var VerificationLogreadACL = []skipchain.VerifierID{skipchain.VerifyBase,
	VerifyLogreadACL}

// VerifyLogreadWLR makes sure that all necessary signatures are present when
// updating the WLR-skipchain.
var VerifyLogreadWLR = skipchain.VerifierID(uuid.NewV5(uuid.NamespaceURL, "LogreadWLR"))

// VerificationLogreadWLR adds the VerifyBase to the VerifyLogreadWLR for a complete
// skipchain.
var VerificationLogreadWLR = []skipchain.VerifierID{skipchain.VerifyBase,
	VerifyLogreadWLR}

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

// CreateSkipchains creates a new credential for the administrator and the two
// necessary skipchains. It returns:
//  - acl-skipchain, wlr-skipchain, admin-credentials, error
func (c *Client) CreateSkipchains(r *onet.Roster, n string) (acl, wlr *skipchain.SkipBlock,
	admin *Credential, cerr onet.ClientError) {
	admin = NewCredential(n)
	dataACL := &DataACL{Admins: NewCredentials(admin)}
	req := &CreateSkipchainsRequest{
		Roster: r,
		ACL:    NewDataACLEvolve(dataACL, nil, admin.Private),
	}
	reply := &CreateSkipchainsReply{}
	cerr = c.SendProtobuf(r.RandomServerIdentity(), req, reply)
	acl, wlr = reply.ACL, reply.Wlr
	return
}

// BunchAddBlock adds a block to the latest block from the bunch. If the block
// doesn't have a roster set, it will be copied from the last block.
func (c *Client) BunchAddBlock(bunch *SkipBlockBunch, r *onet.Roster, data interface{}) (*skipchain.SkipBlock, onet.ClientError) {
	reply, err := skipchain.NewClient().StoreSkipBlock(bunch.Latest, r, data)
	if err != nil {
		return nil, err
	}
	sbNew := reply.Latest
	id := bunch.Store(sbNew)
	if id == nil {
		return nil, onet.NewClientErrorCode(ErrorProtocol,
			"Couldn't add block to bunch")
	}
	return sbNew, nil
}

// EvolveACL asks the skipchain to store a new block with a new Access-Control-List.
// The admin-credential must be present in the previous block, else it will be
// rejected.
func (c *Client) EvolveACL(acl *skipchain.SkipBlock, newACL *DataACL, admin *Credential) (rep *EvolveACLReply,
	cerr onet.ClientError) {
	req := &EvolveACLRequest{
		ACL:     acl.SkipChainID(),
		NewAcls: NewDataACLEvolve(newACL, acl, admin.Private),
	}
	rep = &EvolveACLReply{}
	cerr = NewClient().SendProtobuf(acl.Roster.RandomServerIdentity(), req, rep)
	return
}

// EncryptAndWriteRequest takes data and a credential, then it creates a new
// symmetric encryption key, encrypts the document, and stores the document and
// the encryption key on the blockchain.
func (c *Client) EncryptAndWriteRequest(wlr *skipchain.SkipBlock, data []byte, cred *Credential) (sb *skipchain.SkipBlock,
	cerr onet.ClientError) {
	if len(data) > 1e7 {
		return nil, onet.NewClientErrorCode(ErrorParameter, "Cannot store files bigger than 10MB")
	}
	key := random.Bytes(32, random.Stream)
	cipher := network.Suite.Cipher(key)
	str := cipher.Seal(nil, data)
	encKey, cerr := c.EncryptKeyRequest(wlr.Roster, key)
	if cerr != nil {
		return
	}
	return c.WriteRequest(wlr, str, encKey, cred)
}

// WriteRequest pushes a new block on the skipchain with the encrypted file
// on it.
func (c *Client) WriteRequest(wlr *skipchain.SkipBlock, encData []byte, encKey []byte, cred *Credential) (sb *skipchain.SkipBlock,
	cerr onet.ClientError) {
	if len(encData) > 1e7 {
		return nil, onet.NewClientErrorCode(ErrorParameter, "Cannot store files bigger than 10MB")
	}
	sig, err := crypto.SignSchnorr(network.Suite, cred.Private, encKey)
	if err != nil {
		return nil, onet.NewClientErrorCode(ErrorParameter, err.Error())
	}
	wr := &WriteRequest{
		Write: &DataWlrWrite{
			File:      encData,
			Key:       encKey,
			Signature: &sig,
		},
		Wlr: wlr.SkipChainID(),
	}
	reply := &WriteReply{}
	cerr = c.SendProtobuf(wlr.Roster.RandomServerIdentity(), wr, reply)
	sb = reply.SB
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
	_, wlrDataI, err := network.Unmarshal(sb.Data)
	if err != nil {
		return nil, onet.NewClientError(err)
	}
	wlrData, ok := wlrDataI.(*DataWlr)
	if !ok || wlrData.Write == nil {
		return nil, onet.NewClientError(errors.New("not correct type of data"))
	}
	return wlrData.Write.File, nil
}

// ReadRequest asks the wlr-skipchain to add a block giving access to 'reader'
// for the file that references the skipblock it is stored in.
func (c *Client) ReadRequest(wlr *skipchain.SkipBlock, reader *Credential,
	file skipchain.SkipBlockID) (*skipchain.SkipBlock, error) {
	sig, err := crypto.SignSchnorr(network.Suite, reader.Private, file)
	if err != nil {
		return nil, err
	}
	sbCl := skipchain.NewClient()
	sbFile, cerr := sbCl.GetSingleBlock(wlr.Roster, file)
	if cerr != nil {
		return nil, cerr
	}
	_, dwI, err := network.Unmarshal(sbFile.Data)
	if err != nil {
		return nil, err
	}
	dw, ok := dwI.(*DataWlr)
	if !ok {
		return nil, errors.New("didn't get file-block")
	}
	if dw == nil || dw.Write == nil {
		return nil, errors.New("this is not a write-block")
	}
	request := &ReadRequest{
		Read: &DataWlrRead{
			Pseudonym: reader.Pseudonym,
			Public:    reader.Public,
			File:      file,
			EncKey:    dw.Write.Key,
			Signature: &sig,
		},
		Wlr: wlr.SkipChainID(),
	}
	reply := &ReadReply{}
	err = c.SendProtobuf(wlr.Roster.RandomServerIdentity(), request, reply)
	if err != nil {
		return nil, err
	}
	return reply.SB, nil
}

// EncryptKeyRequest does something to the key before it is sent to the skipchain.
func (c *Client) EncryptKeyRequest(roster *onet.Roster, key []byte) (encKey []byte,
	cerr onet.ClientError) {
	request := &EncryptKeyRequest{
		Roster: roster,
	}
	reply := &EncryptKeyReply{}
	cerr = c.SendProtobuf(roster.RandomServerIdentity(), request, reply)
	if cerr != nil {
		return
	}
	// TODO: do something good with the reply
	encKey = key
	return
}

// DecryptKeyRequest has to retrieve the key from the skipchain.
func (c *Client) DecryptKeyRequest(roster *onet.Roster, reqID skipchain.SkipBlockID, reader *Credential) (key []byte,
	cerr onet.ClientError) {
	request := &DecryptKeyRequest{
		Read: reqID,
	}
	reply := &DecryptKeyReply{}
	cerr = c.SendProtobuf(roster.RandomServerIdentity(), request, reply)
	if cerr != nil {
		return
	}
	for _, eg := range reply.KeyParts {
		msg, err := ElGamalDecrypt(network.Suite, reader.Private, eg)
		if err != nil {
			return nil, onet.NewClientError(err)
		}
		key = append(key, msg...)
	}
	return
}

// GetReadRequests searches the skipchain starting at 'start' for requests and returns all found
// requests. A maximum of 'count' requests are returned.
func (c *Client) GetReadRequests(roster *onet.Roster, start skipchain.SkipBlockID, count int) ([]*ReadDoc, onet.ClientError) {
	request := &GetReadRequests{start, count}
	reply := &GetReadRequestsReply{}
	cerr := c.SendProtobuf(roster.RandomServerIdentity(), request, reply)
	if cerr != nil {
		return nil, cerr
	}
	return reply.Documents, nil
}
