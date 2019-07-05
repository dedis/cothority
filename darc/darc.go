// Package darc implements Distributed Access Right Controls.
//
// In most of our projects we need some kind of access control to protect
// resources. Instead of having a simple password or public key for
// authentication, we want to have access control that can be: evolved with a
// threshold number of keys be delegated. So instead of having a fixed list of
// identities that are allowed to access a resource, the goal is to have an
// evolving description of who is allowed or not to access a certain resource.
//
// The primary type is a Darc, which contains a set of rules that determine
// what type of permission are granted for any identity. A Darc can be updated
// by performing an evolution.  That is, the identities that have the "evolve"
// permission in the old Darc can create a signature that signs off the new
// Darc. Evolutions can be performed any number of times, which creates a chain
// of Darcs, also known as a path. A path can be verified by starting at the
// oldest Darc (also known as the base Darc), walking down the path and
// verifying the signature at every step.
//
// As mentioned before, it is possible to perform delegation. For example,
// instead of giving the "evolve" permission to (public key) identities, we can
// give it to other Darcs. For example, suppose the newest Darc in some path,
// let's called it darc_A, has the "evolve" permission set to true for another
// darc: darc_B. Then darc_B is allowed to evolve the path.
//
// Of course, we do not want to have static rules that allow only one signer.
// Our Darc implementation supports an expression language where the user can
// use logical operators to specify the rule.  For example, the expression
// "darc:a & ed25519:b | ed25519:c" means that "darc:a" and at least one of
// "ed25519:b" and "ed25519:c" must sign. For more information please see the
// expression package.
package darc

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/sign/eddsa"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/kyber/v3/util/encoding"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/protobuf"
)

const evolve = "_evolve"
const sign = "_sign"

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

// InitRules initialise a set of rules with the default actions "_evolve" and
// "_sign".  Owners are joined with logical-AND under "_evolve" and signers are
// joined with logical-Or under "_sign". If other expressions are needed,
// please set the rules manually. You must use the default action names for
// offline-verification. You should use InitRulesWith to initialise the rules
// with custom action names for evolve and sign.
func InitRules(owners []Identity, signers []Identity) Rules {
	return InitRulesWith(owners, signers, evolve)
}

// InitRulesWith initialise a set of rules with a custom evolve action name.
// Owners are joined with logical-AND under evolveAction and signers are joined
// with logical-Or under "_sign". If other expressions are needed, please set
// the rules manually.
func InitRulesWith(owners, signers []Identity, evolveAction Action) Rules {
	rs := NewRules()

	ownerIDs := make([]string, len(owners))
	for i, o := range owners {
		ownerIDs[i] = o.String()
	}
	if err := rs.AddRule(evolveAction, expression.InitAndExpr(ownerIDs...)); err != nil {
		panic("add rule should never fail on an empty rule list: " + err.Error())
	}

	signerIDs := make([]string, len(signers))
	for i, s := range signers {
		signerIDs[i] = s.String()
	}
	if err := rs.AddRule(sign, expression.InitOrExpr(signerIDs...)); err != nil {
		panic("add rule should never fail on an empty rule list: " + err.Error())
	}
	return rs
}

// NewDarc initialises a darc-structure given its owners and users. Note that
// the BaseID is empty if the Version is 0, it must be computed using
// GetBaseID.
func NewDarc(rules Rules, desc []byte) *Darc {
	zeroSha := sha256.Sum256([]byte{})
	return &Darc{
		Version:     0,
		Description: desc,
		Signatures:  []Signature{},
		Rules:       rules,
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
	}
	dCopy.VerificationDarcs = make([]*Darc, len(d.VerificationDarcs))
	for i := range d.VerificationDarcs {
		dCopy.VerificationDarcs[i] = d.VerificationDarcs[i]
	}
	dCopy.Rules = d.Rules.Copy()
	return dCopy
}

// Equal returns true if both darcs point to the same data.
func (d *Darc) Equal(d2 *Darc) bool {
	return d.GetID().Equal(d2.GetID())
}

// ToProto returns a protobuf representation of the Darc-structure. We copy a
// darc first to keep only invariant fields which exclude the delegation
// signature.
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

// NewFromProtobuf interprets a protobuf-representation of the darc and
// returns it.
func NewFromProtobuf(protoDarc []byte) (*Darc, error) {
	d := &Darc{}
	if err := protobuf.Decode(protoDarc, d); err != nil {
		return nil, err
	}
	return d, nil
}

// GetID returns the Darc ID, which is a digest of the values in the Darc. The
// digest does not include the signature.
func (d Darc) GetID() ID {
	h := sha256.New()
	verBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(verBytes, d.Version)
	h.Write(verBytes)
	h.Write(d.Description)
	h.Write(d.BaseID)
	h.Write(d.PrevID)
	for _, rule := range d.Rules.List {
		h.Write([]byte(rule.Action))
		h.Write(rule.Expr)
	}
	return h.Sum(nil)
}

// GetIdentityString returns the string representation of the ID.
func (d Darc) GetIdentityString() string {
	return NewIdentityDarc(d.GetBaseID()).String()
}

// GetBaseID returns the base ID or the ID of this darc if its the first darc.
func (d Darc) GetBaseID() ID {
	if d.Version == 0 {
		return d.GetID()
	}
	return d.BaseID
}

// NewRules creates an empty Rules.
func NewRules() Rules {
	return Rules{[]Rule{}}
}

// AddRule adds a new action expression-pair, the action must not exist.
func (r *Rules) AddRule(a Action, expr expression.Expr) error {
	if r.exists(a) != -1 {
		return errors.New("action already exists")
	}
	r.List = append(r.List, Rule{a, expr})
	return nil
}

// UpdateRule updates an existing action-expression pair, it cannot be the
// evolve or sign action.
func (r *Rules) UpdateRule(a Action, expr expression.Expr) error {
	if isDefault(a) {
		return fmt.Errorf("cannot update action %s", a)
	}
	return r.updateRule(a, expr)
}

// DeleteRules deletes an action, it cannot delete the evolve or sign action.
func (r *Rules) DeleteRules(a Action) error {
	if isDefault(a) {
		return fmt.Errorf("cannot delete action %s", a)
	}
	i := r.exists(a)
	if i == -1 {
		return fmt.Errorf("DeleteRules: action '%v' does not exist", a)
	}
	r.List = append(r.List[:i], r.List[i+1:]...)
	return nil
}

// UpdateEvolution will update the "_evolve" action, which allows identities
// that satisfies the expression to evolve the Darc. Take extreme care when
// using this function.
func (r *Rules) UpdateEvolution(expr expression.Expr) error {
	return r.updateRule(evolve, expr)
}

// UpdateSign will update the "_sign" action, which allows identities that
// satisfies the expression to sign on behalf of another darc.
func (r *Rules) UpdateSign(expr expression.Expr) error {
	return r.updateRule(sign, expr)
}

func (r *Rules) updateRule(a Action, expr expression.Expr) error {
	i := r.exists(a)
	if i == -1 {
		return fmt.Errorf("updateRule: action '%v' does not exist", a)
	}
	r.List[i] = Rule{a, expr}
	return nil
}

// Contains checks if the action a is in the rules.
func (r Rules) Contains(a Action) bool {
	return r.exists(a) != -1
}

// Count the number of rules.
func (r Rules) Count() int {
	return len(r.List)
}

// GetEvolutionExpr returns the expression that describes the evolution action
// under the default name "_evolve".
func (r Rules) GetEvolutionExpr() expression.Expr {
	return r.Get(evolve)
}

// GetSignExpr returns the expression that describes the sign action under the
// default name "_sign".
func (r Rules) GetSignExpr() expression.Expr {
	return r.Get(sign)
}

// Get gets the expression for action a, it returns nil if the action does not
// exist.
func (r Rules) Get(a Action) expression.Expr {
	for _, rule := range r.List {
		if rule.Action == a {
			return rule.Expr
		}
	}
	return nil
}

// Copy copies the rules.
func (r Rules) Copy() Rules {
	rCopy := NewRules()
	rCopy.List = make([]Rule, r.Count())
	copy(rCopy.List, r.List)
	return rCopy
}

// IsSubset checks whether the receiver is a subset of the given actions set.
func (r Rules) IsSubset(superset Rules) bool {
	supersetMap := make(map[Action]bool)
	for _, x := range superset.List {
		supersetMap[x.Action] = true
	}
	for _, x := range r.List {
		if _, ok := supersetMap[x.Action]; !ok {
			return false
		}
	}
	return true
}

func (r Rules) exists(a Action) int {
	for i, rule := range r.List {
		if rule.Action == a {
			return i
		}
	}
	return -1
}

func isDefault(action Action) bool {
	if action == evolve || action == sign {
		return true
	}
	return false
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
	return nil
}

// MakeEvolveRequest creates a request and signs it such that it can be sent to
// the darc service (for example) to execute the evolution. This function
// assumes that the receiver has all the correct attributes to form a valid
// evolution. It returns a request, and the actual serialisation of the darc.
// We do not put the actual Msg in the request because requests should be kept
// small and the actual payload should be managed by the user of darcs. For
// example the payload could be in a ByzCoin transaction.
func (d *Darc) MakeEvolveRequest(prevSigners ...Signer) (*Request, []byte, error) {
	if d == nil {
		return nil, nil, errors.New("darc is nil")
	}
	if len(prevSigners) == 0 {
		return nil, nil, errors.New("no signers")
	}
	// Create the inner request, this is the message that the signers will
	// sign.
	signerIDs := make([]Identity, len(prevSigners))
	for i, s := range prevSigners {
		signerIDs[i] = s.Identity()
	}
	req := Request{
		BaseID:     d.GetBaseID(),
		Action:     evolve,
		Msg:        d.GetID(),
		Identities: signerIDs,
	}
	// Have every signer sign the digest of the Request.
	digest := req.Hash()
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
	req.Signatures = tmpSigs
	return &req, darcBuf, nil
}

// Verify will check that the darc is correct, an error is returned if
// something is wrong. This is used for offline verification where
// Darc.VerificationDarcs has all the required darcs for doing the
// verification. The function will verify every darc up to the genesis darc
// (version 0) if the fullVerification flag is set.
func (d *Darc) Verify(fullVerification bool) error {
	return d.VerifyWithCB(DarcsToGetDarcs(d.VerificationDarcs), fullVerification)
}

// VerifyWithCB will check that the darc is correct, an error is returned if
// something is wrong. The caller should supply the callback GetDarc because if
// one of the IDs in the expression is a Darc ID, then this function needs a
// way to retrieve the correct Darc according to that ID. This function will
// ignore darcs in Darc.VerificationDarcs, please use Darc.Verify if you wish
// to use it. Further, it verifies every darc up to the genesis darc (version
// 0) if the fullVerification flag is set.
func (d *Darc) VerifyWithCB(getDarc GetDarc, fullVerification bool) error {
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
	if fullVerification {
		return verifyEvolutionRecursive(d, prev, getDarc)
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
	if !d.Rules.Contains(r.Action) {
		return fmt.Errorf("VerifyWithCB: action '%v' does not exist", r.Action)
	}
	digest := r.Hash()
	for i, id := range r.Identities {
		if err := id.Verify(digest, r.Signatures[i]); err != nil {
			return err
		}
	}
	validIDs := r.GetIdentityStrings()
	err := EvalExpr(d.Rules.Get(r.Action), getDarc, validIDs...)
	if err != nil {
		return err
	}
	return nil
}

// String returns a human-readable string representation of the darc.
func (d Darc) String() string {
	res := new(strings.Builder)
	res.WriteString("- Darc:\n")
	fmt.Fprintf(res, "-- Description: %-v\n", strconv.Quote(string(d.Description)))
	fmt.Fprintf(res, "-- BaseID: darc:%x\n", d.GetBaseID())
	fmt.Fprintf(res, "-- PrevID: darc:%x\n", d.PrevID)
	fmt.Fprintf(res, "-- Version: %d\n", d.Version)
	res.WriteString("-- Rules:")
	for _, v := range d.Rules.List {
		fmt.Fprintf(res, "\n--- %s - \"%s\"", v.Action, v.Expr)
	}
	res.WriteString("\n-- Signatures:")
	for i, sig := range d.Signatures {
		fmt.Fprintf(res, "\n--- %d - id: %s, sig: %x", i, sig.Signer.String(), sig.Signature)
	}
	return res.String()
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

// SanityCheck performs a sanity check on the receiver against the previous
// darc. It does not check the expression or signature.
func (d Darc) SanityCheck(prev *Darc) error {
	// check base ID
	if d.BaseID == nil {
		return errors.New("nil base ID")
	}
	if !d.GetBaseID().Equal(prev.GetBaseID()) {
		return errors.New("base IDs are not equal")
	}
	// check version
	if d.Version != prev.Version+1 {
		return fmt.Errorf("incorrect version, new version should be %d but it is %d",
			prev.Version+1, d.Version)
	}
	// check prevID
	if !d.PrevID.Equal(prev.GetID()) {
		return errors.New("prev ID is wrong")
	}
	return nil
}

// verifyOneEvolution verifies that one evolution is performed correctly. That
// is, there exists a signature in the newDarc that is signed by one of the
// identities with the evolve permission in the oldDarc. The message that
// prevDarc signs is the digest of a Darc.Request.
func verifyOneEvolution(newDarc, prevDarc *Darc, getDarc func(string, bool) *Darc) error {
	if err := newDarc.SanityCheck(prevDarc); err != nil {
		return err
	}

	// check that signers have the permission
	if err := EvalExprWithSigs(
		prevDarc.Rules.GetEvolutionExpr(),
		getDarc,
		newDarc.Signatures...); err != nil {
		return err
	}

	// convert the darc into a request
	signerIDs := make([]Identity, len(newDarc.Signatures))
	for i, sig := range newDarc.Signatures {
		signerIDs[i] = sig.Signer
	}
	req := Request{
		BaseID:     newDarc.GetBaseID(),
		Action:     evolve,
		Msg:        newDarc.GetID(),
		Identities: signerIDs,
	}

	// perform the verification
	digest := req.Hash()
	for _, sig := range newDarc.Signatures {
		if err := sig.Signer.Verify(digest, sig.Signature); err != nil {
			return err
		}
	}
	return nil
}

// verifyEvolutionRecursive verifies that evolutions, from the genesis darc
// (darc of version 0), are performed correctly, recursively.
func verifyEvolutionRecursive(newDarc, prevDarc *Darc, getDarc func(string, bool) *Darc) error {
	if err := verifyOneEvolution(newDarc, prevDarc, getDarc); err != nil {
		return err
	}
	// recursively verify the previous darc
	return prevDarc.VerifyWithCB(getDarc, true)
}

// EvalExprWithSigs is a simple wrapper around EvalExpr that extracts Signer
// from Signature.
func EvalExprWithSigs(expr expression.Expr, getDarc GetDarc, sigs ...Signature) error {
	signers := make([]string, len(sigs))
	for i, sig := range sigs {
		signers[i] = sig.Signer.String()
	}
	if err := EvalExpr(expr, getDarc, signers...); err != nil {
		return err
	}
	return nil
}

// EvalExpr checks whether the expression evaluates to true given a list of
// identities.
func EvalExpr(expr expression.Expr, getDarc GetDarc, ids ...string) error {
	return EvalExprDarc(expr, getDarc, false, ids...)
}

// evalExprDarc takes an extra visited parameter to track the visited nodes and
// avoid infinite recursion.
func evalExprDarc(visited map[string]bool, expr expression.Expr, getDarc GetDarc, acceptDarc bool, ids ...string) error {
	var issue error
	Y := expression.InitParser(func(s string) bool {
		found := false
		for _, id := range ids {
			if id == s {
				found = true
			}
		}
		if strings.HasPrefix(s, "darc") {
			if acceptDarc && found {
				return true
			}
			// prevent cycles by checking the visited map
			if _, ok := visited[s]; ok {
				issue = errors.New("cycle detected")
				return false
			}
			// we make a copy so that diamond delegation will work,
			// seeTestDarc_DelegationDiamond
			newVisited := make(map[string]bool)
			for k, v := range visited {
				newVisited[k] = v
			}
			newVisited[s] = true
			// getDarc is responsible for returning the latest Darc
			d := getDarc(s, true)
			if d == nil {
				issue = fmt.Errorf("unable to get the darc %s", s)
				return false
			}
			// Evaluate the "sign" action only in the latest darc
			// because it may have revoked some rules in earlier
			// darcs. We do this recursively because there may be
			// further delegations.
			if !d.Rules.Contains(sign) {
				issue = errors.New(sign + " rule does not exist")
				return false
			}
			signExpr := d.Rules.GetSignExpr()
			// Recursively evaluate the sign expression until we
			// find the final signer.
			if err := evalExprDarc(newVisited, signExpr, getDarc, acceptDarc, ids...); err != nil {
				issue = err
				return false
			}
			return true
		}
		if !found {
			issue = errors.New("expression evaluated to false")
		}
		return found
	})
	res, err := expression.Evaluate(Y, expr)
	if err != nil {
		return err
	}
	if res != true {
		if issue == nil {
			return errors.New("issue is nil - file a bug if you see this error")
		}
		return issue
	}
	return nil
}

// EvalExprDarc checks whether the expression evaluates to true given a list of
// identities. It takes 'acceptDarc', and, if it is true, doesn't recurse into
// darcs that fit one of the ids.
func EvalExprDarc(expr expression.Expr, getDarc GetDarc, acceptDarc bool, ids ...string) error {
	return evalExprDarc(make(map[string]bool), expr, getDarc, acceptDarc, ids...)
}

// Type returns an integer representing the type of key held in the signer. It
// is compatible with Identity.Type. For an empty signer, -1 is returned.
func (s Signer) Type() int {
	switch {
	case s.Ed25519 != nil:
		return 1
	case s.X509EC != nil:
		return 2
	case s.Proxy != nil:
		return 3
	default:
		return -1
	}
}

// Identity returns an identity struct with the pre initialised fields for the
// appropriate signer.
func (s Signer) Identity() Identity {
	switch s.Type() {
	case 1:
		return NewIdentityEd25519(s.Ed25519.Point)
	case 2:
		return NewIdentityX509EC(s.X509EC.Point)
	case 3:
		return NewIdentityProxy(s.Proxy)
	default:
		return Identity{}
	}
}

// Sign returns a signature in bytes for a given messages by the signer.
func (s Signer) Sign(msg []byte) ([]byte, error) {
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
	case 3:
		return s.Proxy.Sign(msg)
	default:
		return nil, errors.New("unknown signer type")
	}
}

// GetPrivate returns the private key, if one exists.
func (s Signer) GetPrivate() (kyber.Scalar, error) {
	switch s.Type() {
	case 1:
		return s.Ed25519.Secret, nil
	case 0, 2, 3:
		return nil, errors.New("signer lacks a private key")
	default:
		return nil, errors.New("signer is of unknown type")
	}
}

// Equal first checks the type of the two identities, and if they match, it
// returns true if their data is the same.
func (id Identity) Equal(id2 *Identity) bool {
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
	case 3:
		return id.Proxy.Equal(id2.Proxy)
	}
	return false
}

// Type returns an int indicating what type of identity this is. If all
// identities are nil, it returns -1.
func (id Identity) Type() int {
	switch {
	case id.Darc != nil:
		return 0
	case id.Ed25519 != nil:
		return 1
	case id.X509EC != nil:
		return 2
	case id.Proxy != nil:
		return 3
	}
	return -1
}

// PrimaryIdentity returns true if the identity is a primary identity, which is
// an identity that is associated with a concrete public/private key.
func (id Identity) PrimaryIdentity() bool {
	switch {
	case id.Darc != nil:
		return false
	case id.Ed25519 != nil:
		return true
	case id.X509EC != nil:
		return true
	case id.Proxy != nil:
		return true
	}
	return false
}

// TypeString returns the string of the type of the identity.
func (id Identity) TypeString() string {
	switch id.Type() {
	case 0:
		return "darc"
	case 1:
		return "ed25519"
	case 2:
		return "x509ec"
	case 3:
		return "proxy"
	default:
		return "No identity"
	}
}

// String returns the string representation of the identity.
func (id Identity) String() string {
	switch id.Type() {
	case 0:
		return fmt.Sprintf("%s:%x", id.TypeString(), id.Darc.ID)
	case 1:
		return fmt.Sprintf("%s:%s", id.TypeString(), id.Ed25519.Point.String())
	case 2:
		return fmt.Sprintf("%s:%x", id.TypeString(), id.X509EC.Public)
	case 3:
		return fmt.Sprintf("%s:%v:%v", id.TypeString(), id.Proxy.Public, id.Proxy.Data)
	default:
		return "No identity"
	}
}

// Verify returns nil if the signature is correct, or an error if something
// went wrong.
func (id Identity) Verify(msg, sig []byte) error {
	switch id.Type() {
	case 0:
		return errors.New("cannot verify a darc-signature")
	case 1:
		return id.Ed25519.Verify(msg, sig)
	case 2:
		return id.X509EC.Verify(msg, sig)
	case 3:
		return id.Proxy.Verify(msg, sig)
	default:
		return errors.New("unknown identity")
	}
}

// GetPublicBytes gets the public identifier in bytes. The returned value is
// not a serialisation of the identity.
func (id Identity) GetPublicBytes() []byte {
	switch id.Type() {
	case 0:
		return id.Darc.ID
	case 1:
		buf, err := id.Ed25519.Point.MarshalBinary()
		if err != nil {
			return nil
		}
		return buf
	case 2:
		return id.X509EC.Public
	case 3:
		buf, err := id.Proxy.Public.MarshalBinary()
		if err != nil {
			return nil
		}
		return buf
	default:
		return nil
	}
}

// NewIdentityDarc creates a new darc identity struct given a darc ID.
func NewIdentityDarc(id ID) Identity {
	return Identity{
		Darc: &IdentityDarc{
			ID: id,
		},
	}
}

// Equal returns true if both IdentityDarcs point to the same data.
func (idd IdentityDarc) Equal(idd2 *IdentityDarc) bool {
	return bytes.Equal(idd.ID, idd2.ID)
}

// NewIdentityEd25519 creates a new Ed25519 identity struct given a point.
func NewIdentityEd25519(point kyber.Point) Identity {
	return Identity{
		Ed25519: &IdentityEd25519{
			Point: point,
		},
	}
}

// Equal returns true if both IdentityEd25519 point to the same data.
func (ide IdentityEd25519) Equal(ide2 *IdentityEd25519) bool {
	return ide.Point.Equal(ide2.Point)
}

// Verify returns nil if the signature is correct, or an error if something
// fails.
func (ide IdentityEd25519) Verify(msg, sig []byte) error {
	return schnorr.Verify(cothority.Suite, ide.Point, msg, sig)
}

// NewIdentityX509EC creates a new X509EC identity struct given a point.
func NewIdentityX509EC(public []byte) Identity {
	return Identity{
		X509EC: &IdentityX509EC{
			Public: public,
		},
	}
}

// NewIdentityProxy creates a new OpenID Connect identity struct.
func NewIdentityProxy(s *SignerProxy) Identity {
	return Identity{
		Proxy: &IdentityProxy{
			Data:   s.Data,
			Public: s.Public,
		},
	}
}

// Equal returns true if both IdentityX509EC point to the same data.
func (idkc IdentityX509EC) Equal(idkc2 *IdentityX509EC) bool {
	return bytes.Compare(idkc.Public, idkc2.Public) == 0
}

// Equal returns true if both IdentityProxy are the same.
func (idp IdentityProxy) Equal(i2 *IdentityProxy) bool {
	return idp.Data == i2.Data && idp.Public.Equal(i2.Public)
}

type sigRS struct {
	R *big.Int
	S *big.Int
}

// Verify returns nil if the signature is correct, or an error if something
// fails.
func (idkc IdentityX509EC) Verify(msg, s []byte) error {
	public, err := x509.ParsePKIXPublicKey(idkc.Public)
	if err != nil {
		return err
	}
	digest := sha512.Sum384(msg)
	sig := &sigRS{}
	_, err = asn1.Unmarshal(s, sig)
	if err != nil {
		return err
	}
	if ecdsa.Verify(public.(*ecdsa.PublicKey), digest[:], sig.R, sig.S) {
		return nil
	}
	return errors.New("Wrong signature")
}

// Verify returns nil if the signature is correct, or an error if something
// fails.
func (idp IdentityProxy) Verify(msg, s []byte) error {
	// Make the message we'll be checking: H(len(data)|data|msg)
	h := sha256.New()
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(len(idp.Data)))
	h.Write(b)
	h.Write([]byte(idp.Data))
	h.Write(msg)
	msg2 := h.Sum(nil)

	return eddsa.Verify(idp.Public, msg2, s)
}

// ParseIdentity returns an Identity structure that matches
// the given string.
func ParseIdentity(in string) (Identity, error) {
	fields := strings.SplitN(in, ":", 2)
	if len(fields) != 2 {
		return Identity{}, errors.New("missing identity type")
	}
	switch fields[0] {
	case "darc":
		return parseIDDarc(fields[1])
	case "ed25519":
		return parseIDEd25519(fields[1])
	case "x509ec":
		return parseIDX509ec(fields[1])
	case "proxy":
		return parseIDProxy(fields[1])
	default:
		return Identity{}, fmt.Errorf("unknown identity type %v", fields[0])
	}
}

func parseIDEd25519(in string) (Identity, error) {
	p, err := encoding.StringHexToPoint(suites.MustFind("ed25519"), in)
	if err != nil {
		return Identity{}, err
	}
	return Identity{Ed25519: &IdentityEd25519{Point: p}}, nil
}

func parseIDX509ec(in string) (Identity, error) {
	id := make([]byte, hex.DecodedLen(len(in)))
	_, err := hex.Decode(id, []byte(in))
	if err != nil {
		return Identity{}, err
	}
	return Identity{X509EC: &IdentityX509EC{Public: id}}, nil
}

func parseIDDarc(in string) (Identity, error) {
	id := make([]byte, hex.DecodedLen(len(in)))
	_, err := hex.Decode(id, []byte(in))
	if err != nil {
		return Identity{}, err
	}
	return Identity{Darc: &IdentityDarc{ID: id}}, nil
}

func parseIDProxy(in string) (Identity, error) {
	fields := strings.SplitN(in, ":", 2)
	if len(fields) != 2 {
		return Identity{}, errors.New("expected proxy format of proxy:public-key:data")
	}

	p, err := encoding.StringHexToPoint(suites.MustFind("ed25519"), fields[0])
	if err != nil {
		return Identity{}, err
	}

	return Identity{Proxy: &IdentityProxy{
		Public: p,
		Data:   fields[1],
	}}, nil
}

// NewSignerEd25519 initializes a new SignerEd25519 signer given public and
// private keys. If either of the given keys is nil, then a new key pair is
// generated.
func NewSignerEd25519(public kyber.Point, private kyber.Scalar) Signer {
	if public == nil || private == nil {
		kp := key.NewKeyPair(cothority.Suite)
		public, private = kp.Public, kp.Private
	}
	return Signer{Ed25519: &SignerEd25519{
		Point:  public,
		Secret: private,
	}}
}

// Sign creates a schnorr signautre on the message.
func (eds SignerEd25519) Sign(msg []byte) ([]byte, error) {
	return schnorr.Sign(cothority.Suite, eds.Secret, msg)
}

// Hash computes the digest of the request, the identities and signatures are
// not included.
func (r Request) Hash() []byte {
	h := sha256.New()
	h.Write(r.BaseID)
	h.Write([]byte(r.Action))
	h.Write(r.Msg)
	for _, i := range r.Identities {
		h.Write([]byte(i.String()))
	}
	return h.Sum(nil)
}

// GetIdentityStrings returns a slice of identity strings, this is useful for
// creating a parser.
func (r Request) GetIdentityStrings() []string {
	res := make([]string, len(r.Identities))
	for i, id := range r.Identities {
		res[i] = id.String()
	}
	return res
}

// MsgToDarc attempts to return a darc given the matching darcBuf. This
// function should *not* be used as a way to verify the darc, it only checks
// that darcBuf can be decoded and matches with the Msg part of the request.
func (r Request) MsgToDarc(darcBuf []byte) (*Darc, error) {
	d, err := NewFromProtobuf(darcBuf)
	if err != nil {
		return nil, err
	}

	if !d.GetID().Equal(r.Msg) {
		return nil, errors.New("darc IDs are not equal")
	}

	if len(r.Signatures) != len(r.Identities) {
		return nil, errors.New("signature and identitity length mismatch")
	}
	darcSigs := make([]Signature, len(r.Signatures))
	for i := range r.Signatures {
		darcSigs[i] = Signature{
			Signature: r.Signatures[i],
			Signer:    r.Identities[i],
		}
	}
	d.Signatures = darcSigs

	return d, nil
}

// String returns a formatted string of the rule
func (r Rule) String() string {
	return fmt.Sprintf("%s:%s", r.Action, r.Expr)
}

// NewRequest initialises a request, the caller must provide all the fields of
// the request. There is no guarantee that this request is valid, please see
// InitAndSignRequest is a valid request needs to be created.
func NewRequest(baseID ID, action Action, msg []byte, ids []Identity, sigs [][]byte) Request {
	req := Request{
		BaseID:     baseID,
		Action:     action,
		Msg:        msg,
		Identities: ids,
	}
	req.Signatures = sigs
	return req
}

// InitAndSignRequest creates a new request which can be verified by a Darc.
func InitAndSignRequest(baseID ID, action Action, msg []byte, signers ...Signer) (*Request, error) {
	if len(signers) == 0 {
		return nil, errors.New("there are no signers")
	}
	signerIDs := make([]Identity, len(signers))
	for i, s := range signers {
		signerIDs[i] = s.Identity()
	}
	req := Request{
		BaseID:     baseID,
		Action:     action,
		Msg:        msg,
		Identities: signerIDs,
	}
	digest := req.Hash()
	sigs := make([][]byte, len(signers))
	for i, s := range signers {
		var err error
		sigs[i], err = s.Sign(digest)
		if err != nil {
			return nil, err
		}
	}
	req.Signatures = sigs
	return &req, nil
}

// NewSignerX509EC creates a new SignerX509EC - mostly for tests.
func NewSignerX509EC() Signer {
	return Signer{}
}

// Sign creates a RSA signature on the message.
func (kcs SignerX509EC) Sign(msg []byte) ([]byte, error) {
	return nil, errors.New("not yet implemented")
}

// NewSignerProxy creates a new SignerProxy. When Sign is called, the getSignature
// callback will be called, so that the caller can use the appropriate mechanism
// to retrieve and/or construct the signature.
func NewSignerProxy(data string, pub kyber.Point, getSignature func([]byte) ([]byte, error)) Signer {
	return Signer{
		Proxy: &SignerProxy{
			Data:         data,
			Public:       pub,
			getSignature: getSignature,
		},
	}
}

// Sign delegates generation of the signature to the caller via
// the callback set in the constructor, so that they can contact
// the proxy or proxies and do any necessary interpolation to arrive
// at the final signature.
func (s SignerProxy) Sign(msg []byte) ([]byte, error) {
	sig, err := s.getSignature(msg)
	return sig, err
}

func copyBytes(a []byte) []byte {
	b := make([]byte, len(a))
	copy(b, a)
	return b
}
