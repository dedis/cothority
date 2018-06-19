package lib

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
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

func DebugDumpChain(s *skipchain.Service, w io.Writer, id skipchain.SkipBlockID) {
	fmt.Fprintln(w, "-- dump start --")
	db := s.GetDB()

	sb := db.GetByID(id)
	if sb == nil {
		fmt.Fprintf(w, "chain not found: %v\n", id)
	} else {
		for {
			fmt.Fprintf(w, "block: %x\n", sb.Hash)
			fmt.Fprintf(w, "  fwd:\n")
			for _, x := range sb.ForwardLink {
				ok := "ok"
				if db.GetByID(x.To) == nil {
					ok = "NOT FOUND"
				}
				fmt.Fprintf(w, "       %x %v\n", x.To, ok)
			}

			fmt.Fprintf(w, " back:\n")
			for _, x := range sb.BackLinkIDs {
				ok := "ok"
				if db.GetByID(x) == nil {
					ok = "NOT FOUND"
				}
				fmt.Fprintf(w, "       %x %v\n", x, ok)
			}

			if sb.GetForwardLen() == 0 {
				break
			}

			want := sb.GetForward(0).To
			sb = db.GetByID(want)
			if sb == nil {
				fmt.Fprintf(w, "missing block: %x\n", want)
				break
			}
			fmt.Fprintln(w)
		}
	}
	fmt.Fprintln(w, "-- dump done --")
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
		log.LLvlf4("bad forward link: starting from %x", id)
		buf := &bytes.Buffer{}
		DebugDumpChain(s, buf, id)
		log.LLvl4(string(buf.Bytes()))
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
	block, err := s.GetSingleBlockByIndex(
		&skipchain.GetSingleBlockByIndex{Genesis: m.ID, Index: 0},
	)
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
