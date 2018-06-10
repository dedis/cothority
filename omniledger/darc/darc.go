// Package darc implements Distributed Access Right Controls.
//
// In most of our projects we need some kind of access control to protect
// resources. Instead of having a simple password or public key for
// authentication, we want to have access control that can be: evolved
// with a threshold number of keys be delegated. So instead of having a
// fixed list of identities that are allowed to access a resource, the
// goal is to have an evolving description of who is allowed or not to
// access a certain resource.
//
// The primary type is a Darc, which contains a set of rules that
// determine what type of permission are granted for any identity. A Darc
// can be updated by performing an evolution.  That is, the identities
// that have the "evolve" permission in the old Darc can create a
// signature that signs off the new Darc. Evolutions can be performed any
// number of times, which creates a chain of Darcs, also known as a
// path. A path can be verified by starting at the oldest Darc (also
// known as the base Darc), walking down the path and verifying the
// signature at every step.
//
// As mentioned before, it is possible to perform delegation. For
// example, instead of giving the "evolve" permission to (public key)
// identities, we can give it to other Darcs. For example, suppose the
// newest Darc in some path, let's called it darc_A, has the "evolve"
// permission set to true for another darc: darc_B. Then darc_B is
// allowed to evolve the path.
//
// Of course, we do not want to have static rules that allow only one
// signer. Our Darc implementation supports an expression language where
// the user can use logical operators to specify the rule.  For exmple,
// the expression "darc:a & ed25519:b | ed25519:c" means that "darc:a"
// and at least one of "ed25519:b" and "ed25519:c" must sign. For more
// information please see the expression package.
package darc

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/dedis/cothority/omniledger/darc/expression"
	"github.com/dedis/protobuf"
)

// GetDarc is a callback function that we expect the user of this library to
// supply in some of our methods. The user is free to choose how he/she wants
// to store the darc. Hence, during verification, we need a way to retrieve an
// older darc so check that the a request or an evolution is correctly signed,
// which is done using this callback. The first argument is the ID of the darc
// (output of IdentityDarc.String()), the second argument is to specify whether
// this callback should return the latest darc or the exact darc. For instance,
// if latest is true, then given a darc base-ID (a darc of version 0), the
// callback should return the latest one with that base-ID. If latest is false,
// then the callback should return an exact match. The callback should return
// nil if no match is found.
type GetDarc func(s string, latest bool) *Darc

// NewDarc initialises a darc-structure given its owners and users. Note that
// the BaseID is empty if the Version is 0, it must be computed using
// GetBaseID. It sets the initial set of rules with owners and signers. Signers
// are joined with logical-Or, owners (identities that are allowed to evolve
// the darc) are joined with logical-AND. If other expressions are needed,
// please set the rules manually.
func NewDarc(owners []*Identity, signers []*Identity, evolveName, signName Action, desc []byte) *Darc {
	zeroSha := sha256.Sum256([]byte{})
	return &Darc{
		Version:     0,
		Description: desc,
		EvolveName:  evolveName,
		SignName:    signName,
		Signatures:  []*Signature{},
		Rules:       initRules(owners, signers, evolveName, signName),
		PrevID:      zeroSha[:],
	}
}

// Copy all the fields of a Darc except the signature
func (d *Darc) Copy() *Darc {
	dCopy := &Darc{
		Version:     d.Version,
		Description: copyBytes(d.Description),
		BaseID:      copyBytes(d.BaseID),
		PrevID:      copyBytes(d.PrevID),
		EvolveName:  d.EvolveName,
		SignName:    d.SignName,
	}
	dCopy.VerificationDarcs = make([]*Darc, len(d.VerificationDarcs))
	for i := range d.VerificationDarcs {
		dCopy.VerificationDarcs[i] = d.VerificationDarcs[i]
	}
	dCopy.Rules = d.Rules.copyRules()
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
	if d == nil {
		return nil, errors.New("darc is nil")
	}
	dc := d.Copy()
	b, err := protobuf.Encode(dc)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// NewDarcFromProto interprets a protobuf-representation of the darc and
// returns a created Darc.
func NewDarcFromProto(protoDarc []byte) (*Darc, error) {
	d := &Darc{}
	if err := protobuf.Decode(protoDarc, d); err != nil {
		return nil, err
	}
	return d, nil
}

// GetID returns the Darc ID, which is a digest of the values in the Darc.
// The digest does not include the signature or the path, only the version,
// description, base ID and the rules .
func (d Darc) GetID() ID {
	h := sha256.New()
	verBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(verBytes, d.Version)
	h.Write(verBytes)
	h.Write(d.Description)
	h.Write(d.BaseID)
	h.Write(d.PrevID)
	h.Write([]byte(d.EvolveName))
	h.Write([]byte(d.SignName))

	actions := make([]string, len(d.Rules))
	var i int
	for k := range d.Rules {
		actions[i] = string(k)
		i++
	}
	sort.Strings(actions)
	for _, a := range actions {
		h.Write([]byte(a))
		h.Write(d.Rules[Action(a)])
	}
	return h.Sum(nil)
}

// GetIdentityString returns the string representation of the ID.
func (d Darc) GetIdentityString() string {
	return NewIdentityDarc(d.GetID()).String()
}

// GetBaseID returns the base ID or the ID of this darc if its the
// first darc.
func (d Darc) GetBaseID() ID {
	if d.Version == 0 {
		return d.GetID()
	}
	return d.BaseID
}

// AddRule adds a new action expression-pair, the action must not exist.
func (d *Darc) AddRule(a Action, expr expression.Expr) error {
	if _, ok := d.Rules[a]; ok {
		return errors.New("action already exists")
	}
	d.Rules[a] = expr
	return nil
}

// UpdateRule updates an existing action-expression pair, it cannot be the
// evolve or sign action.
func (d *Darc) UpdateRule(a Action, expr expression.Expr) error {
	if d.isDefault(a) {
		return fmt.Errorf("cannot update action %s", a)
	}
	r, err := d.Rules.updateRule(a, expr)
	if err != nil {
		return err
	}
	d.Rules = r
	return nil
}

// DeleteRules deletes an action, it cannot delete the evolve or sign action.
func (d *Darc) DeleteRules(a Action) error {
	if d.isDefault(a) {
		return fmt.Errorf("cannot delete action %s", a)
	}
	if _, ok := d.Rules[a]; !ok {
		return fmt.Errorf("DeleteRules: action '%v' does not exist", a)
	}
	delete(d.Rules, a)
	return nil
}

// ContainsAction checks if the action a is in the rules.
func (d Darc) ContainsAction(a Action) bool {
	_, ok := d.Rules[a]
	return ok
}

// GetEvolutionExpr returns the expression that describes the evolution action.
func (d Darc) GetEvolutionExpr() expression.Expr {
	return d.Rules[d.EvolveName]
}

// GetSignExpr returns the expression that describes the sign action.
func (d Darc) GetSignExpr() expression.Expr {
	return d.Rules[d.SignName]
}

// UpdateEvolution will update the evolve action, which allows identities
// that satisfies the expression to evolve the Darc. Take extreme care when
// using this function.
func (d *Darc) UpdateEvolution(expr expression.Expr) error {
	r, err := d.Rules.updateRule(d.EvolveName, expr)
	if err != nil {
		return err
	}
	d.Rules = r
	return nil
}

// UpdateSign will update the "_sign" action, which allows identities that
// satisfies the expression to sign on behalf of another darc.
func (d *Darc) UpdateSign(expr expression.Expr) error {
	r, err := d.Rules.updateRule(d.SignName, expr)
	if err != nil {
		return err
	}
	d.Rules = r
	return nil
}

func (d Darc) isDefault(action Action) bool {
	if action == d.EvolveName || action == d.SignName {
		return true
	}
	return false
}

func (r Rules) updateRule(a Action, expr expression.Expr) (Rules, error) {
	if _, ok := r[a]; !ok {
		return r, fmt.Errorf("updateRule: action '%v' does not exist", a)
	}
	r[a] = expr
	return r, nil
}

func (r Rules) copyRules() Rules {
	newRules := make(Rules)
	for k, v := range r {
		newRules[k] = v
	}
	return newRules
}

// EvolveFrom sets the fields of d such that it is a valid evolution from the
// darc given by prev. For the evolution to be accepted, it must be signed
// using Darc.MakeEvolveRequest.
func (d *Darc) EvolveFrom(prev *Darc) error {
	if prev == nil {
		return errors.New("prev darc cannot be nil")
	}
	d.Version = prev.Version + 1
	d.BaseID = prev.GetBaseID()
	d.PrevID = prev.GetID()
	d.EvolveName = prev.EvolveName
	d.SignName = prev.SignName
	return nil
}

// MakeEvolveRequest creates a request and signs it such that it can be sent to
// the darc service (for example) to execute the evolution. This function
// assumes that the receiver has all the correct attributes to form a valid
// evolution. It returns a request, and the actual serialisation of the darc.
// We do not put the actual Msg in the request because requests should be kept
// small and the actual payload should be managed by the user of darcs. For
// example the payload could be in an OmniLedger transaction.
func (d *Darc) MakeEvolveRequest(prevSigners ...*Signer) (*Request, []byte, error) {
	if d == nil {
		return nil, nil, errors.New("darc is nil")
	}
	if len(prevSigners) == 0 {
		return nil, nil, errors.New("no signers")
	}
	// Create the inner request, this is the message that the signers will
	// sign.
	signerIDs := make([]*Identity, len(prevSigners))
	for i, s := range prevSigners {
		signerIDs[i] = s.Identity()
	}
	inner := innerRequest{
		BaseID:     d.GetBaseID(),
		Action:     d.EvolveName,
		Msg:        d.GetID(),
		Identities: signerIDs,
	}
	// Have every signer sign the digest of the innerRequest.
	digest := inner.Hash()
	tmpSigs := make([][]byte, len(prevSigners))
	for i, s := range prevSigners {
		var err error
		tmpSigs[i], err = s.Sign(digest)
		if err != nil {
			return nil, nil, err
		}
	}
	darcBuf, err := d.ToProto()
	if err != nil {
		return nil, nil, err
	}
	return &Request{
		inner,
		tmpSigs,
	}, darcBuf, nil
}

// Verify will check that the darc is correct, an error is returned if
// something is wrong. This is used for offline verification where
// Darc.VerificationDarcs has all the required darcs for doing the
// verification.
func (d *Darc) Verify() error {
	return d.VerifyWithCB(DarcsToGetDarcs(d.VerificationDarcs))
}

// VerifyWithCB will check that the darc is correct, an error is returned if
// something is wrong.  The caller should supply the callback GetDarc because
// if one of the IDs in the expression is a Darc ID, then this function needs a
// way to retrieve the correct Darc according to that ID. This function will
// ignore darcs in Darc.VerificationDarcs, please use Darc.Verify if you wish
// to use it.
func (d *Darc) VerifyWithCB(getDarc GetDarc) error {
	if d == nil {
		return errors.New("darc is nil")
	}
	if d.Version == 0 {
		return nil // nothing to verify on the genesis Darc
	}

	if len(d.Signatures) == 0 {
		return errors.New("no signatures")
	}

	// We try to find an exact match for the darc in Darc.PrevID, so don't
	// ask the callback to return the latest one.
	prev := getDarc(NewIdentityDarc(d.PrevID).String(), false)
	if prev == nil {
		return errors.New("cannot find the previous darc")
	}
	return verifyOneEvolution(d, prev, getDarc)
}

// Verify checks the request with the given darc and returns an error if it
// cannot be accepted. The caller is responsible for providing the latest darc
// in the argument. The darcs in Darc.VerificationDarcs will be used for the
// verification.
func (r *Request) Verify(d *Darc) error {
	return r.VerifyWithCB(d, DarcsToGetDarcs(d.VerificationDarcs))
}

// VerifyWithCB checks the request with the given darc using a callback which
// looks-up missing darcs. The function returns an error if the request cannot
// be accepted. The caller is responsible for providing the latest darc in the
// argument. This function will ignore darcs in Darc.VerificationDarcs, please
// use Darc.Verify if you wish to use it.
func (r *Request) VerifyWithCB(d *Darc, getDarc GetDarc) error {
	if len(r.Signatures) == 0 {
		return errors.New("no signatures - nothing to verify")
	}
	if len(r.Signatures) != len(r.Identities) {
		return fmt.Errorf("signatures and identities have unequal length - %d != %d",
			len(r.Signatures), len(r.Identities))
	}

	if !d.GetBaseID().Equal(r.BaseID) {
		return fmt.Errorf("base id mismatch")
	}
	if !d.ContainsAction(r.Action) {
		return fmt.Errorf("VerifyWithCB: action '%v' does not exist", r.Action)
	}
	digest := r.Hash()
	for i, id := range r.Identities {
		if err := id.Verify(digest, r.Signatures[i]); err != nil {
			return err
		}
	}
	validIDs := r.GetIdentityStrings()
	err := evalExpr(d.Rules[r.Action], getDarc, validIDs...)
	if err != nil {
		return err
	}
	return nil
}

// String returns a human-readable string representation of the darc.
func (d Darc) String() string {
	s := fmt.Sprintf("ID:\t%x\nBase:\t%x\nPrev:\t%x\nVer:\t%d\nRules:", d.GetID(), d.GetBaseID(), d.PrevID, d.Version)
	s += fmt.Sprintf("\nEvolveName:\t%s\nSignName:\t%s", d.EvolveName, d.SignName)
	for k, v := range d.Rules {
		s += fmt.Sprintf("\n\t%s - \"%s\"", k, v)
	}
	for i, sig := range d.Signatures {
		if sig == nil {
			s += fmt.Sprintf("\n\t%d - <nil signature>", i)
		} else {
			s += fmt.Sprintf("\n\t%d - id: %s, sig: %x", i, sig.Signer.String(), sig.Signature)
		}
	}
	return s
}

// IsNull returns true if this DarcID is not initialised.
func (id ID) IsNull() bool {
	return id == nil
}

// Equal compares with another DarcID.
func (id ID) Equal(other ID) bool {
	return bytes.Equal([]byte(id), []byte(other))
}

// DarcsToGetDarcs is a convenience function that convers a slice of darcs into
// the GetDarc callback.
func DarcsToGetDarcs(darcs []*Darc) GetDarc {
	return func(s string, latest bool) *Darc {
		// build a map to store the latest darcs
		m := make(map[string]*Darc)
		for _, inner := range darcs {
			id := NewIdentityDarc(inner.GetBaseID())
			if v, ok := m[id.String()]; ok {
				if v.Version < inner.Version {
					m[id.String()] = inner
				}
			} else {
				m[id.String()] = inner
			}
		}

		// pick the darc depending if we're looking for the latest one
		if latest {
			if v, ok := m[s]; ok {
				return v
			}
		} else {
			for _, inner := range darcs {
				if NewIdentityDarc(inner.GetID()).String() == s {
					return inner
				}
			}
		}
		return nil
	}
}

func initRules(owners []*Identity, signers []*Identity, evolveName, signName Action) Rules {
	rs := make(Rules)

	ownerIDs := make([]string, len(owners))
	for i, o := range owners {
		ownerIDs[i] = o.String()
	}
	rs[evolveName] = expression.InitAndExpr(ownerIDs...)

	signerIDs := make([]string, len(signers))
	for i, s := range signers {
		signerIDs[i] = s.String()
	}
	rs[signName] = expression.InitOrExpr(signerIDs...)
	return rs
}

// verifyOneEvolution verifies that one evolution is performed correctly. That
// is, there exists a signature in the newDarc that is signed by one of the
// identities with the evolve permission in the oldDarc. The message that
// prevDarc signs is the digest of a Darc.Request.
func verifyOneEvolution(newDarc, prevDarc *Darc, getDarc func(string, bool) *Darc) error {
	// check base ID
	if newDarc.BaseID == nil {
		return errors.New("nil base ID")
	}
	if !newDarc.GetBaseID().Equal(prevDarc.GetBaseID()) {
		return errors.New("base IDs are not equal")
	}

	// check version
	if newDarc.Version != prevDarc.Version+1 {
		return fmt.Errorf("incorrect version, new version should be %d but it is %d",
			prevDarc.Version+1, newDarc.Version)
	}

	// check that signers have the permission
	if err := evalExprWithSigs(
		prevDarc.GetEvolutionExpr(),
		getDarc,
		newDarc.Signatures...); err != nil {
		return err
	}

	// convert the darc into a request
	signerIDs := make([]*Identity, len(newDarc.Signatures))
	for i, sig := range newDarc.Signatures {
		signerIDs[i] = &sig.Signer
	}
	inner := innerRequest{
		BaseID:     newDarc.GetBaseID(),
		Action:     newDarc.EvolveName,
		Msg:        newDarc.GetID(),
		Identities: signerIDs,
	}

	// perform the verification
	digest := inner.Hash()
	for _, sig := range newDarc.Signatures {
		if err := sig.Signer.Verify(digest, sig.Signature); err != nil {
			return err
		}
	}

	// recursively verify the previous darc
	return prevDarc.VerifyWithCB(getDarc)
}

// evalExprWithSigs is a simple wrapper around evalExpr that extracts Signer
// from Signature.
func evalExprWithSigs(expr expression.Expr, getDarc GetDarc, sigs ...*Signature) error {
	signers := make([]string, len(sigs))
	for i, sig := range sigs {
		signers[i] = sig.Signer.String()
	}
	if err := evalExpr(expr, getDarc, signers...); err != nil {
		return err
	}
	return nil
}

// evalExpr checks whether the expression evaluates to true
// given a list of identities.
func evalExpr(expr expression.Expr, getDarc GetDarc, ids ...string) error {
	Y := expression.InitParser(func(s string) bool {
		if strings.HasPrefix(s, "darc") {
			// getDarc is responsible for returning the latest Darc
			d := getDarc(s, true)
			if d.VerifyWithCB(getDarc) != nil {
				return false
			}
			// Evaluate the "sign" action only in the latest darc
			// because it may have revoked some rules in earlier
			// darcs. We do this recursively because there may be
			// further delegations.
			if !d.ContainsAction(d.SignName) {
				return false
			}
			// Recursively evaluate the sign expression until we
			// find the final signer with a ed25519 key.
			if err := evalExpr(d.GetSignExpr(), getDarc, ids...); err != nil {
				return false
			}
			return true
		}
		for _, id := range ids {
			if id == s {
				return true
			}
		}
		return false
	})
	res, err := expression.Evaluate(Y, expr)
	if err != nil {
		return fmt.Errorf("evaluation failed on '%s' with error: %v", expr, err)
	}
	if res != true {
		return fmt.Errorf("expression '%s' evaluated to false", expr)
	}
	return nil
}

func copyBytes(a []byte) []byte {
	b := make([]byte, len(a))
	copy(b, a)
	return b
}
