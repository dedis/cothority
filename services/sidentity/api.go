package sidentity

import (
	"bytes"
	"errors"
	"io"

	"fmt"
	"io/ioutil"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/cosi"
)

/*
This is the external API to access the identity-service. It shows the methods
used to create a new identity-skipchain, propose new configurations and how
to vote on these configurations.
*/

func init() {
	for _, s := range []interface{}{
		// Structures
		&Device{},
		&DevicePoint{},
		&Identity{},
		&Config{},
		&Storage{},
		&Service{},
		// API messages
		&CreateIdentity{},
		&CreateIdentityReply{},
		&ConfigUpdate{},
		&ConfigUpdateReply{},
		&GetUpdateChain{},
		&GetUpdateChainReply{},
		&ProposeSend{},
		&ProposeUpdate{},
		&ProposeUpdateReply{},
		&ProposeVote{},
		&Data{},
		&ProposeVoteReply{},
		// Internal messages
		&PropagateIdentity{},
		&UpdateSkipBlock{},
	} {
		network.RegisterPacketType(s)
	}
}

// Identity structure holds the data necessary for a client/device to use the
// identity-service. Each identity-skipchain is tied to a roster that is defined
// in 'Cothority'
type Identity struct {
	// Client is included for easy `Send`-methods.
	*sda.Client
	// IdentityData holds all the data related to this identity
	// It can be stored and loaded from a config file.
	Data
}

// Data contains the data that will be stored / loaded from / to a file
// that enables a client to use the Identity service.
type Data struct {
	// Private key for that device.
	Private abstract.Scalar
	// Public key for that device - will be stored in the identity-skipchain.
	Public abstract.Point
	// ID of the skipchain this device is tied to.
	ID ID
	// ID of the latest known skipblock
	latestID ID
	// Config is the actual, valid configuration of the identity-skipchain.
	Config *Config
	// Proposed is the new configuration that has not been validated by a
	// threshold of devices.
	Proposed *Config
	// DeviceName must be unique in the identity-skipchain.
	DeviceName string
	// Cothority is the roster responsible for the identity-skipchain. It
	// might change in the case of a roster-update.
	Cothority *sda.Roster
}

// NewIdentity starts a new identity that can contain multiple managers with
// different accounts
func NewIdentity(cothority *sda.Roster, threshold int, owner string) *Identity {
	client := sda.NewClient(ServiceName)
	kp := config.NewKeyPair(network.Suite)
	return &Identity{
		Client: client,
		Data: Data{
			Private:    kp.Secret,
			Public:     kp.Public,
			Config:     NewConfig(threshold, kp.Public, owner),
			DeviceName: owner,
			Cothority:  cothority,
		},
	}
}

// NewIdentityFromCothority searches for a given cothority
func NewIdentityFromCothority(el *sda.Roster, id ID) (*Identity, error) {
	iden := &Identity{
		Client: sda.NewClient(ServiceName),
		Data: Data{
			Cothority: el,
			ID:        id,
		},
	}
	err := iden.ConfigUpdate()
	if err != nil {
		return nil, err
	}
	return iden, nil
}

// NewIdentityFromStream reads the configuration of that client from
// any stream
func NewIdentityFromStream(in io.Reader) (*Identity, error) {
	data, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, err
	}
	_, i, err := network.UnmarshalRegistered(data)
	if err != nil {
		return nil, err
	}
	id := i.(*Data)
	identity := &Identity{
		Client: sda.NewClient(ServiceName),
		Data:   *id,
	}
	return identity, nil
}

// SaveToStream stores the configuration of the client to a stream
func (i *Identity) SaveToStream(out io.Writer) error {
	data, err := network.MarshalRegisteredType(&i.Data)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

// GetProposed returns the Propose-field or a copy of the config if
// the Propose-field is nil
func (i *Identity) GetProposed() *Config {
	if i.Proposed != nil {
		return i.Proposed
	}
	return i.Config.Copy()
}

// AttachToIdentity proposes to attach it to an existing Identity
func (i *Identity) AttachToIdentity(ID ID) error {
	i.ID = ID
	i.latestID = ID
	var err error
	err = i.ConfigUpdateNew()
	if err != nil {
		return err
	}
	if _, exists := i.Config.Device[i.DeviceName]; exists {
		return errors.New("Adding with an existing account-name")
	}
	confPropose := i.Config.Copy()
	for _, dev := range confPropose.Device {
		dev.Vote = nil
	}
	confPropose.Device[i.DeviceName] = &Device{Point: i.Public}
	err = i.ProposeSend(confPropose)
	if err != nil {
		return err
	}
	return nil
}

// UpdateIdentityThreshold proposes an update regarding the numbers of votes required
// for any subsequent proposal to be accepted
func (i *Identity) UpdateIdentityThreshold(thr int) error {
	var err error
	err = i.ConfigUpdateNew()
	if err != nil {
		return err
	}
	confPropose := i.Config.Copy()
	for _, dev := range confPropose.Device {
		dev.Vote = nil
	}
	confPropose.Threshold = thr
	err = i.ProposeSend(confPropose)
	if err != nil {
		return err
	}
	return nil
}

// CreateIdentity asks the identityService to create a new Identity
func (i *Identity) CreateIdentity() error {
	msg, err := i.Send(i.Cothority.RandomServerIdentity(), &CreateIdentity{i.Config, i.Cothority})
	if err != nil {
		return err
	}
	air := msg.Msg.(CreateIdentityReply)
	i.ID = ID(air.Data.Hash)
	i.latestID = ID(air.Data.Hash)
	err = i.ConfigUpdate()
	if err != nil {
		return err
	}
	return nil
}

// ProposeConfig proposes a new skipblock with general modifications (add/revoke one or
// more devices and/or change the threshold)
// Devices to be revoked regarding the proposed config should NOT vote upon their revovation
// (or, in the case of voting, a negative vote is the only one accepted)
func (i *Identity) ProposeConfig(add, revoke map[string]abstract.Point, thr int) error {
	var err error
	err = i.ConfigUpdateNew()
	if err != nil {
		return err
	}
	confPropose := i.Config.Copy()
	for _, dev := range confPropose.Device {
		dev.Vote = nil
	}
	confPropose.Threshold = thr
	for name, point := range add {
		confPropose.Device[name] = &Device{Point: point}
	}
	for name, _ := range revoke {
		if _, exists := confPropose.Device[name]; exists {
			delete(confPropose.Device, name)
		}
	}
	err = i.ProposeSend(confPropose)
	if err != nil {
		return err
	}
	return nil
}

// ProposeSend sends the new proposition of this identity
// ProposeVote
func (i *Identity) ProposeSend(il *Config) error {
	_, err := i.Client.Send(i.Cothority.RandomServerIdentity(), &ProposeSend{i.ID, il})
	i.Proposed = il
	return err
}

// ProposeUpdate verifies if there is a new configuration awaiting that
// needs approval from clients
func (i *Identity) ProposeUpdate() error {
	msg, err := i.Send(i.Cothority.RandomServerIdentity(), &ProposeUpdate{
		ID: i.ID,
	})
	if err != nil {
		return err
	}
	cnc := msg.Msg.(ProposeUpdateReply)
	i.Proposed = cnc.Propose
	return nil
}

// ProposeVote calls the 'accept'-vote on the current propose-configuration
func (i *Identity) ProposeVote(accept bool) error {
	fmt.Println("ProposeVote()")
	if i.Proposed == nil {
		return errors.New("No proposed config")
	}
	log.Lvlf3("Voting %t on %s", accept, i.Proposed.Device)
	if !accept {
		return nil
	}
	hash, err := i.Proposed.Hash()
	if err != nil {
		return err
	}
	sig, err := crypto.SignSchnorr(network.Suite, i.Private, hash)
	if err != nil {
		return err
	}
	msg, err := i.Client.Send(i.Cothority.RandomServerIdentity(), &ProposeVote{
		ID:        i.ID,
		Signer:    i.DeviceName,
		Signature: &sig,
	})
	if err != nil {
		return err
	}
	reply, ok := msg.Msg.(ProposeVoteReply)
	if !ok {
		fmt.Println("Device with name: ", i.DeviceName, ": not yet accepted skipblock")
	}
	if ok {
		fmt.Println("Device with name: ", i.DeviceName, ": accepted skipblock")
		_, data, _ := network.UnmarshalRegistered(reply.Data.Data)
		ok, err = i.ValidateConfig(data.(*Config))
		if !ok {
			return err
		}
		log.Lvl2("Threshold reached and signed")
		i.Proposed = nil
	} else {
		log.Lvl2("Threshold not reached")
	}
	return nil
}

func (i *Identity) ConfigUpdateNew() error {
	if i.Cothority == nil || len(i.Cothority.List) == 0 {
		return errors.New("Didn't find any list in the cothority")
	}
	msg, err := i.Client.Send(i.Cothority.RandomServerIdentity(), &ConfigUpdate{ID: i.ID})
	if err != nil {
		return err
	}
	cu := msg.Msg.(ConfigUpdateReply)
	i.Config = cu.Config
	return nil
}

// ConfigUpdate asks if there is any new config available that has already
// been approved by others and updates the local configuration
func (i *Identity) ConfigUpdate() error {
	fmt.Println("ConfigUpdate(): We are device: ", i.DeviceName)
	if i.Cothority == nil || len(i.Cothority.List) == 0 {
		return errors.New("Didn't find any list in the cothority")
	}
	msg, err := i.Client.Send(i.Cothority.RandomServerIdentity(), &ConfigUpdate{ID: i.ID})
	if err != nil {
		return err
	}
	cu := msg.Msg.(ConfigUpdateReply)

	// Validate config
	var ok bool
	ok, err = i.ValidateConfig(cu.Config)
	if !ok {
		return err
	}

	return nil
}

func (i *Identity) ValidateConfig(newconf *Config) (bool, error) {
	fmt.Println("ConfigUpdate()")
	trustedconfig := i.Config
	fmt.Println("latest known block's hash: ", skipchain.SkipBlockID(i.latestID))
	msg, err := i.Client.Send(i.Cothority.RandomServerIdentity(), &GetUpdateChain{LatestID: i.latestID, ID: i.ID})
	if err != nil {
		return false, err
	}
	reply := msg.Msg.(GetUpdateChainReply)
	blocks := reply.Update

	ok := true
	prev := blocks[0]
	fmt.Println(len(blocks), "skipblocks returned, identity: ", i.DeviceName)
	for index, b := range blocks {
		fmt.Println(index, "block with hash: ", b.Hash)
	}

	// Check that the hash of the first block of the returned list is the latest known
	// to us so far
	if !bytes.Equal(prev.Hash, i.latestID) {
		return false, errors.New("Returned chain of skipblocks starts with wrong skipblock hash")
	}

	// Check that the returned valid config is the one included into the last skiblock
	// of the returned list
	//fmt.Println(d)
	//fmt.Println(blocks[len(blocks)-1].Data)
	_, latestconf, _ := network.UnmarshalRegistered(blocks[len(blocks)-1].Data)
	err = newconf.Equal(latestconf.(*Config))
	if err != nil {
		return false, err
	}

	// Check the validity of each skipblock hop
	for index, block := range blocks {
		// Verify that the cothority has signed the forward links
		next := block
		if index > 0 {
			fmt.Println("Checking trust delegation ", index-1, "->", index)
			cnt := 0
			fmt.Println("cnt: ", cnt)
			_, data, err2 := network.UnmarshalRegistered(next.Data)
			if err2 != nil {
				return false, errors.New("Couldn't unmarshal subsequent skipblock's SkipBlockFix field")
			}
			newconfig, ok := data.(*Config)
			if !ok {
				return false, errors.New("Couldn't get type '*Config'")
			}

			for key, newdevice := range newconfig.Device {
				if _, exists := trustedconfig.Device[key]; exists {
					//fmt.Println(newdevice.Point)
					//fmt.Println(trustedconfig.Device[key].Point)
					//if newdevice.Point == trustedconfig.Device[key].Point {

					//fmt.Println("Check whether there is a non-nil signature")
					if newdevice.Vote != nil {
						var hash crypto.HashID
						hash, err = newconfig.Hash()
						if err != nil {
							return false, errors.New("Couldn't get hash")
						}
						fmt.Println("Verify signature of device: ", key)
						err = crypto.VerifySchnorr(network.Suite, newdevice.Point, hash, *newdevice.Vote)
						if err != nil {
							return false, errors.New("Wrong signature")
						}
						cnt++
						fmt.Println(cnt)
					}
				}
			}
			if cnt < trustedconfig.Threshold {
				fmt.Println("number of votes: ", cnt, "threshold: ", trustedconfig.Threshold)
				return false, errors.New("No sufficient threshold of trusted devices' votes")
			}

			fmt.Println("Verify the cothority's signatures regarding the forward links")
			// Verify the cothority's signatures regarding the forward links
			link := prev.ForwardLink[len(prev.ForwardLink)-1]
			//b, err := network.MarshalRegisteredType(next.SkipBlockFix)
			//h, err := crypto.HashBytes(network.Suite.Hash(), b)
			//fmt.Println("Check whether cothority's signature upon wrong skipblock hash")
			if !bytes.Equal(link.Hash, next.Hash) {
				return false, errors.New("Cothority's signature upon wrong skipblock hash")
			}
			//fmt.Println("Check whether cothority's signature verify or not")
			fmt.Println(link.Hash)
			hash := []byte(link.Hash)
			fmt.Println(hash)
			fmt.Println(link.Signature)
			publics := prev.Roster.Publics()
			if prev.Roster != nil {
				for _, key := range publics {
					fmt.Println(key)
				}

				//fmt.Println("XMMM")
				err := cosi.VerifySignature(network.Suite, publics, hash, link.Signature)
				if err != nil {
					return false, errors.New("Cothority's signature doesn't verify")
				}
				/*	err = link.VerifySignature(prev.Roster.Publics())
					if err != nil {
						return false, errors.New("Cothority's signature doesn't verify")
					}
				*/
				fmt.Println("Cothority's signature ok")
			} else {
				return false, errors.New("Found no roster")
			}

		}
		prev = next
		_, data, _ := network.UnmarshalRegistered(prev.Data)
		trustedconfig = data.(*Config)
	}

	i.latestID = ID(blocks[len(blocks)-1].Hash)
	i.Config = newconf
	fmt.Println("Num of device owners: ", len(i.Config.Device))
	//fmt.Println("Returning from ValidateConfig")
	return ok, err
}

func (c1 *Config) Equal(c2 *Config) error {
	d1, _ := c1.Hash()
	d2, _ := c2.Hash()
	if !bytes.Equal(d1, d2) {
		return errors.New("Configs don't match")
	}
	return nil
}
