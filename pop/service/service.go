package service

/*
Service for a Proof-of-Personhood party

Proof-of-personhood parties provide a number of "attendees" with an "anonymous
token" that enables them to "authenticate" to a service as being part of the
party.

These parties are held by a number of "organisers" who set up a party by
defining place, time and purpose of that party and by publishing a
"party configuration" that is signed by the organisers "conodes".
At the party, they "register" all attendees' public keys.
Once the party is over, they create a "party transcript" that is signed by all
organisers' conodes.

The attendees create their "pop token" by joining their private key to the
party transcript. They can now use that token to sign a "message" in a "context"
from a service and send the resulting "signature" and "tag" back to the service.

On the service's side, it can use the party transcript to verify that the
signature has been created using a private key present in the party transcript.
The tag will be unique to that attendee/context pair, but another service using
another context will not be able to link two tags to the same or different
attendee.
*/

import (
	"errors"
	"fmt"

	"math/big"

	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// Name is the name to refer to the Template service from another
// package.
const Name = "PoPServer"
const cfgName = "pop.bin"

func init() {
	onet.RegisterNewService(Name, newService)
	network.RegisterMessage(&saveData{})
}

// Service represents data needed for one pop-party.
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor
	path string
	data *saveData
	// channel to return the configreply
	ccChannel chan *CheckConfigReply
}

type saveData struct {
	// Pin holds the randomly chosen pin
	Pin string
	// Public key of linked pop
	Public abstract.Point
	// The final statement
	Final *FinalStatement
}

// PinRequest prints out a pin if none is given, else it verifies it has the
// correct pin, and if so, it stores the public key as reference.
func (s *Service) PinRequest(req *PinRequest) (network.Message, onet.ClientError) {
	if req.Pin == "" {
		s.data.Pin = fmt.Sprintf("%06d", random.Int(big.NewInt(1000000), random.Stream))
		log.Info("PIN:", s.data.Pin)
		return nil, onet.NewClientErrorCode(ErrorWrongPIN, "Read PIN in server-log")
	}
	if req.Pin != s.data.Pin {
		return nil, onet.NewClientErrorCode(ErrorWrongPIN, "Wrong PIN")
	}
	s.data.Public = req.Public
	s.save()
	log.Lvl1("Successfully registered PIN/Public", s.data.Pin, req.Public)
	return nil, nil
}

// saves the actual identity
func (s *Service) save() {
	log.Lvl3("Saving service")
	err := s.Save("storage", s.data)
	if err != nil {
		log.Error("Couldn't save data:", err)
	}
}

// Tries to load the configuration and updates if a configuration
// is found, else it returns an error.
func (s *Service) tryLoad() error {
	if !s.DataAvailable("storage") {
		return nil
	}
	msg, err := s.Load("storage")
	if err != nil {
		return err
	}
	var ok bool
	s.data, ok = msg.(*saveData)
	if !ok {
		return errors.New("Data of wrong type")
	}
	return nil
}

// newService registers the request-methods.
func newService(c *onet.Context) onet.Service {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		data:             &saveData{},
		ccChannel:        make(chan *CheckConfigReply, 1),
	}
	if err := s.RegisterHandlers(s.PinRequest); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
	}
	return s
}
