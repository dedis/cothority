package identity

import (
	"encoding/binary"
	"sort"

	"fmt"
	"strings"

	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
)

// ID represents one skipblock and corresponds to its Hash
type ID skipchain.SkipBlockID

// AccountList holds the information about all accounts belonging to an
// identity
type AccountList struct {
	Threshold int
	Listeners []*network.Entity
	Owners    map[string]*Owner
	Data      map[string]string
}

// NewAccountList returns a new List with the first owner initialised
func NewAccountList(threshold int, pub abstract.Point, owner string, sshPub string) *AccountList {
	return &AccountList{
		Threshold: threshold,
		Owners:    map[string]*Owner{owner: &Owner{pub}},
		Listeners: []*network.Entity{},
		Data:      map[string]string{owner: sshPub},
	}
}

// Copy makes a deep copy of the AccountList
func (al *AccountList) Copy() *AccountList {
	b, err := network.MarshalRegisteredType(al)
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
func (al *AccountList) Hash() (crypto.HashID, error) {
	hash := network.Suite.Hash()
	err := binary.Write(hash, binary.LittleEndian, int32(al.Threshold))
	if err != nil {
		return nil, err
	}
	for _, e := range al.Listeners {
		b, err := network.MarshalRegisteredType(e)
		if err != nil {
			return nil, err
		}
		_, err = hash.Write(b)
		if err != nil {
			return nil, err
		}
	}
	var owners []string
	for s := range al.Owners {
		owners = append(owners, s)
	}
	sort.Strings(owners)
	for _, s := range owners {
		_, err = hash.Write([]byte(s))
		if err != nil {
			return nil, err
		}
		_, err = hash.Write([]byte(al.Data[s]))
		if err != nil {
			return nil, err
		}
		b, err := network.MarshalRegisteredType(al.Owners[s])
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

// String returns a nicely formatted output of the AccountList
func (al *AccountList) String() string {
	var owners []string
	for n := range al.Owners {
		owners = append(owners, fmt.Sprintf("Owner: %s\nData: %s",
			n, al.Data[n]))
	}
	return fmt.Sprintf("Threshold: %d\n%s", al.Threshold,
		strings.Join(owners, "\n"))
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
	ID        ID
	Public    abstract.Point
	PublicSSH string
}

// ConfigNewCheck verifies if a new config is available. On sending,
// the ID is given, on receiving, the AccountList is given.
type ConfigNewCheck struct {
	ID          ID
	AccountList *AccountList
}

// ConfigUpdate verifies if a new update is available. On sending,
// the ID is given, on receiving, the AccountList is given.
type ConfigUpdate struct {
	ID          ID
	AccountList *AccountList
}

// Vote sends the signature for a specific IdentityList. It replies nil
// if the threshold hasn't been reached, or the new SkipBlock
type Vote struct {
	ID        ID
	Signer    string
	Signature *crypto.SchnorrSig
}

// Messages to be sent from one identity to another

// PropagateIdentity sends a new identity to other identityServices
type PropagateIdentity struct {
	*Storage
}

// PropagateProposition sends a new proposition to be stored in all identities
type PropagateProposition struct {
	ID ID
	*AccountList
}

// UpdateSkipBlock asks the service to fetch the latest SkipBlock
type UpdateSkipBlock struct {
	ID     ID
	Latest *skipchain.SkipBlock
}
