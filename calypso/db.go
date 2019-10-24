package calypso

import (
	"sync"

	"go.dedis.ch/cothority/v4"
	"go.dedis.ch/cothority/v4/byzcoin"
	dkgprotocol "go.dedis.ch/cothority/v4/dkg/pedersen"
	dkg "go.dedis.ch/kyber/v4/share/dkg/pedersen"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
	"golang.org/x/xerrors"
)

const dbVersion = 1

// storageKey reflects the data we're storing - we could store more
// than one structure.
var storageKey = []byte("storage")

// storage is used to save all elements of the DKG.
type storage struct {
	AuthorisedByzCoinIDs map[string]bool

	Shared  map[byzcoin.InstanceID]*dkgprotocol.SharedSecret
	Polys   map[byzcoin.InstanceID]*pubPoly
	Rosters map[byzcoin.InstanceID]*onet.Roster
	Replies map[byzcoin.InstanceID]*CreateLTSReply
	DKS     map[byzcoin.InstanceID]*dkg.DistKeyShare

	sync.Mutex
}

// saves all data.
func (s *Service) save() error {
	s.storage.Lock()
	defer s.storage.Unlock()
	err := s.Save(storageKey, s.storage)
	if err != nil {
		log.Error("Couldn't save data:", err)
		return xerrors.Errorf("saving data: %v", err)
	}
	return nil
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	s.storage = &storage{}
	ver, err := s.LoadVersion()
	if err != nil {
		return xerrors.Errorf("loading configuration: %v", err)
	}

	// Make sure we don't have any unallocated maps.
	defer func() {
		if len(s.storage.Polys) == 0 {
			s.storage.Polys = make(map[byzcoin.InstanceID]*pubPoly)
		}
		if len(s.storage.Shared) == 0 {
			s.storage.Shared = make(map[byzcoin.InstanceID]*dkgprotocol.SharedSecret)
		}
		if len(s.storage.Rosters) == 0 {
			s.storage.Rosters = make(map[byzcoin.InstanceID]*onet.Roster)
		}
		if len(s.storage.Replies) == 0 {
			s.storage.Replies = make(map[byzcoin.InstanceID]*CreateLTSReply)
		}
		if len(s.storage.DKS) == 0 {
			s.storage.DKS = make(map[byzcoin.InstanceID]*dkg.DistKeyShare)
		}
		if len(s.storage.AuthorisedByzCoinIDs) == 0 {
			s.storage.AuthorisedByzCoinIDs = make(map[string]bool)
		}
	}()

	// In the future, we'll make database upgrades below.
	if ver < dbVersion {
		// There is no version 0. Save empty storage and update version number.
		if err = s.save(); err != nil {
			return xerrors.Errorf("saving storage: %v", err)
		}
		return cothority.ErrorOrNil(s.SaveVersion(dbVersion), "saving version")
	}
	msg, err := s.Load(storageKey)
	if err != nil {
		return xerrors.Errorf("loading storage: %v", err)
	}
	if msg == nil {
		return nil
	}
	var ok bool
	s.storage, ok = msg.(*storage)
	if !ok {
		return xerrors.New("data of wrong type")
	}
	return nil
}
