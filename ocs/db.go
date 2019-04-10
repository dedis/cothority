package ocs

import (
	"errors"
	"sync"

	dkgprotocol "go.dedis.ch/cothority/v3/dkg/pedersen"
	dkg "go.dedis.ch/kyber/v3/share/dkg/pedersen"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
)

const dbVersion = 1

// storageKey reflects the data we're storing - we could store more
// than one structure.
var storageKey = []byte("storage")

// storage is used to save all elements of the DKG.
type storage struct {
	Element map[string]*storageElement

	sync.Mutex
}

type storageElement struct {
	Shared dkgprotocol.SharedSecret
	Polys  pubPoly
	Roster onet.Roster
	DKS    dkg.DistKeyShare
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
	s.storage = &storage{}
	ver, err := s.LoadVersion()
	if err != nil {
		return err
	}

	// Make sure we don't have any unallocated maps.
	defer func() {
		if len(s.storage.Element) == 0 {
			s.storage.Element = make(map[string]*storageElement)
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
	s.storage, ok = msg.(*storage)
	if !ok {
		return errors.New("data of wrong type")
	}
	return nil
}
