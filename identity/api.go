package identity

import (
	"errors"
	"io"
	"io/ioutil"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/anon"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
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
		&IDBlock{},
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
	Private kyber.Scalar
	// Public key for that device - will be stored in the identity-skipchain.
	Public kyber.Point
	// ID of the skipchain this device is tied to.
	ID ID
	// Data is the actual, valid data of the identity-skipchain.
	Data *Data
	// Proposed is the new data that has not been validated by a
	// threshold of devices.
	Proposed *Data
	// DeviceName must be unique in the identity-skipchain.
	DeviceName string
}

// NewIdentity starts a new identity that can contain multiple managers with
// different accounts
func NewIdentity(r *onet.Roster, threshold int, owner string, kp *key.Pair) *Identity {
	client := onet.NewClient(cothority.Suite, ServiceName)
	if kp == nil {
		kp = key.NewKeyPair(cothority.Suite)
	}
	return &Identity{
		Client:     client,
		Private:    kp.Private,
		Public:     kp.Public,
		Data:       NewData(r, threshold, kp.Public, owner),
		DeviceName: owner,
	}
}

// NewIdentityFromRoster searches for a given cothority
func NewIdentityFromRoster(r *onet.Roster, id ID) (*Identity, error) {
	iden := &Identity{
		Client: onet.NewClient(cothority.Suite, ServiceName),
		Data:   &Data{Roster: r},
		ID:     id,
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
	_, idInt, err := network.Unmarshal(idBuf, cothority.Suite)
	if err != nil {
		return nil, err
	}
	id := idInt.(*Identity)
	id.Client = onet.NewClient(cothority.Suite, ServiceName)
	return id, nil
}

// Roster gets the roster from the latest data
func (i *Identity) Roster() *onet.Roster {
	return i.Data.Roster
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
func (i *Identity) AttachToIdentity(ID ID) error {
	i.ID = ID
	err := i.DataUpdate()
	if err != nil {
		return err
	}
	if _, exists := i.Data.Device[i.DeviceName]; exists {
		return errors.New("Adding with an existing account-name")
	}
	confPropose := i.Data.Copy()
	confPropose.Device[i.DeviceName] = &Device{i.Public}
	err = i.ProposeSend(confPropose)
	if err != nil {
		return err
	}
	return nil
}

func (i *Identity) popAuth(au *Authenticate, atts []kyber.Point, priv kyber.Scalar) (*CreateIdentity, error) {
	var as anon.Suite
	var ok bool

	if as, ok = i.Client.Suite().(anon.Suite); !ok {
		return nil, errors.New("suite does not implement anon.Suite")
	}

	// we need to find index of public key
	index := 0
	for j, key := range atts {
		if key.Equal(i.Public) {
			index = j
			break
		}
	}
	sigtag := anon.Sign(as, au.Nonce, anon.Set(atts), au.Ctx, index, priv)
	cr := &CreateIdentity{
		Data:  i.Data,
		Sig:   sigtag,
		Nonce: au.Nonce,
	}
	return cr, nil
}

func (i *Identity) publicAuth(nonce []byte, priv kyber.Scalar) (*CreateIdentity, error) {
	sig, err := schnorr.Sign(i.Client.Suite(), priv, nonce)
	if err != nil {
		return nil, err
	}
	cr := &CreateIdentity{
		Data:    i.Data,
		Sig:     []byte{},
		SchnSig: &sig,
		Nonce:   nonce,
	}
	return cr, nil
}

// CreateIdentity asks the identityService to create a new Identity
func (i *Identity) CreateIdentity(t AuthType, atts []kyber.Point, priv kyber.Scalar) error {
	log.Lvl3("Creating identity", i)

	// request for authentication
	si := i.Data.Roster.List[0]
	au := &Authenticate{[]byte{}, []byte{}}
	cerr := i.Client.SendProtobuf(si, au, au)
	if cerr != nil {
		return cerr
	}

	var cr *CreateIdentity
	var err error

	switch t {
	case PoPAuth:
		cr, err = i.popAuth(au, atts, priv)
	case PublicAuth:
		cr, err = i.publicAuth(au.Nonce, priv)
	default:
		return errors.New("wrong type of authentication")
	}
	if err != nil {
		return err
	}
	cr.Type = t
	air := &CreateIdentityReply{}
	err = i.Client.SendProtobuf(si, cr, air)
	if err != nil {
		return err
	}
	i.ID = ID(air.Genesis.Hash)
	return nil
}

// ProposeSend sends the new proposition of this identity
// ProposeVote
func (i *Identity) ProposeSend(d *Data) error {
	log.Lvl3("Sending proposal", d)
	err := i.Client.SendProtobuf(i.Data.Roster.List[0],
		&ProposeSend{i.ID, d}, nil)
	i.Proposed = d
	return err
}

// ProposeUpdate verifies if there is a new data waiting that
// needs approval from clients
func (i *Identity) ProposeUpdate() error {
	log.Lvl3("Updating proposal")
	cnc := &ProposeUpdateReply{}
	err := i.Client.SendProtobuf(i.Data.Roster.List[0], &ProposeUpdate{
		ID: i.ID,
	}, cnc)
	if err != nil {
		return err
	}
	i.Proposed = cnc.Propose
	return nil
}

// ProposeVote calls the 'accept'-vote on the current propose-data
func (i *Identity) ProposeVote(accept bool) error {
	log.Lvl3("Voting proposal")
	if i.Proposed == nil {
		return errors.New("No proposed data")
	}
	log.Lvlf3("Voting %t on %s", accept, i.Proposed.Device)
	if !accept {
		return nil
	}
	hash, err := i.Proposed.Hash(i.Client.Suite().(kyber.HashFactory))
	if err != nil {
		return err
	}
	if i.Private == nil {
		return errors.New("no private key is provided")
	}
	sig, err := schnorr.Sign(i.Client.Suite(), i.Private, hash)
	if err != nil {
		return err
	}
	log.Lvl3("Signed with public-key:", cothority.Suite.Point().Mul(i.Private, nil).String())
	pvr := &ProposeVoteReply{}
	err = i.Client.SendProtobuf(i.Data.Roster.List[0], &ProposeVote{
		ID:        i.ID,
		Signer:    i.DeviceName,
		Signature: sig,
	}, pvr)
	if err != nil {
		return err
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
func (i *Identity) DataUpdate() error {
	log.Lvl3(i)
	if i.Data.Roster == nil || len(i.Data.Roster.List) == 0 {
		return errors.New("Didn't find any list in the cothority")
	}
	cur := &DataUpdateReply{}
	err := i.Client.SendProtobuf(i.Data.Roster.List[0],
		&DataUpdate{ID: i.ID}, cur)
	if err != nil {
		return err
	}
	// TODO - verify new data
	i.Data = cur.Data
	return nil
}
