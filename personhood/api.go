package personhood

// api for personhood - very minimalistic for the moment, as most of the
// calls are made from javascript.

import (
	"fmt"
	"golang.org/x/xerrors"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

// Client is a structure to communicate with the personhood
// service
type Client struct {
	*onet.Client
}

// NewClient instantiates a new personhood.Client
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// WipeParties removes all parties stored in the system.
func (c *Client) WipeParties(r onet.Roster) (errs []error) {
	t := true
	pl := PartyList{
		WipeParties: &t,
	}
	for _, si := range r.List {
		err := c.SendProtobuf(si, &pl, nil)
		if err != nil {
			errs = append(errs, fmt.Errorf("error in node %s: %s", si.Address, err))
		}
	}
	return
}

// WipeRoPaScis removes all stored RoPaScis from the service.
func (c *Client) WipeRoPaScis(r onet.Roster) (errs []error) {
	t := true
	pl := RoPaSciList{
		Wipe: &t,
	}
	for _, si := range r.List {
		err := c.SendProtobuf(si, &pl, nil)
		if err != nil {
			errs = append(errs, fmt.Errorf("error in node %s: %s", si.Address, err))
		}
	}
	return
}

// GetAdminDarcIDs returns the current DarcIDs and the nonce to be used to
// sign a new SetAdminDarcIDs.
func (c *Client) GetAdminDarcIDs(si *network.ServerIdentity) (
	gadReply GetAdminDarcIDsReply, errs []error) {
	gad := &GetAdminDarcIDs{}
	err := c.SendProtobuf(si, gad, &gadReply)
	if err != nil {
		errs = append(errs, fmt.Errorf("error in node %s: %s",
			si.Address, err))
	}
	return
}

// SetAdminDarcIDs sets a new slice of adminDarcIDs.
func (c *Client) SetAdminDarcIDs(si *network.ServerIdentity, adminDarcIDs []darc.ID,
	priv kyber.Scalar) (errs []error) {
	sadid := &SetAdminDarcIDs{
		NewAdminDarcIDs: adminDarcIDs,
	}
	msg := make([]byte, 0, len(adminDarcIDs)*32)
	for _, adid := range adminDarcIDs {
		msg = append(msg, adid...)
	}
	log.Infof("message is: %x", msg)
	var err error
	sadid.Signature, err = schnorr.Sign(cothority.Suite, priv, msg)
	if err != nil {
		errs = []error{err}
	}
	err = c.SendProtobuf(si, sadid, nil)
	if err != nil {
		errs = append(errs, fmt.Errorf("error in node %s: %s",
			si.Address, err))
	}
	return
}

// EmailSetup sends the email setup request to the server.
// The ServerIdentity needs to contain a valid private key corresponding to
// the public key of the node.
func (c *Client) EmailSetup(si *network.ServerIdentity,
	es *EmailSetup) error {
	if err := es.Sign(si.GetPrivate()); err != nil {
		return xerrors.Errorf("couldn't sign request: %v", err)
	}

	err := c.SendProtobuf(si, es, nil)
	if err != nil {
		return xerrors.Errorf("couldn't send request: %v", err)
	}
	return nil
}

// EmailSignup asks for a new email address to be signed up.
// The signup-link is sent to the address given.
func (c *Client) EmailSignup(si *network.ServerIdentity,
	alias, email string) (EmailSignupReply, error) {
	var reply EmailSignupReply
	err := c.SendProtobuf(si, &EmailSignup{
		Alias: alias,
		Email: email,
	}, &reply)
	return reply, err
}

// EmailRecover asks for a recovery of an existing account.
// Recovering an account that doesn't exist yet returns an error.
func (c *Client) EmailRecover(si *network.ServerIdentity,
	email string) (EmailRecoverReply, error) {
	var reply EmailRecoverReply
	err := c.SendProtobuf(si, &EmailRecover{Email: email}, &reply)
	return reply, err
}
