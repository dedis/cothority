package identity

import (
	"io"

	"io/ioutil"

	"math/rand"

	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/anon"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

/*
This is the external API to access the identity-service. It shows the methods
used to create a new identity-skipchain, propose new data and how
to vote on this data.
*/

func init() {
	for _, s := range []interface{}{
		// Structures
		&Device{},
		&Identity{},
		&Data{},
		&Storage{},
		&Service{},
		// API messages
		&CreateIdentity{},
		&CreateIdentityReply{},
		&DataUpdate{},
		&DataUpdateReply{},
		&ProposeSend{},
		&ProposeUpdate{},
		&ProposeUpdateReply{},
		&ProposeVote{},
		&ProposeVoteReply{},
		// Internal messages
		&PropagateIdentity{},
		&UpdateSkipBlock{},
	} {
		network.RegisterMessage(s)
	}
}

// The errors are above the skipchain-errors so that they don't mix and the
// skipchain-errors can be passed through unchanged.
const (
	ErrorDataMissing = 4200 + iota
	ErrorBlockMissing
	ErrorAccountDouble
	ErrorAccountMissing
	ErrorVoteDouble
	ErrorVoteSignature
	ErrorListMissing
	ErrorOnet
	ErrorWrongPIN
	ErrorAuthentication
	ErrorInvalidSignature
)

// AuthType is type of authentication to create skipchains
type AuthType int

// AuthType consts
const (
	PoPAuth AuthType = 100 + iota
	PublicAuth
)

// Identity structure holds the data necessary for a client/device to use the
// identity-service. Each identity-skipchain is tied to a roster that is defined
// in 'Cothority'
type Identity struct {
	// Client represents the connection to the service.
	Client *onet.Client
	// Private key for that device.
	Private abstract.Scalar
	// Public key for that device - will be stored in the identity-skipchain.
	Public abstract.Point
	// ID of the skipchain this device is tied to.
	ID ID
	// Data is the actual, valid data of the identity-skipchain.
	Data *Data
	// Proposed is the new data that has not been validated by a
	// threshold of devices.
	Proposed *Data
	// DeviceName must be unique in the identity-skipchain.
	DeviceName string
	// Cothority is the roster responsible for the identity-skipchain. It
	// might change in the case of a roster-update.
	Cothority *onet.Roster
}

// NewIdentity starts a new identity that can contain multiple managers with
// different accounts
func NewIdentity(cothority *onet.Roster, threshold int, owner string, kp *config.KeyPair) *Identity {
	client := onet.NewClient(ServiceName)
	if kp == nil {
		kp = config.NewKeyPair(network.Suite)
	}
	return &Identity{
		Client:     client,
		Private:    kp.Secret,
		Public:     kp.Public,
		Data:       NewData(threshold, kp.Public, owner),
		DeviceName: owner,
		Cothority:  cothority,
	}
}

// NewIdentityFromCothority searches for a given cothority
func NewIdentityFromCothority(el *onet.Roster, id ID) (*Identity, error) {
	iden := &Identity{
		Client:    onet.NewClient(ServiceName),
		Cothority: el,
		ID:        id,
	}
	err := iden.DataUpdate()
	if err != nil {
		return nil, err
	}
	return iden, nil
}

// NewIdentityFromStream reads the data of that client from
// any stream
func NewIdentityFromStream(in io.Reader) (*Identity, error) {
	idBuf, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, err
	}
	_, idInt, err := network.Unmarshal(idBuf)
	if err != nil {
		return nil, err
	}
	id := idInt.(*Identity)
	id.Client = onet.NewClient(ServiceName)
	return id, nil
}

// SaveToStream stores the data of the client to a stream
func (i *Identity) SaveToStream(out io.Writer) error {
	// Marshal doesn't work with the Client, so a copy is generated
	// and its Client is set to nil.
	iCopy := &Identity{}
	*iCopy = *i
	iCopy.Client = nil
	idBuf, err := network.Marshal(iCopy)
	if err != nil {
		return err
	}
	_, err = out.Write(idBuf)
	return err
}

// GetProposed returns the Propose-field or a copy of the data if
// the Propose-field is nil
func (i *Identity) GetProposed() *Data {
	if i.Proposed != nil {
		return i.Proposed
	}
	return i.Data.Copy()
}

// AttachToIdentity proposes to attach it to an existing Identity
func (i *Identity) AttachToIdentity(ID ID) onet.ClientError {
	i.ID = ID
	cerr := i.DataUpdate()
	if cerr != nil {
		return cerr
	}
	if _, exists := i.Data.Device[i.DeviceName]; exists {
		return onet.NewClientErrorCode(ErrorAccountDouble, "Adding with an existing account-name")
	}
	confPropose := i.Data.Copy()
	confPropose.Device[i.DeviceName] = &Device{i.Public}
	cerr = i.ProposeSend(confPropose)
	if cerr != nil {
		return cerr
	}
	return nil
}

func (i *Identity) popAuth(au *Authenticate, atts []abstract.Point) (*CreateIdentity, error) {
	// we need to find index of public key
	index := 0
	for j, key := range atts {
		if key.Equal(i.Public) {
			index = j
			break
		}
	}
	sigtag := anon.Sign(network.Suite, random.Stream, au.Nonce,
		anon.Set(atts), au.Ctx, index, i.Private)
	cr := &CreateIdentity{}
	cr.Data = i.Data
	cr.Roster = i.Cothority
	cr.Sig = sigtag
	cr.Nonce = au.Nonce
	return cr, nil
}

func (i *Identity) publicAuth(msg []byte) (*CreateIdentity, error) {
	sig, err := crypto.SignSchnorr(network.Suite, i.Private, msg)
	if err != nil {
		return nil, err
	}
	cr := &CreateIdentity{}
	cr.Data = i.Data
	cr.Sig = []byte{}
	cr.Roster = i.Cothority
	cr.Public = i.Public
	cr.SchnSig = sig
	cr.Nonce = msg
	return cr, nil
}

// CreateIdentity asks the identityService to create a new Identity. It will
// use the authentication auth and the ?? `atts`. If leader is nil,
// a random leader will be chosen. Else the given leader will be used.
func (i *Identity) CreateIdentity(auth AuthType, atts []abstract.Point, leader *network.ServerIdentity) onet.ClientError {
	log.Lvl3("Creating identity", i)

	// request for authentication
	log.Print(i.Cothority.List)
	rand.Seed(int64(random.Uint64(random.Stream)))
	si := leader
	if si == nil {
		si = i.Cothority.RandomServerIdentity()
	}
	au := &Authenticate{[]byte{}, []byte{}}
	cerr := i.Client.SendProtobuf(si, au, au)
	if cerr != nil {
		return cerr
	}

	var cr *CreateIdentity
	var err error
	switch auth {
	case PoPAuth:
		cr, err = i.popAuth(au, atts)
	case PublicAuth:
		cr, err = i.publicAuth(au.Nonce)
	default:
		return onet.NewClientErrorCode(ErrorAuthentication, "wrong type of authentication")
	}
	if err != nil {
		return onet.NewClientError(err)
	}
	cr.Type = auth
	air := &CreateIdentityReply{}
	cerr = i.Client.SendProtobuf(si, cr, air)
	if cerr != nil {
		return cerr
	}
	i.ID = ID(air.Data.Hash)
	return nil
}

// ProposeSend sends the new proposition of this identity
// ProposeVote
func (i *Identity) ProposeSend(d *Data) onet.ClientError {
	log.Lvl3("Sending proposal", d)
	err := i.Client.SendProtobuf(i.Cothority.RandomServerIdentity(),
		&ProposeSend{i.ID, d}, nil)
	i.Proposed = d
	return err
}

// ProposeUpdate verifies if there is a new data waiting that
// needs approval from clients
func (i *Identity) ProposeUpdate() onet.ClientError {
	log.Lvl3("Updating proposal")
	cnc := &ProposeUpdateReply{}
	err := i.Client.SendProtobuf(i.Cothority.RandomServerIdentity(), &ProposeUpdate{
		ID: i.ID,
	}, cnc)
	if err != nil {
		return err
	}
	i.Proposed = cnc.Propose
	return nil
}

// ProposeVote calls the 'accept'-vote on the current propose-data
func (i *Identity) ProposeVote(accept bool) onet.ClientError {
	log.Lvl3("Voting proposal")
	if i.Proposed == nil {
		return onet.NewClientErrorCode(ErrorDataMissing, "No proposed data")
	}
	log.Lvlf3("Voting %t on %s", accept, i.Proposed.Device)
	if !accept {
		return nil
	}
	hash, err := i.Proposed.Hash()
	if err != nil {
		return onet.NewClientErrorCode(ErrorOnet, err.Error())
	}
	if i.Private == nil {
		return onet.NewClientErrorCode(ErrorVoteSignature, "no private key is provided")
	}
	sig, err := crypto.SignSchnorr(network.Suite, i.Private, hash)
	if err != nil {
		return onet.NewClientErrorCode(ErrorOnet, err.Error())
	}
	pvr := &ProposeVoteReply{}
	cerr := i.Client.SendProtobuf(i.Cothority.RandomServerIdentity(), &ProposeVote{
		ID:        i.ID,
		Signer:    i.DeviceName,
		Signature: &sig,
	}, pvr)
	if cerr != nil {
		return cerr
	}
	if pvr.Data != nil {
		log.Lvl2("Threshold reached and signed")
		i.Data = i.Proposed
		i.Proposed = nil
	} else {
		log.Lvl2("Threshold not reached")
	}
	return nil
}

// DataUpdate asks if there is any new data available that has already
// been approved by others and updates the local data
func (i *Identity) DataUpdate() onet.ClientError {
	log.Lvl3(i)
	if i.Cothority == nil || len(i.Cothority.List) == 0 {
		return onet.NewClientErrorCode(ErrorListMissing, "Didn't find any list in the cothority")
	}
	cur := &DataUpdateReply{}
	err := i.Client.SendProtobuf(i.Cothority.RandomServerIdentity(),
		&DataUpdate{ID: i.ID}, cur)
	if err != nil {
		return err
	}
	// TODO - verify new data
	i.Data = cur.Data
	return nil
}

// RequestLinkPIN tries to store a public key at the conode for it to
// recognize further admin-requests. There are two link-types possible:
// either by using a PIN that is read from the conode-logs, or by
// using the private key of the conode.
func (i *Identity) RequestLinkPIN(si *network.ServerIdentity, pin string, public abstract.Point) onet.ClientError {
	return i.Client.SendProtobuf(si, &RequestLink{PIN: &pin, Public: public}, nil)
}

// RequestLinkPrivate tries to store a public key at the conode for it to
// recognize further admin-requests. There are two link-types possible:
// either by using a PIN that is read from the conode-logs, or by
// using the private key of the conode.
// The public key given will be stored in the conode and will be used to
// verify any signatures for admin-requests. This public key should be
// different from the conode's public key!
func (i *Identity) RequestLinkPrivate(si *network.ServerIdentity, private abstract.Scalar, public abstract.Point) onet.ClientError {
	if si.Public != nil && si.Public.Equal(public) {
		return onet.NewClientErrorCode(ErrorAuthentication, "don't use conode's public key for authentication")
	}
	auth := &Authenticate{}
	if err := i.Client.SendProtobuf(si, auth, auth); err != nil {
		return err
	}
	sig, err := crypto.SignSchnorr(network.Suite, i.Private, auth.Nonce)
	if err != nil {
		return onet.NewClientErrorCode(ErrorAuthentication, "error while schnorr signing the nonce")
	}

	return i.Client.SendProtobuf(si, &RequestLink{Public: public, Nonce: &auth.Nonce, Sig: &sig}, nil)
}
