/*
Package darc in most of our projects we need some kind of access control to protect resources. Instead of having a simple password
or public key for authentication, we want to have access control that can be:
evolved with a threshold number of keys
be delegated
So instead of having a fixed list of identities that are allowed to access a resource, the goal is to have an evolving
description of who is allowed or not to access a certain resource.
*/
package darc

import (
	"errors"
	"fmt"

	"bytes"
	"crypto/sha256"

	"github.com/dedis/protobuf"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/ed25519"
	"gopkg.in/dedis/crypto.v0/sign"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// NewDarc initialises a darc-structure given its owners and users
func NewDarc(owners *[]*Identity, users *[]*Identity, desc []byte) *Darc {
	var ow, us []*Identity
	if owners != nil {
		ow = append(ow, *owners...)
	}
	if users != nil {
		us = append(us, *users...)
	}
	return &Darc{
		Owners:      &ow,
		Users:       &us,
		Version:     0,
		Description: &desc,
		Signature:   nil,
	}
}

// Copy all the fields of a Darc except the signature
func (d *Darc) Copy() *Darc {
	dCopy := &Darc{
		Version: d.Version,
		BaseID:  d.BaseID,
	}
	if d.Owners != nil {
		owners := append([]*Identity{}, *d.Owners...)
		dCopy.Owners = &owners
	}
	if d.Users != nil {
		users := append([]*Identity{}, *d.Users...)
		dCopy.Users = &users
	}
	if d.Description != nil {
		desc := *(d.Description)
		dCopy.Description = &desc
	}
	return dCopy
}

// ToProto returns a protobuf representation of the Darc-structure.
// We copy a darc first to keep only invariant fields which exclude
// the delegation signature.
func (d *Darc) ToProto() ([]byte, error) {
	dc := d.Copy()
	b, err := protobuf.Encode(dc)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// NewDarcFromProto interprets a protobuf-representation of the darc and
// returns a created Darc.
func NewDarcFromProto(protoDarc []byte) *Darc {
	d := &Darc{}
	protobuf.Decode(protoDarc, d)
	return d
}

// GetID returns the hash of the protobuf-representation of the Darc as its Id.
func (d *Darc) GetID() ID {
	// get protobuf representation
	protoDarc, err := d.ToProto()
	if err != nil {
		log.Error("couldn't convert darc to protobuf for computing its id: " + err.Error())
		return nil
	}
	// compute the hash
	h := sha256.New()
	if _, err := h.Write(protoDarc); err != nil {
		log.Error(err)
		return nil
	}
	hash := h.Sum(nil)
	return ID(hash)
}

// GetBaseID returns the base ID or the ID of this darc if its the
// first darc.
func (d *Darc) GetBaseID() ID {
	if d.Version == 0 {
		return d.GetID()
	}
	return *d.BaseID
}

// AddUser adds a given user to the list of Users in the Darc
// Use as 'Darc.AddUser(user)'
func (d *Darc) AddUser(user *Identity) []*Identity {
	var users []*Identity
	if d.Users != nil {
		users = *d.Users
	}
	users = append(users, user)
	d.Users = &users
	return *d.Users
}

// AddOwner adds a given user to the list of Users in the Darc
// Use as 'Darc.AddUser(user)'
func (d *Darc) AddOwner(owner *Identity) []*Identity {
	var owners []*Identity
	if d.Owners != nil {
		owners = *d.Owners
	}
	owners = append(owners, owner)
	d.Owners = &owners
	return owners
}

// RemoveUser removes s a given user from the list of Users in the Darc
// Use as 'Darc.RemoveUser(user)'
func (d *Darc) RemoveUser(user *Identity) ([]*Identity, error) {
	var userIndex = -1
	var users []*Identity
	if d.Users == nil {
		return nil, errors.New("users list of the darc is empty")
	}
	users = *d.Users
	for i, u := range *d.Users {
		if u == user {
			userIndex = i
		}
	}
	if userIndex == -1 { // If initial index has not changed
		return nil, errors.New("user cannot be removed because it is not in the darc")
	}
	// Actually removing the userIndexth element
	users = append(users[:userIndex], users[userIndex+1:]...)
	d.Users = &users
	return *d.Users, nil
}

// SetEvolution evolves a darc, the latest valid darc needs to sign the new darc.
// Only if one of the previous owners signs off on the new darc will it be
// valid and accepted to sign on behalf of the old darc. The path can be nil
// unless if the previousOwner is an Ed25519Signer and found directly in the
// previous darc.
func (d *Darc) SetEvolution(prevd *Darc, pth *SignaturePath, prevOwner *Signer) error {
	d.Signature = nil
	d.Version = prevd.Version + 1
	if pth == nil {
		prevOwnerID := Identity{Ed25519: &IdentityEd25519{Point: prevOwner.Ed25519.Point}}
		pth = NewSignaturePath([]*Darc{prevd}, prevOwnerID, Owner)
	}
	if prevd.BaseID == nil {
		id := prevd.GetID()
		d.BaseID = &id
	}
	sig, err := NewDarcSignature(d.GetID(), pth, prevOwner)
	if err != nil {
		return errors.New("error creating a darc signature for evolution: " + err.Error())
	}
	if sig != nil {
		d.Signature = sig
	} else {
		return errors.New("the resulting signature is nil")
	}
	return nil
}

// IncrementVersion updates the version number of the Darc
func (d *Darc) IncrementVersion() {
	d.Version++
}

// Verify returns nil if the verification is OK, or an error
// if something is wrong.
func (d Darc) Verify() error {
	if d.Version == 0 {
		return nil
	}
	if d.Signature == nil || len(d.Signature.Signature) == 0 {
		return errors.New("No signature available")
	}
	latest, err := d.GetLatest()
	if err != nil {
		return err
	}
	if err := d.Signature.SignaturePath.Verify(Owner); err != nil {
		return err
	}
	return d.Signature.Verify(d.GetID(), latest)
}

// GetLatest searches for the previous darc in the signature and returns an
// error if it's not an evolving darc.
func (d Darc) GetLatest() (*Darc, error) {
	if d.Signature == nil {
		return nil, nil
	}
	if d.Signature.SignaturePath.Darcs == nil {
		return nil, errors.New("signature but no darcs")
	}
	prev := (*d.Signature.SignaturePath.Darcs)[0]
	if prev.Version+1 != d.Version {
		return nil, errors.New("not clean evolution - version mismatch")
	}
	return prev, nil
}

func (d Darc) String() string {
	ret := fmt.Sprintf("this[base]: %x[%x]\nVersion: %d", d.GetID(), d.GetBaseID(), d.Version)
	for idStr, list := range map[string]*[]*Identity{"owner": d.Owners, "user": d.Users} {
		if list != nil {
			for ui, u := range *list {
				if u.Ed25519 != nil {
					ret += fmt.Sprintf("\n%sEd25519[%d] = %s", idStr, ui, u.Ed25519.Point)
				}
				if u.Darc != nil {
					ret += fmt.Sprintf("\n%sDarc[%d] = %x", idStr, ui, u.Darc.ID)
				}
			}
		}
	}
	return ret
}

// IsNull returns true if this DarcID is not initialised.
func (di ID) IsNull() bool {
	return di == nil
}

// Equal compares with another DarcID.
func (di ID) Equal(other ID) bool {
	return bytes.Equal([]byte(di), []byte(other))
}

// NewDarcSignature creates a new darc signature by hashing (PathMsg + msg),
// where PathMsg is retrieved from a given signature path, and signing it
// with a given signer.
func NewDarcSignature(msg []byte, sigpath *SignaturePath, signer *Signer) (*Signature, error) {
	if sigpath == nil || signer == nil {
		return nil, errors.New("signature path or signer are missing")
	}
	hash, err := sigHash(sigpath, msg)
	if err != nil {
		return nil, err
	}
	sig, err := signer.Sign(hash)
	if err != nil {
		return nil, errors.New("failed to sign a hash")
	}
	return &Signature{Signature: sig, SignaturePath: *sigpath}, nil
}

// Verify returns nil if the signature is correct, or an error
// if something is wrong.
func (ds *Signature) Verify(msg []byte, base *Darc) error {
	if base == nil {
		return errors.New("Base-darc is missing")
	}
	if ds.SignaturePath.Darcs == nil || len(*ds.SignaturePath.Darcs) == 0 {
		return errors.New("No path stored in signaturepath")
	}
	sigBase := (*ds.SignaturePath.Darcs)[0].GetID()
	if !sigBase.Equal(base.GetID()) {
		return errors.New("Base-darc is not at root of path")
	}
	hash, err := sigHash(&ds.SignaturePath, msg)
	if err != nil {
		return err
	}
	// TODO: use correct signer interface
	pub := ds.SignaturePath.Signer.Ed25519.Point
	return sign.VerifySchnorr(network.Suite, pub, hash, ds.Signature)
}

// sigHash returns the hash needed to create or verify a DarcSignature.
func sigHash(sigpath *SignaturePath, msg []byte) ([]byte, error) {
	h := sha256.New()
	msgpath := sigpath.GetPathMsg()
	_, err := h.Write(msgpath)
	if err != nil {
		return nil, err
	}
	_, err = h.Write(msg)
	if err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// NewSignaturePath returns an initialized SignaturePath structure.
func NewSignaturePath(darcs []*Darc, signer Identity, role Role) *SignaturePath {
	return &SignaturePath{
		Darcs:  &darcs,
		Signer: signer,
		Role:   role,
	}
}

// GetPathMsg returns the concatenated Darc-IDs of the path.
func (sigpath *SignaturePath) GetPathMsg() []byte {
	if sigpath == nil {
		return []byte{}
	}
	var path []byte
	for _, darc := range *sigpath.Darcs {
		path = append(path, darc.GetID()...)
	}
	return path
}

// Verify makes sure that the path is a correctly evolving one (each next
// darc should be referenced by the previous one) and that the signer
// is present in the last darc.
func (sigpath *SignaturePath) Verify(role Role) error {
	if len(*sigpath.Darcs) == 0 {
		return errors.New("no path stored")
	}
	var previous *Darc
	for n, d := range *sigpath.Darcs {
		if d == nil {
			return errors.New("null pointer in path list")
		}
		if previous != nil {
			// Check if its an evolving darc
			latest, err := d.GetLatest()
			if err != nil {
				return errors.New("found incorrect darc in chain")
			}
			if latest != nil {
				log.Lvlf2("Verifying evolution from %x", d.GetID())
				if err := d.Verify(); err != nil {
					return errors.New("not correct evolution of darcs in path: " + err.Error())
				}
			}
			if latest == nil || bytes.Compare(latest.GetID(), previous.GetID()) != 0 {
				// The darc link can only come from an owner of the first darc. Afterwards
				// darc links have to be user-links.
				found := false
				if role == Owner && n == 1 {
					if previous.Owners != nil {
						for _, id := range *previous.Owners {
							if id.Darc != nil && id.Darc.ID.Equal(d.GetID()) {
								found = true
								break
							}
						}
					} else {
						return errors.New("no owners defined in base darc")
					}
				} else {
					if previous.Users != nil {
						for _, id := range *previous.Users {
							if id.Darc != nil && id.Darc.ID.Equal(d.GetID()) {
								found = true
								break
							}
						}
					} else {
						return errors.New("no users defined for user signature")
					}
				}
				if !found {
					return fmt.Errorf("didn't find valid darc-link in chain at position %d", n)
				}
			}
		}
		previous = d
	}
	if role == User {
		for _, id := range *previous.Users {
			if id.Ed25519 != nil &&
				id.Ed25519.Point.Equal(sigpath.Signer.Ed25519.Point) {
				return nil
			}
		}
	} else {
		for _, id := range *previous.Owners {
			if id.Ed25519 != nil &&
				id.Ed25519.Point.Equal(sigpath.Signer.Ed25519.Point) {
				return nil
			}
		}
	}
	return errors.New("didn't find signer in last darc of path")
}

// Sign returns a signature in bytes for a given messages by the signer
func (s *Signer) Sign(msg []byte) ([]byte, error) {
	if msg == nil {
		return nil, errors.New("nothing to sign, message is empty")
	}
	if s.Ed25519 != nil {
		key, err := s.GetPrivate()
		if err != nil {
			return nil, errors.New("could not retrieve a private key")
		}
		return sign.Schnorr(ed25519.NewAES128SHA256Ed25519(false), key, msg)
	}
	return nil, errors.New("signer is of unknown type")
}

// GetPrivate returns the private key, if one exists.
func (s *Signer) GetPrivate() (abstract.Scalar, error) {
	if s.Ed25519 != nil {
		if s.Ed25519.Secret != nil {
			return s.Ed25519.Secret, nil
		}
		return nil, errors.New("signer lacks a private key")
	}
	return nil, errors.New("signer is of unknown type")
}

// NewIdentity creates an identity with either a link to another darc
// or an Ed25519 identity (containing a point). You're only allowed
// to give either a darc or a point, but not both.
func NewIdentity(darc *IdentityDarc, ed *IdentityEd25519) (*Identity, error) {
	if darc != nil && ed != nil {
		return nil, errors.New("cannot have both darc and ed25519 point in one identity")
	}
	if darc == nil && ed == nil {
		return nil, errors.New("give one of darc or point")
	}
	return &Identity{
		Darc:    darc,
		Ed25519: ed,
	}, nil
}

// NewDarcIdentity creates a new darc identity struct given a darcid
func NewDarcIdentity(id ID) *IdentityDarc {
	return &IdentityDarc{
		ID: id,
	}
}

// NewEd25519Identity creates a new ed25519 identity given a public-key point
func NewEd25519Identity(point abstract.Point) *IdentityEd25519 {
	return &IdentityEd25519{
		Point: point,
	}
}

// NewEd25519Signer initializes a new Ed25519Signer given a public and private keys.
// If any of the given values is nil or both are nil, then a new key pair is generated.
func NewEd25519Signer(point abstract.Point, secret abstract.Scalar) *Ed25519Signer {
	if point == nil || secret == nil {
		kp := config.NewKeyPair(network.Suite)
		point, secret = kp.Public, kp.Secret
	}
	return &Ed25519Signer{
		Point:  point,
		Secret: secret,
	}
}
