package calypso

import (
	"crypto/sha256"
	"errors"
	"sync"

	dkgprotocol "github.com/dedis/cothority/dkg/pedersen"
	dkg "github.com/dedis/kyber/share/dkg/pedersen"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

const dbVersion = 1

// storageKey reflects the data we're storing - we could store more
// than one structure.
var storageKey = []byte("storage")

// storage1 is used to save all elements of the DKG.
type storage1 struct {
	AuthorisedByzCoinIDs map[string]bool

	Shared  map[string]*dkgprotocol.SharedSecret
	Polys   map[string]*pubPoly
	Rosters map[string]*onet.Roster
	Replies map[string]*CreateLTSReply
	DKS     map[string]*dkg.DistKeyShare

	sync.Mutex
}

// Hash computes the hash of an LTS reply, this is an ID that is often used to
// identify the LTS.
func (r *CreateLTSReply) Hash() []byte {
	h := sha256.New()
	h.Write(r.ByzCoinID)
	h.Write(r.InstanceID)
	r.X.MarshalTo(h)
	return h.Sum(nil)
}

// saves all data.
func (s *Service) save() error {
	s.storage.Lock()
	defer s.storage.Unlock()
	err := s.Save(storageKey, s.storage)
	if err != nil {
		log.Error("Couldn't save data:", err)
		return err
	}
	return nil
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	s.storage = &storage1{}
	ver, err := s.LoadVersion()
	if err != nil {
		return err
	}

	// Make sure we don't have any unallocated maps.
	defer func() {
		if len(s.storage.Polys) == 0 {
			s.storage.Polys = make(map[string]*pubPoly)
		}
		if len(s.storage.Shared) == 0 {
			s.storage.Shared = make(map[string]*dkgprotocol.SharedSecret)
		}
		if len(s.storage.Rosters) == 0 {
			s.storage.Rosters = make(map[string]*onet.Roster)
		}
		if len(s.storage.Replies) == 0 {
			s.storage.Replies = make(map[string]*CreateLTSReply)
		}
		if len(s.storage.DKS) == 0 {
			s.storage.DKS = make(map[string]*dkg.DistKeyShare)
		}
	}()

	// In the future, we'll make database upgrades below.
	if ver < dbVersion {
		// There is no version 0. Save empty storage and update version number.
		if err = s.save(); err != nil {
			return err
		}
		return s.SaveVersion(dbVersion)
	}
	msg, err := s.Load(storageKey)
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	var ok bool
	s.storage, ok = msg.(*storage1)
	if !ok {
		return errors.New("data of wrong type")
	}
	return nil
}
