package lib

import (
	"errors"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/network"

	"go.dedis.ch/cothority/v3/skipchain"
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
func GetMaster(s *skipchain.Service, id skipchain.SkipBlockID) (*Master, error) {
	// Search backwards from the end of the chain, unmarshalling each block until
	// we find the first Master transaction. (There are Link transactions mixed in
	// with the Masters.)
	gb := s.GetDB().GetByID(id)
	if gb == nil {
		return nil, errors.New("No such genesis-block")
	}
	block, err := s.GetDB().GetLatest(gb)
	if err != nil {
		return nil, err
	}

	for block != nil && len(block.BackLinkIDs) != 0 {
		transaction := UnmarshalTransaction(block.Data)
		if transaction == nil {
			continue
		}
		if transaction.Master != nil {
			return transaction.Master, nil
		}
		block = s.GetDB().GetByID(block.BackLinkIDs[0])
	}
	return nil, errors.New("could not find master")
}

// Links returns all the links appended to the master skipchain.
func (m *Master) Links(s *skipchain.Service) ([]*Link, error) {
	search, err := s.GetSingleBlockByIndex(
		&skipchain.GetSingleBlockByIndex{Genesis: m.ID, Index: 0},
	)
	if err != nil {
		return nil, err
	}
	block := search.SkipBlock

	links := make([]*Link, 0)
	for {
		transaction := UnmarshalTransaction(block.Data)
		if transaction != nil && transaction.Link != nil {
			links = append(links, transaction.Link)
		}

		if len(block.ForwardLink) <= 0 {
			break
		}
		block, _ = s.GetSingleBlock(
			&skipchain.GetSingleBlock{ID: block.ForwardLink[0].To},
		)
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
