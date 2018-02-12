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
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"math/big"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet/log"
	"github.com/dedis/protobuf"
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
	if desc == nil {
		desc = []byte{}
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

// Equal returns true if both darcs point to the same data.
func (d *Darc) Equal(d2 *Darc) bool {
	return d.GetID().Equal(d2.GetID())
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
		if u.Equal(user) {
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
// unless if the previousOwner is an SignerEd25519 and found directly in the
// previous darc.
func (d *Darc) SetEvolution(prevd *Darc, pth *SignaturePath, prevOwner *Signer) error {
	d.Signature = nil
	d.Version = prevd.Version + 1
	if pth == nil {
		pth = NewSignaturePath([]*Darc{prevd}, *prevOwner.Identity(), Owner)
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

// SetEvolutionOnline works like SetEvolution, but doesn't inlcude all the
// necessary data to verify the update in an offline setting. This is enough
// for the use case where the ocs stores all darcs in its internal database.
// The service verifying the signature will have to verify if there is a valid
// path from the previous darc to the signer.
func (d *Darc) SetEvolutionOnline(prevd *Darc, prevOwner *Signer) error {
	d.Signature = nil
	d.Version = prevd.Version + 1
	if prevd.BaseID == nil {
		id := prevd.GetID()
		d.BaseID = &id
	}
	path := &SignaturePath{Signer: *prevOwner.Identity(), Role: Owner}
	sig, err := NewDarcSignature(d.GetID(), path, prevOwner)
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
			for _, u := range *list {
				ret += fmt.Sprintf("\n%s: %s", idStr, u.String())
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
	hash, err := sigpath.SigHash(msg)
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
	hash, err := ds.SignaturePath.SigHash(msg)
	if err != nil {
		return err
	}
	return ds.SignaturePath.Signer.Verify(hash, ds.Signature)
}

// NewSignaturePath returns an initialized SignaturePath structure.
func NewSignaturePath(darcs []*Darc, signer Identity, role Role) *SignaturePath {
	return &SignaturePath{
		Darcs:  &darcs,
		Signer: signer,
		Role:   role,
	}
}

// SigHash returns the hash needed to create or verify a DarcSignature.
func (sigpath *SignaturePath) SigHash(msg []byte) ([]byte, error) {
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

// GetPathMsg returns the concatenated Darc-IDs of the path.
func (sigpath *SignaturePath) GetPathMsg() []byte {
	if sigpath == nil {
		return []byte{}
	}
	var path []byte
	if sigpath.Darcs == nil {
		path = []byte("online")
	} else {
		for _, darc := range *sigpath.Darcs {
			path = append(path, darc.GetID()...)
		}
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
			if sigpath.Signer.Equal(id) {
				return nil
			}
		}
	} else {
		for _, id := range *previous.Owners {
			if sigpath.Signer.Equal(id) {
				return nil
			}
		}
	}
	return errors.New("didn't find signer in last darc of path")
}

// Type returns an integer representing the type of key held in the signer.
// It is compatible with Identity.Type. For an empty signer, -1 is returned.
func (s *Signer) Type() int {
	switch {
	case s.Ed25519 != nil:
		return 1
	case s.X509EC != nil:
		return 2
	default:
		return -1
	}
}

// Identity returns an identity struct with the pre initialised fields
// for the appropriate signer.
func (s *Signer) Identity() *Identity {
	switch s.Type() {
	case 1:
		return &Identity{Ed25519: &IdentityEd25519{Point: s.Ed25519.Point}}
	case 2:
		return &Identity{X509EC: &IdentityX509EC{Public: s.X509EC.Point}}
	default:
		return nil
	}
}

// Sign returns a signature in bytes for a given messages by the signer
func (s *Signer) Sign(msg []byte) ([]byte, error) {
	if msg == nil {
		return nil, errors.New("nothing to sign, message is empty")
	}
	switch s.Type() {
	case 0:
		return nil, errors.New("cannot sign with a darc")
	case 1:
		return s.Ed25519.Sign(msg)
	case 2:
		return s.X509EC.Sign(msg)
	default:
		return nil, errors.New("unknown signer type")
	}
}

// GetPrivate returns the private key, if one exists.
func (s *Signer) GetPrivate() (kyber.Scalar, error) {
	switch s.Type() {
	case 1:
		return s.Ed25519.Secret, nil
	case 0, 2:
		return nil, errors.New("signer lacks a private key")
	default:
		return nil, errors.New("signer is of unknown type")
	}
}

// Equal first checks the type of the two identities, and if they match,
// it returns if their data is the same.
func (id *Identity) Equal(id2 *Identity) bool {
	if id.Type() != id2.Type() {
		return false
	}
	switch id.Type() {
	case 0:
		return id.Darc.Equal(id2.Darc)
	case 1:
		return id.Ed25519.Equal(id2.Ed25519)
	case 2:
		return id.X509EC.Equal(id2.X509EC)
	}
	return false
}

// Type returns an int indicating what type of identity this is. If all
// identities are nil, it returns -1.
func (id *Identity) Type() int {
	switch {
	case id.Darc != nil:
		return 0
	case id.Ed25519 != nil:
		return 1
	case id.X509EC != nil:
		return 2
	}
	return -1
}

// String returns the string representation of the identity
func (id *Identity) String() string {
	switch id.Type() {
	case 0:
		return fmt.Sprintf("Darc: %x", id.Darc.ID)
	case 1:
		return fmt.Sprintf("Ed25519: %s", id.Ed25519.Point.String())
	case 2:
		return fmt.Sprintf("X509EC: %x", id.X509EC.Public)
	default:
		return fmt.Sprintf("No identity")
	}
}

// Verify returns nil if the signature is correct, or an error if something
// went wrong.
func (id *Identity) Verify(msg, sig []byte) error {
	switch id.Type() {
	case 0:
		return errors.New("cannot verify a darc-signature")
	case 1:
		return id.Ed25519.Verify(msg, sig)
	case 2:
		return id.X509EC.Verify(msg, sig)
	default:
		return errors.New("unknown identity")
	}
}

// NewIdentityDarc creates a new darc identity struct given a darcid
func NewIdentityDarc(id ID) *Identity {
	return &Identity{
		Darc: &IdentityDarc{
			ID: id,
		},
	}
}

// Equal returns true if both IdentityDarcs point to the same data.
func (idd *IdentityDarc) Equal(idd2 *IdentityDarc) bool {
	return bytes.Compare(idd.ID, idd2.ID) == 0
}

// NewIdentityEd25519 creates a new Ed25519 identity struct given a point
func NewIdentityEd25519(point kyber.Point) *Identity {
	return &Identity{
		Ed25519: &IdentityEd25519{
			Point: point,
		},
	}
}

// Equal returns true if both IdentityEd25519 point to the same data.
func (ide *IdentityEd25519) Equal(ide2 *IdentityEd25519) bool {
	return ide.Point.Equal(ide2.Point)
}

// Verify returns nil if the signature is correct, or an error if something
// fails.
func (ide *IdentityEd25519) Verify(msg, sig []byte) error {
	return schnorr.Verify(cothority.Suite, ide.Point, msg, sig)
}

// NewIdentityX509EC creates a new X509EC identity struct given a point
func NewIdentityX509EC(public []byte) *Identity {
	return &Identity{
		X509EC: &IdentityX509EC{
			Public: public,
		},
	}
}

// Equal returns true if both IdentityX509EC point to the same data.
func (idkc *IdentityX509EC) Equal(idkc2 *IdentityX509EC) bool {
	return bytes.Compare(idkc.Public, idkc2.Public) == 0
}

type sigRS struct {
	R *big.Int
	S *big.Int
}

// Verify returns nil if the signature is correct, or an error if something
// fails.
func (idkc *IdentityX509EC) Verify(msg, s []byte) error {
	public, err := x509.ParsePKIXPublicKey(idkc.Public)
	if err != nil {
		return err
	}
	hash := sha512.Sum384(msg)
	sig := &sigRS{}
	_, err = asn1.Unmarshal(s, sig)
	if err != nil {
		return err
	}
	if ecdsa.Verify(public.(*ecdsa.PublicKey), hash[:], sig.R, sig.S) {
		return nil
	}
	return errors.New("Wrong signature")
}

// NewSignerEd25519 initializes a new SignerEd25519 given a public and private keys.
// If any of the given values is nil or both are nil, then a new key pair is generated.
// It returns a signer.
func NewSignerEd25519(point kyber.Point, secret kyber.Scalar) *Signer {
	if point == nil || secret == nil {
		kp := key.NewKeyPair(cothority.Suite)
		point, secret = kp.Public, kp.Private
	}
	return &Signer{Ed25519: &SignerEd25519{
		Point:  point,
		Secret: secret,
	}}
}

// Sign creates a schnorr signautre on the message
func (eds *SignerEd25519) Sign(msg []byte) ([]byte, error) {
	return schnorr.Sign(cothority.Suite, eds.Secret, msg)
}

// NewSignerX509EC creates a new SignerX509EC - mostly for tests
func NewSignerX509EC() *Signer {
	return nil
}

// Sign creates a RSA signature on the message
func (kcs *SignerX509EC) Sign(msg []byte) ([]byte, error) {
	return nil, errors.New("not yet implemented")
}
