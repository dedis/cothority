package lib

import (
	"fmt"

	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/dedis/cothority/skipchain"
)

func init() {
	network.RegisterMessages(Master{}, Link{})
}

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

// GetMaster retrieves the master object from its skipchain.
func GetMaster(roster *onet.Roster, id skipchain.SkipBlockID) (*Master, error) {
	client := skipchain.NewClient()

	block, err := client.GetSingleBlockByIndex(roster, id, 1)
	if err != nil {
		return nil, err
	}

	transaction := UnmarshalTransaction(block.Data)
	if transaction == nil && transaction.Master == nil {
		return nil, fmt.Errorf("no master structure in %s", id.Short())
	}
	return transaction.Master, nil
}

// Links returns all the links appended to the master skipchain.
func (m *Master) Links() ([]*Link, error) {
	client := skipchain.NewClient()

	block, err := client.GetSingleBlockByIndex(m.Roster, m.ID, 0)
	if err != nil {
		return nil, err
	}

	links := make([]*Link, 0)
	for {
		transaction := UnmarshalTransaction(block.Data)
		if transaction != nil && transaction.Link != nil {
			links = append(links, transaction.Link)
		}

		if len(block.ForwardLink) <= 0 {
			break
		}
		block, _ = client.GetSingleBlock(m.Roster, block.ForwardLink[0].To)
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
