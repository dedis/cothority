package lib

import (
	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
)

// Master is the foundation object of the entire service.
// It contains mission critical information that can only be accessed and
// set by an administrators.
type Master struct {
	ID     skipchain.SkipBlockID // ID is the hash of the genesis skipblock.
	Roster *onet.Roster          // Roster is the set of responsible conodes.

	Admins []uint32 // Admins is the list of administrators.

	Key kyber.Point // Key is the front-end public key.
}

// Link is a wrapper around the genesis Skipblock identifier of an
// election. Every newly created election adds a new link to the master Skipchain.
type Link struct {
	ID skipchain.SkipBlockID
}

func init() {
	network.RegisterMessages(Master{}, Link{})
}

// FetchMaster retrieves the master object from its skipchain.
func FetchMaster(roster *onet.Roster, id skipchain.SkipBlockID) (*Master, error) {
	chain, err := chain(roster, id)
	if err != nil {
		return nil, err
	}

	_, blob, _ := network.Unmarshal(chain[1].Data, cothority.Suite)
	return blob.(*Master), nil
}

// GenChain generates a master skipchain with the given list of links.
func (m *Master) GenChain(links ...skipchain.SkipBlockID) {
	chain, _ := New(m.Roster, nil)

	m.ID = chain.Hash
	m.Store(m)

	for _, link := range links {
		m.Store(&Link{ID: link})
	}
}

// Store appends a given structure to the master skipchain.
func (m *Master) Store(data interface{}) error {
	chain, err := chain(m.Roster, m.ID)
	if err != nil {
		return err
	}

	latest := chain[len(chain)-1]
	if _, err := client.StoreSkipBlock(latest, m.Roster, data); err != nil {
		return err
	}
	return nil
}

// Links returns all the links appended to the master skipchain.
func (m *Master) Links() ([]*Link, error) {
	chain, err := chain(m.Roster, m.ID)
	if err != nil {
		return nil, err
	}

	links := make([]*Link, 0)
	for i := 2; i < len(chain); i++ {
		_, blob, _ := network.Unmarshal(chain[i].Data, cothority.Suite)
		links = append(links, blob.(*Link))
	}
	return links, nil
}

// IsAdmin checks if a given user is part of the administrator list.
func (m *Master) IsAdmin(user uint32) bool {
	for _, admin := range m.Admins {
		if admin == user {
			return true
		}
	}
	return false
}
