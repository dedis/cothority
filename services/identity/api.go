package identity

import (
	"errors"
	"io"

	"io/ioutil"

	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

/*
 */

func init() {
	for _, s := range []interface{}{
		&Owner{},
		&Identity{},
		&AccountList{},
		&AddIdentity{},
		&AddIdentityReply{},
		&PropagateIdentity{},
		&PropagateProposition{},
		&AttachToIdentity{},
		&ConfigNewCheck{},
		&ConfigUpdate{},
		&UpdateSkipBlock{},
		&Vote{},
	} {
		network.RegisterMessageType(s)
	}
}

// Identity can both follow and update an IdentityList
type Identity struct {
	*sda.Client
	Private    abstract.Secret
	Public     abstract.Point
	ID         ID
	Config     *AccountList
	Proposed   *AccountList
	ManagerStr string
	SSHPub     string
	Cothority  *sda.EntityList
	skipchain  *skipchain.Client
	root       *skipchain.SkipBlock
	data       *skipchain.SkipBlock
}

// NewIdentity starts a new identity that can contain multiple managers with
// different accounts
func NewIdentity(cothority *sda.EntityList, majority int, owner, sshPub string) *Identity {
	client := sda.NewClient(ServiceName)
	kp := config.NewKeyPair(network.Suite)
	return &Identity{
		Client:     client,
		Private:    kp.Secret,
		Public:     kp.Public,
		Config:     NewAccountList(majority, kp.Public, owner, sshPub),
		ManagerStr: owner,
		SSHPub:     sshPub,
		Cothority:  cothority,
		skipchain:  skipchain.NewClient(),
	}
}

// NewIdentityFromCothority searches for a given cothority
func NewIdentityFromCothority(el *sda.EntityList, id ID) (*Identity, error) {
	iden := &Identity{
		Client:    sda.NewClient(ServiceName),
		Cothority: el,
		ID:        id,
	}
	err := iden.ConfigUpdate()
	if err != nil {
		return nil, err
	}
	return iden, nil
}

// NewClientFromStream reads the configuration of that client from
// any stream
func NewIdentityFromStream(in io.Reader) (*Identity, error) {
	data, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, err
	}
	_, id, err := network.UnmarshalRegistered(data)
	if err != nil {
		return nil, err
	}
	return id.(*Identity), nil
}

// SaveToStream stores the configuration of the client to a stream
func (i *Identity) SaveToStream(out io.Writer) error {
	data, err := network.MarshalRegisteredType(i)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

// AttachToIdentity proposes to attach it to an existing Identity
func (i *Identity) AttachToIdentity(ID ID) error {
	i.ID = ID
	err := i.ConfigUpdate()
	if err != nil {
		return err
	}
	if _, exists := i.Config.Owners[i.ManagerStr]; exists {
		return errors.New("Adding with an existing account-name")
	}
	confPropose := i.Config.Copy()
	confPropose.Owners[i.ManagerStr] = &Owner{i.Public}
	confPropose.Data[i.ManagerStr] = i.SSHPub
	err = i.ConfigNewPropose(confPropose)
	if err != nil {
		return err
	}
	return nil
}

// CreateIdentity asks the identityService to create a new Identity
func (i *Identity) CreateIdentity() error {
	msg, err := i.Send(i.Cothority.GetRandom(), &AddIdentity{i.Config, i.Cothority})
	if err != nil {
		return err
	}
	air := msg.Msg.(AddIdentityReply)
	i.root = air.Root
	i.data = air.Data
	i.ID = ID(i.data.Hash)

	return nil
}

// ConfigNewPropose collects new IdentityLists and waits for confirmation with
// ConfigNewVote
func (i *Identity) ConfigNewPropose(il *AccountList) error {
	_, err := i.Send(i.Cothority.GetRandom(), &PropagateProposition{i.ID, il})
	i.Proposed = il
	return err
}

// ConfigNewCheck verifies if there is a new configuration awaiting that
// needs approval from clients
func (i *Identity) ConfigNewCheck() error {
	msg, err := i.Send(i.Cothority.GetRandom(), &ConfigNewCheck{
		ID:          i.ID,
		AccountList: nil,
	})
	if err != nil {
		return err
	}
	cnc := msg.Msg.(ConfigNewCheck)
	i.Proposed = cnc.AccountList
	return nil
}

// VoteProposed calls the 'accept'-vote on the current propose-configuration
func (i *Identity) VoteProposed(accept bool) error {
	if i.Proposed == nil {
		return errors.New("No proposed config")
	}
	h, err := i.Proposed.Hash()
	if err != nil {
		return err
	}
	return i.ConfigNewVote(h, accept)
}

// ConfigNewVote sends a vote (accept or reject) with regard to a new configuration
func (i *Identity) ConfigNewVote(configID crypto.HashID, accept bool) error {
	dbg.Lvl3("Voting on", i.Proposed.Owners)
	hash, err := i.Proposed.Hash()
	if err != nil {
		return err
	}
	sig, err := crypto.SignSchnorr(network.Suite, i.Private, hash)
	if err != nil {
		return err
	}
	msg, err := i.Send(i.Cothority.GetRandom(), &Vote{
		ID:        i.ID,
		Signer:    i.ManagerStr,
		Signature: &sig,
	})
	if err != nil {
		return err
	}
	if msg == nil {
		dbg.Lvl3("Threshold not reached")
	} else {
		sb, ok := msg.Msg.(skipchain.SkipBlock)
		if ok {
			dbg.Lvl3("Threshold reached and signed")
			i.data = &sb
			i.Config = i.Proposed
		} else {
			return errors.New(msg.Msg.(sda.StatusRet).Status)
		}
	}
	return nil
}

// ConfigUpdate asks if there is any new config available that has already
// been approved by others and updates the local configuration
func (i *Identity) ConfigUpdate() error {
	if i.Cothority == nil || len(i.Cothority.List) == 0 {
		return errors.New("Didn't find any list in the cothority")
	}
	msg, err := i.Send(i.Cothority.GetRandom(), &ConfigUpdate{ID: i.ID})
	if err != nil {
		return err
	}
	cu := msg.Msg.(ConfigUpdate)
	// TODO - verify new config
	i.Config = cu.AccountList
	return nil
}
