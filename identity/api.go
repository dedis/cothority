package identity

import (
	"io"

	"io/ioutil"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

/*
This is the external API to access the identity-service. It shows the methods
used to create a new identity-skipchain, propose new configurations and how
to vote on these configurations.
*/

func init() {
	for _, s := range []interface{}{
		// Structures
		&Device{},
		&Identity{},
		&Config{},
		&Storage{},
		&Service{},
		// API messages
		&CreateIdentity{},
		&CreateIdentityReply{},
		&ConfigUpdate{},
		&ConfigUpdateReply{},
		&ProposeSend{},
		&ProposeUpdate{},
		&ProposeUpdateReply{},
		&ProposeVote{},
		&Data{},
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
	ErrorConfigMissing = 4200 + iota
	ErrorBlockMissing
	ErrorAccountDouble
	ErrorAccountMissing
	ErrorVoteDouble
	ErrorVoteSignature
	ErrorListMissing
	ErrorOnet
)

// Identity structure holds the data necessary for a client/device to use the
// identity-service. Each identity-skipchain is tied to a roster that is defined
// in 'Cothority'
type Identity struct {
	// Client is included for easy `Send`-methods.
	*onet.Client
	// IdentityData holds all the data related to this identity
	// It can be stored and loaded from a config file.
	Data
}

// Data contains the data that will be stored / loaded from / to a file
// that enables a client to use the Identity service.
type Data struct {
	// Private key for that device.
	Private abstract.Scalar
	// Public key for that device - will be stored in the identity-skipchain.
	Public abstract.Point
	// ID of the skipchain this device is tied to.
	ID ID
	// Config is the actual, valid configuration of the identity-skipchain.
	Config *Config
	// Proposed is the new configuration that has not been validated by a
	// threshold of devices.
	Proposed *Config
	// DeviceName must be unique in the identity-skipchain.
	DeviceName string
	// Cothority is the roster responsible for the identity-skipchain. It
	// might change in the case of a roster-update.
	Cothority *onet.Roster
}

// NewIdentity starts a new identity that can contain multiple managers with
// different accounts
func NewIdentity(cothority *onet.Roster, threshold int, owner string) *Identity {
	client := onet.NewClient(ServiceName)
	kp := config.NewKeyPair(network.Suite)
	return &Identity{
		Client: client,
		Data: Data{
			Private:    kp.Secret,
			Public:     kp.Public,
			Config:     NewConfig(threshold, kp.Public, owner),
			DeviceName: owner,
			Cothority:  cothority,
		},
	}
}

// NewIdentityFromCothority searches for a given cothority
func NewIdentityFromCothority(el *onet.Roster, id ID) (*Identity, error) {
	iden := &Identity{
		Client: onet.NewClient(ServiceName),
		Data: Data{
			Cothority: el,
			ID:        id,
		},
	}
	err := iden.ConfigUpdate()
	if err != nil {
		return nil, err
	}
	return iden, nil
}

// NewIdentityFromStream reads the configuration of that client from
// any stream
func NewIdentityFromStream(in io.Reader) (*Identity, error) {
	data, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, err
	}
	_, i, err := network.Unmarshal(data)
	if err != nil {
		return nil, err
	}
	id := i.(*Data)
	identity := &Identity{
		Client: onet.NewClient(ServiceName),
		Data:   *id,
	}
	return identity, nil
}

// SaveToStream stores the configuration of the client to a stream
func (i *Identity) SaveToStream(out io.Writer) error {
	data, err := network.Marshal(&i.Data)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

// GetProposed returns the Propose-field or a copy of the config if
// the Propose-field is nil
func (i *Identity) GetProposed() *Config {
	if i.Proposed != nil {
		return i.Proposed
	}
	return i.Config.Copy()
}

// AttachToIdentity proposes to attach it to an existing Identity
func (i *Identity) AttachToIdentity(ID ID) onet.ClientError {
	i.ID = ID
	cerr := i.ConfigUpdate()
	if cerr != nil {
		return cerr
	}
	if _, exists := i.Config.Device[i.DeviceName]; exists {
		return onet.NewClientErrorCode(ErrorAccountDouble, "Adding with an existing account-name")
	}
	confPropose := i.Config.Copy()
	confPropose.Device[i.DeviceName] = &Device{i.Public}
	cerr = i.ProposeSend(confPropose)
	if cerr != nil {
		return cerr
	}
	return nil
}

// CreateIdentity asks the identityService to create a new Identity
func (i *Identity) CreateIdentity() onet.ClientError {
	log.Lvl3("Creating identity", i)
	air := &CreateIdentityReply{}
	err := i.SendProtobuf(i.Cothority.RandomServerIdentity(),
		&CreateIdentity{i.Config, i.Cothority},
		air)
	if err != nil {
		return err
	}
	i.ID = ID(air.Data.Hash)
	return nil
}

// ProposeSend sends the new proposition of this identity
// ProposeVote
func (i *Identity) ProposeSend(il *Config) onet.ClientError {
	log.Lvl3("Sending proposal", il)
	err := i.Client.SendProtobuf(i.Cothority.RandomServerIdentity(),
		&ProposeSend{i.ID, il}, nil)
	i.Proposed = il
	return err
}

// ProposeUpdate verifies if there is a new configuration awaiting that
// needs approval from clients
func (i *Identity) ProposeUpdate() onet.ClientError {
	log.Lvl3("Updating proposal")
	cnc := &ProposeUpdateReply{}
	err := i.SendProtobuf(i.Cothority.RandomServerIdentity(), &ProposeUpdate{
		ID: i.ID,
	}, cnc)
	if err != nil {
		return err
	}
	i.Proposed = cnc.Propose
	return nil
}

// ProposeVote calls the 'accept'-vote on the current propose-configuration
func (i *Identity) ProposeVote(accept bool) onet.ClientError {
	log.Lvl3("Voting proposal")
	if i.Proposed == nil {
		return onet.NewClientErrorCode(ErrorConfigMissing, "No proposed config")
	}
	log.Lvlf3("Voting %t on %s", accept, i.Proposed.Device)
	if !accept {
		return nil
	}
	hash, err := i.Proposed.Hash()
	if err != nil {
		return onet.NewClientErrorCode(ErrorOnet, err.Error())
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
		i.Config = i.Proposed
		i.Proposed = nil
	} else {
		log.Lvl2("Threshold not reached")
	}
	return nil
}

// ConfigUpdate asks if there is any new config available that has already
// been approved by others and updates the local configuration
func (i *Identity) ConfigUpdate() onet.ClientError {
	log.Lvl3("ConfigUpdate", i)
	if i.Cothority == nil || len(i.Cothority.List) == 0 {
		return onet.NewClientErrorCode(ErrorListMissing, "Didn't find any list in the cothority")
	}
	cur := &ConfigUpdateReply{}
	err := i.Client.SendProtobuf(i.Cothority.RandomServerIdentity(),
		&ConfigUpdate{ID: i.ID}, cur)
	if err != nil {
		return err
	}
	// TODO - verify new config
	i.Config = cur.Config
	return nil
}
