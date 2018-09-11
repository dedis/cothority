package identity

import (
	"sync"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/anon"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

const dbVersion = 1

var storageKey = []byte("storage")

func init() {
	network.RegisterMessage(&storage1{})
}

// saves all data.
func (s *Service) save() error {
	s.storageMutex.Lock()
	defer s.storageMutex.Unlock()
	err := s.Save(storageKey, s.Storage)
	if err != nil {
		log.Error("Couldn't save data:", err)
	}
	return nil
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	s.Storage = &storage1{}
	defer func() {
		if s.Storage.Identities == nil {
			s.Storage.Identities = make(map[string]*IDBlock)
		}
		if s.Storage.Auth == nil {
			s.Storage.Auth = &authData1{}
		}
		if len(s.Storage.Auth.Pins) == 0 {
			s.Storage.Auth.Pins = map[string]bool{}
		}
		if len(s.Storage.Auth.Nonces) == 0 {
			s.Storage.Auth.Nonces = map[string]bool{}
		}
		if s.Storage.Auth.Sets == nil {
			s.Storage.Auth.Sets = []anonSet1{}
		}
		if s.Storage.Auth.AdminKeys == nil {
			s.Storage.Auth.AdminKeys = []kyber.Point{}
		}
	}()
	ver, err := s.LoadVersion()
	if err != nil {
		return err
	}
	if ver < dbVersion {
		// There are two version 0s...
		s.Storage, err = updateFrom0(s, ver)
		if err != nil {
			return err
		}
		if err = s.save(); err != nil {
			return err
		}
		return s.SaveVersion(dbVersion)
	}
	buf, err := s.LoadRaw(storageKey)
	if err != nil {
		return err
	}
	if len(buf) <= 16 {
		return nil
	}
	return protobuf.DecodeWithConstructors(buf[16:], s.Storage,
		network.DefaultConstructors(cothority.Suite))
}

// storage2 holds the map to the storages so it can be marshaled.
type storage1 struct {
	Identities map[string]*IDBlock
	// The key that is stored in the skipchain service to authenticate
	// new blocks.
	SkipchainKeyPair *key.Pair
	// Auth is a list of all authentications allowed for this service
	Auth *authData1
}

type authData1 struct {
	// set of Pins and keys
	Pins map[string]bool
	// Sets of public keys to verify linkable ring signatures
	Sets []anonSet1
	// list of public Keys to verify simple authentication with Schnorr sig
	Keys []kyber.Point
	// list of AdminKeys
	AdminKeys []kyber.Point
	// set of Nonces
	Nonces map[string]bool
}

type anonSet1 struct {
	Set anon.Set
}

// updateFrom0 tries first to load the oldest version of the database, then the
// somewhat newer one.
func updateFrom0(l onet.ContextDB, vers int) (*storage1, error) {
	s := &storage1{}
	err := updateFrom0a(l, s)
	if err == nil {
		return s, nil
	}
	return s, updateFrom0b(l, s)
}

//
// This is the oldest version of the database.
//

type storage0a struct {
	Identities map[string]*idBlock0
	// OldSkipchainKey is a placeholder for protobuf being able to read old config-files
	OldSkipchainKey kyber.Scalar
	// The key that is stored in the skipchain service to authenticate
	// new blocks.
	SkipchainKeyPair *key.Pair
	// Auth is a list of all authentications allowed for this service
	Auth *authData1
}

type idBlock0 struct {
	sync.Mutex
	Latest          *Data
	Proposed        *Data
	LatestSkipblock *skipchain.SkipBlock
}

func updateFrom0a(l onet.ContextDB, s *storage1) error {
	s0Buf, err := l.LoadRaw(storageKey)
	if err != nil {
		return err
	}
	if len(s0Buf) <= 16 {
		return nil
	}
	s0 := &storage0a{}
	err = protobuf.DecodeWithConstructors(s0Buf[16:], s0, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return err
	}
	s.Identities = make(map[string]*IDBlock)
	for k, v := range s0.Identities {
		s.Identities[k] = &IDBlock{
			Latest:          v.Latest,
			Proposed:        v.Proposed,
			LatestSkipblock: v.LatestSkipblock,
		}
	}
	s.SkipchainKeyPair = s0.SkipchainKeyPair
	return nil
}

//
// This is a somewhat newer version of the database.
//

type storage0b struct {
	Identities map[string]*IDBlock
	// OldSkipchainKey is a placeholder for protobuf being able to read old config-files
	OldSkipchainKey kyber.Scalar
	// The key that is stored in the skipchain service to authenticate
	// new blocks.
	SkipchainKeyPair *key.Pair
	// Auth is a list of all authentications allowed for this service
	Auth *authData1
}

func updateFrom0b(l onet.ContextDB, s *storage1) error {
	s0Buf, err := l.LoadRaw(storageKey)
	if err != nil {
		return err
	}
	if len(s0Buf) <= 16 {
		return nil
	}
	s0 := &storage0b{}
	err = protobuf.DecodeWithConstructors(s0Buf[16:], s0, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return err
	}
	s.Identities = s0.Identities
	s.SkipchainKeyPair = s0.SkipchainKeyPair
	s.Auth = s0.Auth
	return nil
}
