/*
Package darc in most of our projects we need some kind of access control to
protect resources. Instead of having a simple password
or public key for authentication, we want to have access control that can be:
evolved with a threshold number of keys
be delegated
So instead of having a fixed list of identities that are allowed to access a
resource, the goal is to have an evolving
description of who is allowed or not to access a certain resource.
*/
package darc

import (
	"errors"
	//"fmt"

	"bytes"
	"crypto/sha256"
	"encoding/json"
	"strings"

	"github.com/dedis/protobuf"
	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/kyber.v2"
	"gopkg.in/dedis/kyber.v2/sign/schnorr"
	"gopkg.in/dedis/kyber.v2/util/key"
	"gopkg.in/dedis/kyber.v2/util/random"
	"gopkg.in/dedis/onet.v2/log"
)

// NewDarc initialises a darc-structure
func NewDarc(rules *[]*Rule) *Darc {
	var ru []*Rule
	ru = append(ru, *rules...)
	id := CreateID()
	return &Darc{
		ID:      id,
		Version: 0,
		Rules:   &ru,
	}
}

func NewRule(action string, subjects *[]*Subject, expression string) *Rule {
	var subs []*Subject
	subs = append(subs, *subjects...)
	return &Rule{
		Action:     action,
		Subjects:   &subs,
		Expression: expression,
	}
}

// NewSubject creates an identity with either a link to another darc
// or an Ed25519 identity (containing a point). You're only allowed
// to give either a darc or a point, but not both.
func NewSubject(darc *SubjectDarc, pk *SubjectPK) (*Subject, error) {
	if darc != nil && pk != nil {
		return nil, errors.New("cannot have both darc and ed25519 point in one subject")
	}
	if darc == nil && pk == nil {
		return nil, errors.New("give one of darc or point")
	}
	return &Subject{
		Darc: darc,
		PK:   pk,
	}, nil
}

// NewSubjectDarc creates a new darc identity struct given a darcid
func NewSubjectDarc(id ID) *SubjectDarc {
	return &SubjectDarc{
		ID: id,
	}
}

// NewSubjectPK creates a new ed25519 identity given a public-key point
func NewSubjectPK(point kyber.Point) *SubjectPK {
	return &SubjectPK{
		Point: point,
	}
}

// Copy all the fields of a Darc
func (d *Darc) Copy() *Darc {
	dCopy := &Darc{
		ID:      d.ID,
		Version: d.Version,
	}
	if d.Rules != nil {
		var rules []*Rule
		for _, r := range *d.Rules {
			x := *r
			rules = append(rules, &x)
		}
		//rules := append([]*Rule{}, *d.Rules...)
		dCopy.Rules = &rules
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

func CreateID() ID {
	idsize := 32
	id := make([]byte, idsize)
	random.Bytes(id, random.New())
	return id
}

func (d *Darc) GetID() ID {
	return d.ID
}

// GetHash returns the hash of the protobuf-representation of the Darc as its Id.
func (d *Darc) GetHash() ID {
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

//To-do: Add admin rule first?
//Use as 'Darc.AddRule(rule)'
func (d *Darc) AddRule(rule *Rule) ([]*Rule, error) {
	//Check if Admin Rule is trying to be duplicated
	if strings.Compare(rule.Action, "Admin") == 0 {
		for _, r := range *d.Rules {
			if strings.Compare(r.Action, "Admin") == 0 {
				return nil, errors.New("Cannot have two Admin rules")
			}
		}
	}
	var rules []*Rule
	if d.Rules != nil {
		rules = *d.Rules
	}
	rules = append(rules, rule)
	d.Rules = &rules
	return *d.Rules, nil
}

//Use as 'Darc.RemoveRule(rule)'
func (d *Darc) RemoveRule(ruleind int) ([]*Rule, error) {
	var ruleIndex = -1
	var rules []*Rule
	if d.Rules == nil {
		return nil, errors.New("Empty rule list")
	}
	rules = *d.Rules
	for i, r := range *d.Rules {
		if i == ruleind {
			if strings.Compare(r.Action, "Admin") == 0 {
				return nil, errors.New("Cannot remove Admin rule")
			}
			ruleIndex = i
		}
	}
	if ruleIndex == -1 {
		return nil, errors.New("Rule is not present in the Darc")
	}
	//Removing rule
	rules = append(rules[:ruleIndex], rules[ruleIndex+1:]...)
	d.Rules = &rules
	return *d.Rules, nil
}

func (d *Darc) RuleUpdateAction(ruleind int, action string) ([]*Rule, error) {
	rules := *d.Rules
	if d.Rules == nil {
		return nil, errors.New("Empty rule list")
	}
	if (ruleind > len(rules)-1) || (ruleind < 0) {
		return nil, errors.New("Invalid RuleID in request")
	}
	rules[ruleind].Action = action
	d.Rules = &rules
	return *d.Rules, nil
}

func (d *Darc) RuleAddSubject(ruleind int, subject *Subject) ([]*Rule, error) {
	rules := *d.Rules
	if d.Rules == nil {
		return nil, errors.New("Empty rule list")
	}
	if (ruleind > len(rules)-1) || (ruleind < 0) {
		return nil, errors.New("Invalid RuleID in request")
	}
	var subjects = *rules[ruleind].Subjects
	subjects = append(subjects, subject)
	rules[ruleind].Subjects = &subjects
	d.Rules = &rules
	return *d.Rules, nil
}

func (d *Darc) RuleRemoveSubject(ruleind int, subject *Subject) ([]*Rule, error) {
	rules := *d.Rules
	if d.Rules == nil {
		return nil, errors.New("Empty rule list")
	}
	if (ruleind > len(rules)-1) || (ruleind < 0) {
		return nil, errors.New("Invalid RuleID in request")
	}
	var subjectIndex = -1
	var subjects = *rules[ruleind].Subjects
	if subjects == nil {
		return nil, errors.New("Empty subjects list")
	}
	for i, s := range subjects {
		if s == subject {
			subjectIndex = i
		}
	}
	if subjectIndex == -1 {
		return nil, errors.New("Subject ID not found")
	}
	subjects = append(subjects[:subjectIndex], subjects[subjectIndex+1:]...)
	rules[ruleind].Subjects = &subjects
	d.Rules = &rules
	return *d.Rules, nil
}

func (d *Darc) RuleUpdateExpression(ruleind int, expression string) ([]*Rule, error) {
	rules := *d.Rules
	if d.Rules == nil {
		return nil, errors.New("Empty rule list")
	}
	if (ruleind > len(rules)-1) || (ruleind < 0) {
		return nil, errors.New("Invalid RuleID in request")
	}
	rules[ruleind].Expression = expression
	d.Rules = &rules
	return *d.Rules, nil
}

func EvaluateExpression(expression string, indexMap map[int]*Signature) (bool, error) {
	in := []byte(expression)
	var raw interface{}
	json.Unmarshal(in, &raw)
	result, err := ProcessJson(raw, indexMap, false)
	return result, err
}

//For now, we just take a JSON expression and convert it into
// a string showing evaluation. This will be replaced by actual
//evaluation when we introduce signatures
func ProcessJson(raw interface{}, indexMap map[int]*Signature, evaluation bool) (bool, error) {
	m := raw.(map[string]interface{})
	for k, v := range m {
		switch vv := v.(type) {
		case []interface{}:
			for i, u := range vv {
				switch x := u.(type) {
				case map[string]interface{}:
					if i == 0 {
						y, err := ProcessJson(x, indexMap, evaluation)
						if err != nil {
							return false, err
						}
						if i == len(vv)-1 {
							z, err := operation(k, y, false)
							if err != nil {
								return false, err
							}
							evaluation = z
						} else {
							evaluation = y
						}
					} else {
						y, err := ProcessJson(x, indexMap, evaluation)
						if err != nil {
							return false, err
						}
						evaluation, err = operation(k, evaluation, y)
						if err != nil {
							return false, err
						}
					}
				case float64:
					if i == 0 {
						if i == len(vv)-1 {
							z, err := operation(k, checkMap(indexMap, int(x)), false)
							if err != nil {
								return false, err
							}
							evaluation = z
						} else {
							evaluation = checkMap(indexMap, int(x))
						}
					} else {
						y, err := operation(k, evaluation, checkMap(indexMap, int(x)))
						if err != nil {
							return false, err
						}
						evaluation = y
					}
				}
			}
		default:
			return false, errors.New("Unknown type in expression interface.")
		}
	}
	return evaluation, nil
}

func checkMap(indexMap map[int]*Signature, index int) bool {
	_, exists := indexMap[index]
	return exists
}

func operation(operand string, op1 bool, op2 bool) (bool, error) {
	if operand == "and" {
		return op1 && op2, nil
	} else if operand == "or" {
		return op1 || op2, nil
	} else if operand == "not" {
		return !op1, nil
	} else {
		return false, errors.New("Unknown operand")
	}
}

// NewDarc initialises a darc-structure
func NewRequest(darcid ID, ruleid int, message []byte) *Request {
	return &Request{
		DarcID:  darcid,
		RuleID:  ruleid,
		Message: message,
	}
}

func (r *Request) CopyReq() *Request {
	rCopy := &Request{
		DarcID:  r.DarcID,
		RuleID:  r.RuleID,
		Message: r.Message,
	}
	return rCopy
}

func (s *Signer) Sign(req *Request) (*Signature, error) {
	rc := req.CopyReq()
	b, err := protobuf.Encode(rc)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, errors.New("nothing to sign, message is empty")
	}
	if s.Ed25519 != nil {
		key, err := s.GetPrivate()
		if err != nil {
			return nil, errors.New("could not retrieve a private key")
		}
		pub, err := s.GetPublic()
		if err != nil {
			return nil, errors.New("could not retrieve a public key")
		}
		signature, _ := schnorr.Sign(cothority.Suite, key, b)
		signer := &SubjectPK{Point: pub}
		return &Signature{Signature: signature, Signer: *signer}, nil
	}
	return nil, errors.New("signer is of unknown type")
}

func (s *Signer) SignWithPath(req *Request, path []int) (*SignaturePath, error) {
	rc := req.CopyReq()
	b, err := protobuf.Encode(rc)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, errors.New("nothing to sign, message is empty")
	}
	if s.Ed25519 != nil {
		key, err := s.GetPrivate()
		if err != nil {
			return nil, errors.New("could not retrieve a private key")
		}
		pub, err := s.GetPublic()
		if err != nil {
			return nil, errors.New("could not retrieve a public key")
		}
		signature, _ := schnorr.Sign(cothority.Suite, key, b)
		signer := &SubjectPK{Point: pub}
		return &SignaturePath{Signature: signature, Signer: *signer, Path: path}, nil
	}
	return nil, errors.New("signer is of unknown type")
}

func (s *Signer) SignWithPathCheck(req *Request, darcs map[string]*Darc) (*Signature, [][]int, error) {
	rc := req.CopyReq()
	b, err := protobuf.Encode(rc)
	if err != nil {
		return nil, nil, err
	}
	if b == nil {
		return nil, nil, errors.New("nothing to sign, message is empty")
	}
	if s.Ed25519 != nil {
		key, err := s.GetPrivate()
		if err != nil {
			return nil, nil, errors.New("could not retrieve a private key")
		}
		pub, err := s.GetPublic()
		if err != nil {
			return nil, nil, errors.New("could not retrieve a public key")
		}
		var pathindex []int
		var paths [][]int
		sub := &SubjectPK{Point: pub}
		targetDarc, err := FindDarc(darcs, req.DarcID)
		if err != nil {
			return nil, nil, err
		}
		rules := *targetDarc.Rules
		targetRule, err := FindRule(rules, req.RuleID)
		if err != nil {
			return nil, nil, err
		}
		subs := *targetRule.Subjects
		paths, err = FindAllPaths(subs, &Subject{PK: sub}, darcs, pathindex, paths)
		if err != nil {
			return nil, nil, errors.New("There does not seem to be a valid path from target darc to signer")
		}
		if len(paths) > 1 {
			return nil, paths, errors.New("Multiple paths present. Sign with specific path.")
		}
		signature, _ := schnorr.Sign(cothority.Suite, key, b)
		signer := &SubjectPK{Point: pub}
		return &Signature{Signature: signature, Signer: *signer}, nil, nil
	}
	return nil, nil, errors.New("signer is of unknown type")
}

func (s *Signer) GetPublic() (kyber.Point, error) {
	if s.Ed25519 != nil {
		if s.Ed25519.Point != nil {
			return s.Ed25519.Point, nil
		}
		return nil, errors.New("signer lacks a public key")
	}
	return nil, errors.New("signer is of unknown type")
}

func (s *Signer) GetPrivate() (kyber.Scalar, error) {
	if s.Ed25519 != nil {
		if s.Ed25519.Secret != nil {
			return s.Ed25519.Secret, nil
		}
		return nil, errors.New("signer lacks a private key")
	}
	return nil, errors.New("signer is of unknown type")
}

//Verifying multisig requests
func VerifyMultiSig(req *Request, sigs []*Signature, darcs map[string]*Darc) error {
	var indexMap = make(map[int]*Signature)
	targetDarc, err := FindDarc(darcs, req.DarcID)
	if err != nil {
		return err
	}
	rules := *targetDarc.Rules
	targetRule, err := FindRule(rules, req.RuleID)
	if err != nil {
		return err
	}
	subs := *targetRule.Subjects
	//Check if signatures are correct
	for _, sig := range sigs {
		if sig == nil || len(sig.Signature) == 0 {
			return errors.New("No signature present")
		}
		rc := req.CopyReq()
		b, err := protobuf.Encode(rc)
		if err != nil {
			return err
		}
		if b == nil {
			return errors.New("Nothing to verify, message is empty.")
		}
		pub := sig.Signer.Point
		err = schnorr.Verify(cothority.Suite, pub, b, sig.Signature)
		if err != nil {
			return err
		}
		var pathIndex []int
		signer := sig.Signer
		pa, err := FindSubject(subs, &Subject{PK: &signer}, darcs, pathIndex)
		if err != nil {
			//fmt.Println(err)
			return errors.New("Signature not in path.")
		}
		indexMap[pa[0]] = sig
	}
	//Evaluate expression
	expression := targetRule.Expression
	evaluation, err := EvaluateExpression(expression, indexMap)
	if err != nil {
		return err
	}
	if evaluation {
		return nil
	} else {
		return errors.New("Signatures don't satisfy expression.")
	}
}

func Verify(req *Request, sig *Signature, darcs map[string]*Darc) error {
	//Check if signature is correct
	if sig == nil || len(sig.Signature) == 0 {
		return errors.New("No signature available")
	}
	rc := req.CopyReq()
	b, err := protobuf.Encode(rc)
	if err != nil {
		return err
	}
	if b == nil {
		return errors.New("nothing to verify, message is empty")
	}
	pub := sig.Signer.Point
	err = schnorr.Verify(cothority.Suite, pub, b, sig.Signature)
	if err != nil {
		return err
	}
	//Check if path from rule to signer is correct
	err = GetPath(darcs, req, sig)
	if err != nil {
		return err
	}
	return err
}

func VerifySigWithPath(req *Request, sig *SignaturePath, darcs map[string]*Darc) error {
	//Check if signature is correct
	if sig == nil || len(sig.Signature) == 0 {
		return errors.New("No signature available")
	}
	rc := req.CopyReq()
	b, err := protobuf.Encode(rc)
	if err != nil {
		return err
	}
	if b == nil {
		return errors.New("nothing to verify, message is empty")
	}
	pub := sig.Signer.Point
	err = schnorr.Verify(cothority.Suite, pub, b, sig.Signature)
	if err != nil {
		return err
	}
	//Check if path from rule to signer is correct
	err = VerifyPath(darcs, req, sig)
	if err != nil {
		return err
	}
	return err
}

func VerifyPath(darcs map[string]*Darc, req *Request, sig *SignaturePath) error {
	path := sig.Path
	subject := &Subject{PK: &sig.Signer}
	current_darc, err := FindDarc(darcs, req.DarcID)
	if err != nil {
		return err
	}
	for i := 0; i < len(path); i++ {
		if i == len(path)-1 {
			ruleind, err := FindUserRuleIndex(*current_darc.Rules)
			if err != nil {
				return err
			}
			subs := *(*current_darc.Rules)[ruleind].Subjects
			target_sub := subs[path[i]]
			if CompareSubjects(target_sub, subject) {
				return nil
			}
		}
		//fmt.Println(i, path[i])
		ruleind, err := FindUserRuleIndex(*current_darc.Rules)
		if err != nil {
			return err
		}
		subs := *(*current_darc.Rules)[ruleind].Subjects
		if path[i] > len(subs) {
			return errors.New("Path is incorrect.")
		}
		target := subs[path[i]]
		target_darc := *target.Darc
		target_id := target_darc.ID
		current_darc, err = FindDarc(darcs, target_id)
		if err != nil {
			return err
		}
	}
	return errors.New("Path is incorrect.")
}

func GetPath(darcs map[string]*Darc, req *Request, sig *Signature) error {
	//Find Darc from request DarcID
	targetDarc, err := FindDarc(darcs, req.DarcID)
	if err != nil {
		return err
	}
	rules := *targetDarc.Rules
	targetRule, err := FindRule(rules, req.RuleID)
	if err != nil {
		return err
	}
	signer := sig.Signer
	subs := *targetRule.Subjects
	var pathIndex []int
	_, err = FindSubject(subs, &Subject{PK: &signer}, darcs, pathIndex)
	//fmt.Println(pa)
	return err
}

func CompareSubjects(s1 *Subject, s2 *Subject) bool {
	if s1.PK != nil && s2.PK != nil {
		if s1.PK.Point == s2.PK.Point {
			return true
		}
	} else if s1.Darc != nil && s2.Darc != nil {
		return s1.Darc.ID.Equal(s2.Darc.ID)
	}
	return false
}

func FindSubject(subjects []*Subject, requester *Subject, darcs map[string]*Darc, pathIndex []int) ([]int, error) {
	//fmt.Println(pathIndex)
	for i, s := range subjects {
		if CompareSubjects(s, requester) == true {
			pathIndex = append(pathIndex, i)
			return pathIndex, nil
		} else if s.Darc != nil {
			targetDarc, err := FindDarc(darcs, s.Darc.ID)
			if err != nil {
				return nil, err
			}
			ruleind, err := FindUserRuleIndex(*targetDarc.Rules)
			if err != nil {
				return nil, errors.New("User rule ID not found")
			}
			subs := *(*targetDarc.Rules)[ruleind].Subjects
			pathIndex = append(pathIndex, i)
			pa, err := FindSubject(subs, requester, darcs, pathIndex)
			if err != nil {
				pathIndex = pathIndex[:len(pathIndex)-1]
			} else {
				return pa, nil
			}
		}
	}
	return nil, errors.New("Subject not found")
}

func FindDarc(darcs map[string]*Darc, darcid ID) (*Darc, error) {
	d, ok := darcs[string(darcid)]
	if ok == false {
		return nil, errors.New("Invalid DarcID")
	}
	return d, nil
}

func FindRule(rules []*Rule, ruleid int) (*Rule, error) {
	if (ruleid > len(rules)-1) || (ruleid < 0) {
		return nil, errors.New("Invalid RuleID in request")
	}
	return rules[ruleid], nil
}

func FindUserRuleIndex(rules []*Rule) (int, error) {
	ruleind := -1
	for i, rule := range rules {
		if rule.Action == "User" {
			ruleind = i
		}
	}
	if ruleind == -1 {
		return ruleind, errors.New("User rule ID not found")
	}
	return ruleind, nil
}

func FindAllPaths(subjects []*Subject, requester *Subject, darcs map[string]*Darc, pathIndex []int, allpaths [][]int) ([][]int, error) {
	l := len(allpaths)
	for i, s := range subjects {
		if CompareSubjects(s, requester) == true {
			pathIndex = append(pathIndex, i)
			allpaths = append(allpaths, pathIndex)
		} else if s.Darc != nil {
			targetDarc, err := FindDarc(darcs, s.Darc.ID)
			if err != nil {
				return nil, err
			}
			ruleind := -1
			for i, rule := range *targetDarc.Rules {
				if rule.Action == "User" {
					ruleind = i
				}
			}
			if ruleind == -1 {
				return nil, errors.New("User rule ID not found")
			}
			subs := *(*targetDarc.Rules)[ruleind].Subjects
			pathIndex = append(pathIndex, i)
			pa, err := FindAllPaths(subs, requester, darcs, pathIndex, allpaths)
			if err != nil {
				pathIndex = pathIndex[:len(pathIndex)-1]
			} else {
				allpaths = pa
				pathIndex = pathIndex[:0]
			}
		}
	}
	if len(allpaths) > l {
		return allpaths, nil
	} else {
		return nil, errors.New("No path found")
	}
}

// NewEd25519Signer initializes a new Ed25519Signer given a public and private
// keys.
// If any of the given values is nil or both are nil, then a new key pair is
// generated.
func NewEd25519Signer(point kyber.Point, secret kyber.Scalar) *Ed25519Signer {
	if point == nil || secret == nil {
		kp := key.NewKeyPair(cothority.Suite)
		point, secret = kp.Public, kp.Private
	}
	return &Ed25519Signer{
		Point:  point,
		Secret: secret,
	}
}

// IncrementVersion updates the version number of the Darc
func (d *Darc) IncrementVersion() {
	d.Version++
}

// IsNull returns true if this DarcID is not initialised.
func (di ID) IsNull() bool {
	return di == nil
}

// Equal compares with another DarcID.
func (di ID) Equal(other ID) bool {
	return bytes.Equal([]byte(di), []byte(other))
}
