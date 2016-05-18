package identity

import (
	"encoding/binary"
	"sort"

	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
)

type IdentityID skipchain.SkipBlockID

type AccountList struct {
	Threshold int
	Listeners []*network.Entity
	Owners    map[string]*Owner
	Data      map[abstract.Point]string
}

func NewAccountList(threshold int, pub abstract.Point, owner string, sshPub string) *AccountList {
	return &AccountList{
		Threshold:  threshold,
		Owners:    map[string]*Owner{owner: &Owner{pub}},
		Listeners: []*network.Entity{},
		Data:      map[abstract.Point]string{pub: sshPub},
	}
}

// Copy makes a deep copy of the AccountList
func (il *AccountList) Copy() *AccountList {
	b, err := network.MarshalRegisteredType(il)
	if err != nil {
		dbg.Error("Couldn't marshal AccountList:", err)
		return nil
	}
	_, msg, err := network.UnmarshalRegisteredType(b, network.DefaultConstructors(network.Suite))
	if err != nil {
		dbg.Error("Couldn't unmarshal AccountList:", err)
	}
	ilNew := msg.(AccountList)
	return &ilNew
}

// Hash makes a cryptographic hash of the configuration-file - this
// can be used as an ID
func (il *AccountList) Hash() (crypto.HashID, error) {
	hash := network.Suite.Hash()
	err := binary.Write(hash, binary.LittleEndian, int32(il.Threshold))
	if err != nil {
		return nil, err
	}
	for _, e := range il.Listeners {
		b, err := network.MarshalRegisteredType(e)
		if err != nil {
			return nil, err
		}
		_, err = hash.Write(b)
		if err != nil {
			return nil, err
		}
	}
	owners := []string{}
	for s := range il.Owners {
		owners = append(owners, s)
	}
	sort.Strings(owners)
	for _, s := range owners {
		_, err = hash.Write([]byte(s))
		if err != nil {
			return nil, err
		}
		_, err = hash.Write([]byte(il.Data[il.Owners[s].Point]))
		if err != nil {
			return nil, err
		}
		b, err := network.MarshalRegisteredType(il.Owners[s])
		if err != nil {
			return nil, err
		}
		_, err = hash.Write(b)
		if err != nil {
			return nil, err
		}
	}
	return hash.Sum(nil), nil
}

// Owner has write-access to the IdentityList if the threshold is given
type Owner struct {
	Point abstract.Point
}

// Messages between the Client-API and the Service

// AddIdentity starts a new identity-skipchain
type AddIdentity struct {
	*AccountList
	*sda.EntityList
}

// AddIdentityReply is the reply when a new Identity has been added
type AddIdentityReply struct {
	Root *skipchain.SkipBlock
	Data *skipchain.SkipBlock
}

// AttachToIdentity requests to attach the manager-device to an
// existing identity
type AttachToIdentity struct {
	ID        IdentityID
	Public    abstract.Point
	PublicSSH string
}

// ConfigNewCheck verifies if a new config is available. On sending,
// the ID is given, on receiving, the AccountList is given.
type ConfigNewCheck struct {
	ID          IdentityID
	AccountList *AccountList
}

// ConfigUpdate verifies if a new update is available. On sending,
// the ID is given, on receiving, the AccountList is given.
type ConfigUpdate struct {
	ID          IdentityID
	AccountList *AccountList
}

// Vote sends the signature for a specific IdentityList. It replies nil
// if the threshold hasn't been reached, or the new SkipBlock
type Vote struct {
	ID        IdentityID
	Signer    string
	Signature *crypto.SchnorrSig
}

// Messages to be sent from one identity to another

// PropagateIdentity sends a new identity to other identityServices
type PropagateIdentity struct {
	*IdentityStorage
}

// PropagateProposition sends a new proposition to be stored in all identities
type PropagateProposition struct {
	ID IdentityID
	*AccountList
}

// UpdateSkipBlock asks the service to fetch the latest SkipBlock
type UpdateSkipBlock struct {
	ID IdentityID
}