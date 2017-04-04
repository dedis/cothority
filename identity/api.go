package identity

import (
	"io"

	"io/ioutil"

	"errors"

	"github.com/dedis/cothority/skipchain"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
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
		&Identity{},
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
	// Private key for that device.
	Private abstract.Scalar
	// Public key for that device - will be stored in the identity-skipchain.
	Public abstract.Point
	// Config is the actual, valid configuration of the identity-skipchain.
	Config *Config
	// Proposed is the new configuration that has not been validated by a
	// threshold of devices.
	Proposed *Config
	// DeviceName must be unique in the identity-skipchain.
	DeviceName string
	// SkipBlock is the latest block holding our data.
	SkipBlock *skipchain.SkipBlock
	// client for easy communication
	client *onet.Client
}

// NewIdentity starts a new identity that can contain multiple managers with
// different accounts. It takes a control-skipchain as argument and will
// append a data-skipchain on that.
func NewIdentity(control *skipchain.SkipBlock, threshold int, owner string) (*Identity, error) {
	kp := config.NewKeyPair(network.Suite)
	config := NewConfig(threshold, kp.Public, owner)
	var sbData *skipchain.SkipBlock
	if control != nil {
		var err error
		sbData, err = skipchain.NewClient().CreateGenesis(control.Roster, 4, 4,
			verificationIdentity, config, control.SkipChainID())
		if err != nil {
			return nil, err
		}
	}
	return &Identity{
		Private:    kp.Secret,
		Public:     kp.Public,
		Config:     config,
		DeviceName: owner,
		SkipBlock:  sbData,
		client:     onet.NewClient(ServiceName),
	}, nil
}

// NewIdentityFromRoster takes a roster and creates a root-, and
// control- skipchain where the identity data-skipchain will be
// added.
func NewIdentityFromRoster(el *onet.Roster, rootKeys []abstract.Point, threshold int, owner string) (root, control *skipchain.SkipBlock, id *Identity, err error) {
	root, control, err = skipchain.NewClient().CreateRootControl(
		el, el, rootKeys, 4, 4, 4)
	if err != nil {
		return
	}
	id, err = NewIdentity(control, threshold, owner)
	return
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
	id, ok := i.(*Identity)
	if !ok {
		return nil, errors.New("could not convert to Identity")
	}
	return id, nil
}

// NewFollower searches for an existing identity-skipchain and returns a
// read-only identity. The url is where the skipchain holding the identity
// can be found. If it is empty (""), it defaults to skipchain.dedis.ch.
func NewFollower(id skipchain.SkipBlockID, url string) (*Identity, error) {
	sb, err := skipchain.FindSkipChain(id, url)
	if err != nil {
		return nil, err
	}
	_, idInt, err := network.Unmarshal(sb.Data)
	if err != nil {
		return nil, err
	}
	identity, ok := idInt.(*Identity)
	if !ok {
		return nil, errors.New("This is not a cisc-skipchain")
	}
	return identity, nil
}

// SaveToStream stores the configuration of the client to a stream
func (i *Identity) SaveToStream(out io.Writer) error {
	data, err := network.Marshal(i)
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

// AttachToIdentity proposes to attach it to an existing Identity. It takes
// the name of this device that should sign up to the identity-skipchain.
func (i *Identity) AttachToIdentity(name string) onet.ClientError {
	cerr := i.ConfigUpdate()
	if cerr != nil {
		return cerr
	}
	if _, exists := i.Config.Device[i.DeviceName]; exists {
		return onet.NewClientErrorCode(ErrorAccountDouble, "Adding with an existing account-name")
	}
	confPropose := i.Config.Copy()
	confPropose.Device[name] = &Device{i.Public}
	cerr = i.ProposeSend(confPropose)
	if cerr != nil {
		return cerr
	}
	return nil
}

// ProposeSend sends the new proposition of this identity
// ProposeVote
func (i *Identity) ProposeSend(il *Config) onet.ClientError {
	log.Lvl3("Sending proposal", il)
	err := i.sendProtobuf(i.randomSI(),
		&ProposeSend{i.SkipBlock.SkipChainID(), il}, nil)
	i.Proposed = il
	return err
}

// ProposeUpdate verifies if there is a new configuration awaiting that
// needs approval from clients
func (i *Identity) ProposeUpdate() onet.ClientError {
	log.Lvl3("Updating proposal")
	cnc := &ProposeUpdateReply{}
	err := i.sendProtobuf(i.randomSI(), &ProposeUpdate{
		ID: i.SkipBlock.SkipChainID(),
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
	cerr := i.sendProtobuf(i.randomSI(), &ProposeVote{
		ID:        i.ID(),
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
	gucr, cerr := skipchain.NewClient().GetUpdateChain(i.SkipBlock.Roster, i.ID())
	if cerr != nil {
		return cerr
	}
	if len(gucr.Update) == 0 {
		log.Lvl3("Didn't get any update")
		return nil
	}
	last := gucr.Update[len(gucr.Update)-1]
	_, d, err := network.Unmarshal(last.Data)
	if err != nil {
		return onet.NewClientError(err)
	}
	conf, ok := d.(*Config)
	if !ok {
		return onet.NewClientErrorCode(4000, "Didn't find config in data-part")
	}
	// TODO - verify new config
	i.Config = conf
	return nil
}

// ID returns the id of the identity-skipchain, which is the genesis-id
// of the data-skipchain holding the data.
func (i *Identity) ID() skipchain.SkipBlockID {
	return i.SkipBlock.SkipChainID()
}

// convenience-function that wraps the creation of a client if necessary. This
// is handy if the identity-structure got loaded from inside another structure,
// like it is done in cothority/cisc/lib::loadConfig.
func (i *Identity) sendProtobuf(dst *network.ServerIdentity, msg interface{}, ret interface{}) onet.ClientError {
	if i.client == nil {
		i.client = onet.NewClientKeep(ServiceName)
	}
	return i.client.SendProtobuf(dst, msg, ret)
}

func (i *Identity) randomSI() *network.ServerIdentity {
	return i.SkipBlock.Roster.RandomServerIdentity()
}
