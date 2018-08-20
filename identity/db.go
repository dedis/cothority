package identity

import (
	"sync"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/anon"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

// DB-versioning, allows propoer passage from one version to another. This example
// shows how to handle the case where there was no previous versioning in the
// database, and we already have two possible incompatible versions out there,
// version 0a and 0b. Version 1 will be the correct one.
//
// loadVersion starts trying to get version 1, but only if the database returns
// the correct version. If the version is 0 (or nonexistant), then it calls first
// updateFrom0a, if that fails it tries updateFrom0b and if all fails it returns an error.
//
// In case of a future incompatible change, one would have to add `updateFrom1` which
// would call `updateFrom0` if the version < 1. `updateFrom1` would return `storage2`.
// And the `Service` structure would use `storage2` for the up-to-date storage
// version.

const dbVersion = 1

var storageKey = []byte("storage")
var versionKey = []byte("version")

func loadVersion(l onet.ContextDB) (*storage1, error) {
	vers, err := l.LoadVersion()
	if err != nil {
		return nil, err
	}
	if vers < dbVersion {
		storage, err := updateFrom0(l, vers)
		if err != nil {
			return nil, err
		}

		// TODO: this is really ugly...
		if c, ok := l.(*onet.Context); ok {
			err = c.Save(storageKey, storage)
		}
		err = l.SaveVersion(dbVersion)
		return storage, err
	}
	sInt, err := l.Load(storageKey)
	if err != nil {
		return nil, err
	}
	if sInt == nil {
		return &storage1{}, nil
	}
	return sInt.(*storage1), err
}

// storage1 holds the map to the storages so it can be marshaled.
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
