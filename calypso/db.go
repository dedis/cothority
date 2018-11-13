package calypso

import (
	"errors"
	"sync"

	dkgprotocol "github.com/dedis/cothority/dkg/pedersen"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

const dbVersion = 1

// storageKey reflects the data we're storing - we could store more
// than one structure.
var storageKey = []byte("storage")

// storage1 is used to save all elements of the DKG.
type storage1 struct {
	Shared  map[string]*dkgprotocol.SharedSecret
	Polys   map[string]*pubPoly
	Rosters map[string]*onet.Roster
	OLIDs   map[string]skipchain.SkipBlockID

	sync.Mutex
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
		if len(s.storage.OLIDs) == 0 {
			s.storage.OLIDs = make(map[string]skipchain.SkipBlockID)
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
