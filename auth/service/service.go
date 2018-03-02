// Package authentication centralizes authentication for services like
// skipchain, cisc, pop and others who need it. This first version simply
// holds a list of all authentication tokens that are allowed to do
// things with this conode. A later version will include policy-darcs to
// correctly authenticate using modern tools.
//
// Usage of Darcs
//
// Distributed Access Rights Control is a data structure for handling access
// rights to given resources. A darc has a list of Owners who are allowed
// to propose a new version of the darc. It also has a list of Users who are
// allowed to sign as authentication for the execution of an action defined
// by the darc.
//
// Both Owners and Users can point to other darcs, so that a general admin
// darc can point to specific user darcs and then each user darc can
// update his access rights without having to contact the admin darc.
//
// Policy definition
//
// When starting up, the authentication service sets up a new Darc with the
// public key of the conode and stores it under the policy "". All policies are
// compared using strings.HasPrefix, and the longest comparison is used to
// verify the authentication, so an empty string is the same as /^.*/ in
// regular expressions.
//
// This darc is the root darc and responsible for all services and all methods.
// Everybody having the private key of the conode is thus allowed to sign for
// any action of any service.
//
// For more fine-grained authentication, the client can ask to store a new
// darc with a 'Data' field set to the name of the service. This allows
// a client to give access to an external user only to a given service.
//
// For even more fine-grained authentication, the 'Data' can hold the name
// of the service separated with a '.' from the method. For example to protect
// the `CreateSkipchain` method of the identity service, the client would have to
// create a darc with a 'Data' of 'Identity.CreateSkipchain'.
//
// How a service should use it
//
// Each service that wants to use authentication can include the following
// structure in its api-messages sent from the client:
//
//  type Auth struct{
//    Signature darc.Signature
//  }
//
// on the service-side, the service-method has to call
//
//  authentication.Verify(s onet.Service, service, method string, auth Auth) error
//
// to verify if the request has been properly authenticated. The service and method
// strings must correspond to the service method that wants to verify the
// authentication. When comparing Policy-darcs against the Auth struct,
// the Policy is compared using strings.HasPrefix.
package authentication

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/dedis/cothority/auth/darc"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

// ServiceName is the global name of that service.
const ServiceName = "Authentication"

var authID onet.ServiceID

func init() {
	var err error
	authID, err = onet.RegisterNewService(ServiceName, newService)
	if err != nil {
		log.Fatal(err)
	}
}

// Service for the auhtentication holds all darcs that define who is allowed
// to do what. It interacts with the users through the api.
type Service struct {
	*onet.ServiceProcessor
	storageMutex sync.Mutex
	storage      *storage
	pin          string
}

var storageID = []byte("auth")

type storage struct {
	Policies map[string]*darc.Darc
}

// Auth can be included in an api-client-message to send an authentication to
// the service.
type Auth struct {
	Signature darc.Signature
}

// Verify checks the signature on the message given by "service.method"
// by going through all darcs and looking if one of the darc-policies matches the
// message and if the signature is correct.
func Verify(s onet.Context, service, method string, auth Auth, msg []byte) error {
	authService := s.Service(ServiceName).(*Service)
	path := fmt.Sprintf("%s.%s", service, method)
	authService.storageMutex.Lock()
	defer authService.storageMutex.Unlock()
	for pol, resp := range authService.storage.Policies {
		if strings.HasPrefix(path, pol) {
			log.Lvlf3("Found policy :%s: for %s", pol, path)
			err := authService.verifyDarcSignature(resp, &auth.Signature, darc.User, msg)
			if err == nil {
				return nil
			}
			log.Lvlf2("couldn't verify path %s for policy :%s: %s", path, pol, err)
		}
	}
	return errors.New("didn't find matching policy")
}

// GetPolicy returns the latest version of the chosen policy with the ID
// given. If the ID == null, the latest version of the
// basic policy is returned.
func (s *Service) GetPolicy(req *GetPolicy) (*GetPolicyReply, error) {
	s.storageMutex.Lock()
	defer s.storageMutex.Unlock()
	p, d := s.findParentPolicy(req.Policy)
	reply := &GetPolicyReply{
		Policy: p,
		Latest: d,
	}
	return reply, nil
}

// UpdatePolicy updates an existing policy. If it's an existing policy, then
// it must be signed off by the previous version of the darc. If it's a new
// policy, then the Signature field must be set to a valid darc.Signature
// created by the responsible darc for that sub-policy.
func (s *Service) UpdatePolicy(req *UpdatePolicy) (*UpdatePolicyReply, error) {
	s.storageMutex.Lock()
	defer s.storageMutex.Unlock()
	next := req.NewDarc
	previous, exists := s.storage.Policies[req.Policy]
	var responsible *darc.Darc
	var signature *darc.Signature
	if next.Version > 0 {
		// Updating an existing policy - verify if the new darc is correct
		if !exists {
			return nil, errors.New("cannot update darc for non existing policy")
		}
		if previous.Version+1 != next.Version {
			return nil, errors.New("need to update monotonically")
		}
		responsible = previous
		signature = next.Signature
	} else {
		// Adding a new policy - verifying that the darc will fit in
		if exists {
			return nil, errors.New("cannot replace existing darc")
		}
		if req.Signature == nil {
			return nil, errors.New("need a signature for new darcs")
		}
		_, responsible = s.findParentPolicy(req.Policy)
		signature = req.Signature
	}

	if err := s.verifyDarcSignature(responsible, signature, darc.Owner, next.GetID()); err != nil {
		return nil, errors.New("signature verification failed: " + err.Error())
	}
	s.storage.Policies[req.Policy] = req.NewDarc
	return &UpdatePolicyReply{}, nil
}

// UpdatePolicyPIN can be used in case the private key is not available,
// but if the user has access to the logs of the server. On the first
// call the PIN == "", and the server will print a 6-digit PIN in the log
// files. When he receives the policy and the correct PIN, the server will
// auto-sign the policy using his private key and add it to the policy-list.
func (s *Service) UpdatePolicyPIN(req *UpdatePolicyPIN) (*UpdatePolicyPINReply, error) {
	s.storageMutex.Lock()
	defer s.storageMutex.Unlock()
	if req.PIN == "" {
		modulo := big.NewInt(1000000)
		s.pin = fmt.Sprintf("%06d", random.Int(modulo, random.New()))
		log.Lvl1("PIN for authentication is:", s.pin)
		return nil, errors.New("please read PIN on server")
	}
	if req.PIN != s.pin {
		return nil, errors.New("wrong pin - please try again")
	}
	s.storage.Policies[req.Policy] = req.NewDarc
	return &UpdatePolicyPINReply{}, nil
}

// findParentPolicy searches for the policy matching best the given policy string.
func (s *Service) findParentPolicy(policy string) (bestPolicy string, policyDarc *darc.Darc) {
	bestPolicy = ""
	policyDarc = s.storage.Policies[""]
	for p, d := range s.storage.Policies {
		if strings.HasPrefix(policy, p) {
			if len(p) > len(bestPolicy) {
				bestPolicy = p
				policyDarc = d
			}
		}
	}
	return bestPolicy, policyDarc
}

// verifyDarcSignature checks if the given signature is validly
func (s *Service) verifyDarcSignature(responsible *darc.Darc, signature *darc.Signature,
	role darc.Role, msg []byte) error {
	path := signature.SignaturePath
	signer := path.Signer

	// Is the signer present in the responsible darc?
	if err := s.verifySignerInDarc(&signer, responsible, role); err != nil {
		return errors.New("didn't find signer in responsible darc")
	}

	// Create correct signature message and verify it against the signer's
	// public key.
	hash, err := path.SigHash(msg)
	if err != nil {
		return errors.New("couldn't create hash: " + err.Error())
	}
	if err := signer.Verify(hash, signature.Signature); err != nil {
		return errors.New("wrong signature: " + err.Error())
	}
	return nil
}

// Verify signer is legit - for the moment this means that he needs
// to be in the previous darc. Later versions of the darcs will be
// able to point to darcs stored in skipchains which can be updated
// by individual users.
func (s *Service) verifySignerInDarc(signer *darc.Identity, responsible *darc.Darc,
	role darc.Role) error {
	if responsible.HasRole(signer, role) {
		return nil
	}
	return errors.New("didn't find signer in darc")
}

// saves all data.
func (s *Service) save() {
	s.storageMutex.Lock()
	defer s.storageMutex.Unlock()
	err := s.Save(storageID, s.storage)
	if err != nil {
		log.Error("Couldn't save data:", err)
	}
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	s.storage = &storage{
		Policies: map[string]*darc.Darc{},
	}
	msg, err := s.Load(storageID)
	if err != nil {
		return err
	}
	if msg == nil {
		s.storeRootPolicy()
		return nil
	}
	var ok bool
	s.storage, ok = msg.(*storage)
	if !ok {
		return errors.New("Data of wrong type")
	}
	if len(s.storage.Policies) == 0 {
		s.storeRootPolicy()
	}
	return nil
}

// Creates a root policy "" with the public key of the conode.
func (s *Service) storeRootPolicy() {
	conode := &[]*darc.Identity{darc.NewIdentityEd25519(s.ServerIdentity().Public)}
	root := &darc.Darc{
		Owners:  conode,
		Users:   conode,
		Version: 0,
	}
	s.storage.Policies = map[string]*darc.Darc{"": root}
}

// newService instantiates an authentication service and registers the
// messages to the websocket-api.
func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	if err := s.RegisterHandlers(s.GetPolicy, s.UpdatePolicy, s.UpdatePolicyPIN); err != nil {
		return nil, errors.New("Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	return s, nil
}
