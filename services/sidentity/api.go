package sidentity

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	//"strings"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/skipchain"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/cosi"
	//"github.com/dedis/crypto/nist"
	"github.com/dedis/crypto/ed25519"
)

/*
This is the external API to access the identity-service. It shows the methods
used to create a new identity-skipchain, propose new configurations and how
to vote on these configurations.
*/

func init() {
	for _, s := range []interface{}{
		// Structures
		&common_structs.Device{},
		&common_structs.APoint{},
		&Identity{},
		&common_structs.Config{},
		&common_structs.PinState{},
		&Storage{},
		&Service{},
		&common_structs.My_Scalar{},
		&common_structs.WSconfig{},

		&common_structs.CAInfo{},
		&common_structs.WSInfo{},
		&common_structs.SiteInfo{},
		&common_structs.Key{},

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
		&GetSkipblocks{},
		&GetSkipblocksReply{},
		&GetValidSbPath{},
		&GetValidSbPathReply{},
		&PropagateIdentity{},
		&PropagateCert{},
		&UpdateSkipBlock{},
		&PushPublicKey{},
		&PushPublicKeyReply{},
		&PullPublicKey{},
		&PullPublicKeyReply{},
		&GetCert{},
		&GetCertReply{},
	} {
		network.RegisterPacketType(s)
	}
}

// Identity structure holds the data necessary for a client/device to use the
// identity-service. Each identity-skipchain is tied to a roster that is defined
// in 'Cothority'
type Identity struct {
	// Client is included for easy `Send`-methods.
	CothorityClient *sda.Client
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
	// Pin-state
	//PinState *common_structs.PinState
	// Client type {"device" or "ws"}
	Ctype string
	// ID of the skipchain this device is tied to.
	ID skipchain.SkipBlockID
	// ID of the latest known skipblock
	LatestID skipchain.SkipBlockID
	// Latest known skipblock
	Latest *skipchain.SkipBlock
	// Config is the actual, valid configuration of the identity-skipchain.
	Config *common_structs.Config
	// Proposed is the new configuration that has not been validated by a
	// threshold of devices.
	Proposed *common_structs.Config
	// DeviceName must be unique in the identity-skipchain.
	DeviceName string
	// Cothority is the roster responsible for the identity-skipchain. It
	// might change in the case of a roster-update.
	Cothority *sda.Roster
	// The current valid cert
	Cert *common_structs.Cert
}

// NewIdentity starts a new identity that can contain multiple managers with
// different accounts
// For the first device to be added (that will vote for the genesis config) the 'data'
// argument should contain the ServerIdentities of the web servers to be attached.
// For the subsequent devices, the 'data' field can contain garbage (its values are
// not taken into account in the code)
//func NewIdentity(cothority *sda.Roster, threshold int, owner string, pinstate *common_structs.PinState, cas []common_structs.CAInfo, data map[string]*common_structs.WSconfig) *Identity {
//	switch pinstate.Ctype {
func NewIdentity(cothority *sda.Roster, fqdn string, threshold int, owner string, ctype string, cas []common_structs.CAInfo, data map[string]*common_structs.WSconfig, duration int64) *Identity {
	switch ctype {
	case "device":
		kp := config.NewKeyPair(network.Suite)
		return &Identity{
			CothorityClient: sda.NewClient(ServiceName),
			Data: Data{
				Private: kp.Secret,
				Public:  kp.Public,
				//PinState:   pinstate,
				Ctype:      ctype,
				Config:     common_structs.NewConfig(fqdn, threshold, kp.Public, owner, cas, data, duration),
				DeviceName: owner,
				Cothority:  cothority,
			},
		}
	case "ws":
		return &Identity{
			CothorityClient: sda.NewClient(ServiceName),
			Data: Data{
				//PinState: pinstate,
				Ctype: ctype,
				// Cothority roster should be given before attempting to reach the service!
			},
		}
	}
	return nil
}

// CreateIdentity asks the sidentityService to create a new SIdentity
func (i *Identity) CreateIdentity() error {
	log.LLvlf2("CreateIdentity(): Start")
	_ = i.Config.SetNowTimestamp()

	// configure the tls keypairs of the web servers (pull their public
	// keys which will be used for the ecnryption of the tls private
	// key of each one)
	proposedConf := i.Config.Copy()
	serverIDs := make([]*network.ServerIdentity, 0)
	for _, server := range i.Config.Data {
		serverIDs = append(serverIDs, server.ServerID)
	}
	_ = i.UpdateTLSKeypairs(proposedConf, serverIDs)
	i.Config = proposedConf.Copy()

	hash, _ := i.Config.Hash()
	log.LLvlf2("Proposed config's hash: %v", hash)
	//log.LLvlf2("CreateIdentity(): 1")
	sig, _ := crypto.SignSchnorr(network.Suite, i.Private, hash)
	i.Config.Device[i.DeviceName].Vote = &sig

	//log.LLvlf2("CreateIdentity(): 2")
	msg, err := i.CothorityClient.Send(i.Cothority.RandomServerIdentity(), &CreateIdentity{i.Config, i.Cothority})
	if err != nil {
		return err
	}
	//log.LLvlf2("CreateIdentity(): 3")
	air := msg.Msg.(CreateIdentityReply)
	i.ID = air.Data.Hash
	i.LatestID = air.Data.Hash
	err = i.ConfigUpdate()
	if err != nil {
		return err
	}
	return nil
}

//func NewIdentityMultDevs(cothority *sda.Roster, threshold int, owners []string, pinstate *common_structs.PinState, cas []common_structs.CAInfo, data map[string]*common_structs.WSconfig) ([]*Identity, error) {
func NewIdentityMultDevs(cothority *sda.Roster, fqdn string, threshold int, owners []string, ctype string, cas []common_structs.CAInfo, data map[string]*common_structs.WSconfig, duration int64) ([]*Identity, error) {
	log.LLvlf2("NewIdentityMultDevs(): Start")
	ids := make([]*Identity, len(owners))
	for index, owner := range owners {
		if index == 0 {
			ids[index] = NewIdentity(cothority, fqdn, threshold, owner, ctype, cas, data, duration)
		} else {
			ids[index] = NewIdentity(cothority, fqdn, threshold, owner, ctype, cas, data, duration)
			if _, exists := ids[0].Config.Device[owner]; exists {
				return nil, errors.New("NewIdentityMultDevs(): Adding with an existing account-name")
			}
			ids[0].Config.Device[owner] = &common_structs.Device{Point: ids[index].Public, Vote: nil}
		}
	}
	return ids, nil
}

// CreateIdentityMultDev asks the sidentityService to create a new SIdentity constituted of multiple
// devices
func (i *Identity) CreateIdentityMultDevs(ids []*Identity) error {
	log.LLvlf2("CreateIdentityMultDevs(): Start")
	_ = i.Config.SetNowTimestamp()

	// configure the tls keypairs of the web servers (pull their public
	// keys which will be used for the ecnryption of the tls private
	// key of each one)
	proposedConf := i.Config.Copy()
	serverIDs := make([]*network.ServerIdentity, 0)
	for _, server := range i.Config.Data {
		serverIDs = append(serverIDs, server.ServerID)
	}
	_ = i.UpdateTLSKeypairs(proposedConf, serverIDs)
	i.Config = proposedConf.Copy()

	hash, _ := i.Config.Hash()
	//log.LLvlf2("Proposed config's hash: %v", hash)
	for _, id := range ids {
		sig, _ := crypto.SignSchnorr(network.Suite, id.Private, hash)
		i.Config.Device[id.DeviceName].Vote = &sig
	}

	msg, err := i.CothorityClient.Send(i.Cothority.RandomServerIdentity(), &CreateIdentity{i.Config, i.Cothority})
	if err != nil {
		return err
	}
	air := msg.Msg.(CreateIdentityReply)
	for _, id := range ids {
		id.ID = air.Data.Hash
		id.LatestID = air.Data.Hash
		err = id.ConfigUpdate()
		if err != nil {
			return err
		}
	}
	return nil
}

// AttachToIdentity proposes to attach a device to an existing Identity
func (i *Identity) AttachToIdentity(ID skipchain.SkipBlockID) error {
	i.ID = ID
	i.LatestID = ID
	i.ConfigUpdate()

	var err error

	switch i.Ctype {
	case "device":
		if _, exists := i.Config.Device[i.DeviceName]; exists {
			log.LLvlf2("AttachToIdentity(): Adding with an existing account-name: %v", i.DeviceName)
			return errors.New("AttachToIdentity(): Adding with an existing account-name")
		}

		if i.Config == nil {
			log.LLvlf2("AttachToIdentity(): Nil config")
			return errors.New("AttachToIdentity(): Nil config")
		}

		confPropose := i.Config.Copy()
		for _, dev := range confPropose.Device {
			dev.Vote = nil
		}

		confPropose.Device[i.DeviceName] = &common_structs.Device{Point: i.Public}
		err = i.ProposeSend(confPropose)

		if err != nil {
			return err
		}
	}
	return nil
}

// ProposeConfig proposes a new skipblock with general modifications (add/revoke one or
// more devices and/or change the threshold and/or change tls keypairs of one or more web servers
// specified by their ServerIdentities as they are given in the argument 'serverIDs')
// Devices to be revoked regarding the proposed config should NOT vote upon their revovation
// (or, in the case of voting, a negative vote is the only one accepted)
func (i *Identity) ProposeConfig(add, revoke map[string]abstract.Point, thr int, duration int64, serverIDs []*network.ServerIdentity) error {
	var err error
	err = i.ConfigUpdate() //common_structs.ConfigUpdateNew() before
	if err != nil {
		return err
	}
	confPropose := i.Config.Copy()
	for _, dev := range confPropose.Device {
		dev.Vote = nil
	}

	for name, point := range add {
		confPropose.Device[name] = &common_structs.Device{Point: point}
	}
	for name, _ := range revoke {
		if _, exists := confPropose.Device[name]; exists {
			delete(confPropose.Device, name)
		}
	}

	if thr != 0 {
		confPropose.Threshold = thr
	}

	if duration != 0 {
		confPropose.MaxDuration = duration
	}

	if serverIDs != nil {
		_ = i.UpdateTLSKeypairs(confPropose, serverIDs)
	}

	err = i.ProposeSend(confPropose)
	if err != nil {
		return err
	}
	return nil
}

// ProposeSend sends the new proposition of this identity
func (i *Identity) ProposeSend(il *common_structs.Config) error {
	log.LLvlf2("Device: %v proposes a config", i.DeviceName)
	err := il.SetNowTimestamp()
	_, err = i.CothorityClient.Send(i.Cothority.RandomServerIdentity(), &ProposeSend{i.ID, il})
	i.Proposed = il
	return err
}

// ProposeUpdate verifies if there is a new configuration awaiting that
// needs approval from clients
func (i *Identity) ProposeUpdate() error {
	msg, err := i.CothorityClient.Send(i.Cothority.RandomServerIdentity(), &ProposeUpdate{
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
	log.LLvlf2("ProposeVote(): device: %v", i.DeviceName)
	if i.Proposed == nil {
		return errors.New("No proposed config")
	}
	//log.LLvlf2("Voting %t on %s", accept, i.Proposed.Device)
	if !accept {
		return nil
	}

	// Check whether our clock is relatively close or not to the proposed timestamp
	//now := time.Now().Unix()
	err := i.Proposed.CheckTimeDiff(maxdiff_sign)
	if err != nil {
		log.LLvlf2("Device: %v %v", i.DeviceName, err)
		return err
	}

	var hash crypto.HashID
	hash, err = i.Proposed.Hash()
	if err != nil {
		return err
	}
	//log.LLvlf2("Proposed config's hash: %v", hash)
	sig, err := crypto.SignSchnorr(network.Suite, i.Private, hash)
	if err != nil {
		return err
	}
	msg, err := i.CothorityClient.Send(i.Cothority.RandomServerIdentity(), &ProposeVote{
		ID:        i.ID,
		Signer:    i.DeviceName,
		Signature: &sig,
	})
	if err != nil {
		return err
	}
	reply, ok := msg.Msg.(ProposeVoteReply)
	if !ok {
		log.LLvlf2("Device with name: %v : not yet accepted skipblock", i.DeviceName)
	}
	if ok {
		log.LLvlf2("Device with name: %v : accepted skipblock", i.DeviceName)
		_, data, _ := network.UnmarshalRegistered(reply.Data.Data)
		ok, err = i.ValidateUpdateConfig(data.(*common_structs.Config))
		if !ok {
			return err
		}
		log.LLvl2("Threshold reached and signed")
		i.Proposed = nil
	} else {
		log.LLvl2("Threshold not reached")
	}
	return nil
}

// ConfigUpdate asks if there is any new config available that has already
// been approved by others and updates the local configuration
func (i *Identity) ConfigUpdate() error {
	if i.Ctype == "device" {
		log.Lvlf2("ConfigUpdate(): We are device: %v", i.DeviceName)
	}
	if i.Cothority == nil || len(i.Cothority.List) == 0 {
		return errors.New("Didn't find any list in the cothority")
	}
	//log.LLvlf2("ConfigUpdate(): ID: %v", i.ID)
	msg, err := i.CothorityClient.Send(i.Cothority.RandomServerIdentity(), &ConfigUpdate{ID: i.ID})
	if err != nil {
		return err
	}
	cu := msg.Msg.(ConfigUpdateReply)

	// Validate config
	//log.LLvlf2("Before invoking ValidateUpdateConfig()")
	var ok bool
	ok, err = i.ValidateUpdateConfig(cu.Config)
	if !ok {
		return err
	}
	//log.LLvlf2("ValidateUpdateConfig(): End")
	return nil
}

func (i *Identity) ValidateConfigChain(newconf *common_structs.Config, blocks []*skipchain.SkipBlock) error {

	prev := blocks[0]
	for index, b := range blocks {
		log.Lvlf2("%v block with hash: %v", index, b.Hash)
	}

	// Check that the hash of the first block of the returned list is the latest known
	// to us so far
	if !bytes.Equal(prev.Hash, i.LatestID) {
		log.LLvlf2("Returned chain of skipblocks starts with wrong skipblock hash (prev.Hash=%v, i.LatestID=%v)", prev.Hash, i.LatestID)
		return errors.New("Returned chain of skipblocks starts with wrong skipblock hash")
	}

	// TODO: Check that the returned valid config is the one included into the last skipblock
	// of the returned list
	b1 := blocks[len(blocks)-1].Data
	/*b2, _ := network.MarshalRegisteredType(newconf)
	if !bytes.Equal(b1, b2) {
		log.Lvlf2("Configs don't match!")
		return false, err
	}
	*/
	_, data, _ := network.UnmarshalRegistered(b1)
	c, _ := data.(*common_structs.Config)
	h1, _ := c.Hash()
	h2, _ := newconf.Hash()
	log.Lvlf2("h1: %v, h2: %v", h1, h2)

	if !bytes.Equal(h1, h2) {
		log.LLvlf2("Configs don't match!")
		return errors.New("Configs don't match!")
	}

	var trustedconfig *common_structs.Config
	var err error
	// Check the validity of each skipblock hop
	for index, block := range blocks {
		next := block
		if index > 0 {
			log.LLvlf2("Checking trust delegation: %v -> %v", index-1, index)
			cnt := 0
			//fmt.Println("cnt: ", cnt)
			_, data, err2 := network.UnmarshalRegistered(next.Data)
			if err2 != nil {
				return errors.New("Couldn't unmarshal subsequent skipblock's SkipBlockFix field")
			}
			newconfig, ok := data.(*common_structs.Config)
			if !ok {
				return errors.New("Couldn't get type '*Config'")
			}

			for key, newdevice := range newconfig.Device {
				if _, exists := trustedconfig.Device[key]; exists {
					b1, _ := network.MarshalRegisteredType(newdevice.Point)
					b2, _ := network.MarshalRegisteredType(trustedconfig.Device[key].Point)
					if bytes.Equal(b1, b2) {
						//fmt.Println("Check whether there is a non-nil signature")
						if newdevice.Vote != nil {
							var hash crypto.HashID
							hash, err = newconfig.Hash()
							if err != nil {
								log.Lvlf2("Couldn't get hash")
								return errors.New("Couldn't get hash")
							}
							//log.LLvlf2("Verify signature of device: %v", key)
							err = crypto.VerifySchnorr(network.Suite, newdevice.Point, hash, *newdevice.Vote)
							if err != nil {
								log.LLvlf2("Wrong signature")
								return errors.New("Wrong signature")
							}
							cnt++
							//fmt.Println(cnt)
						}
					}
				}
			}
			if cnt < trustedconfig.Threshold {
				log.LLvlf2("number of votes: %v, threshold: %v", cnt, trustedconfig.Threshold)
				return errors.New("No sufficient threshold of trusted devices' votes")
			}

			//log.Lvlf2("Verify the cothority's signatures regarding the forward links")
			// Verify the cothority's signatures regarding the forward links
			link := prev.ForwardLink[len(prev.ForwardLink)-1]

			//fmt.Println("Check whether cothority's signature upon wrong skipblock hash")
			if !bytes.Equal(link.Hash, next.Hash) {
				log.LLvlf2("Cothority's signature upon wrong skipblock hash")
				return errors.New("Cothority's signature upon wrong skipblock hash")
			}
			//fmt.Println("Check whether cothority's signature verify or not")
			hash := []byte(link.Hash)
			publics := prev.Roster.Publics()
			if prev.Roster != nil {
				err := cosi.VerifySignature(network.Suite, publics, hash, link.Signature)
				if err != nil {
					log.Lvlf2("Cothority's signature NOT ok")
					return errors.New("Cothority's signature doesn't verify")
				}
			} else {
				log.LLvlf2("Found no roster")
				return errors.New("Found no roster")
			}

		}
		prev = next
		_, data, _ := network.UnmarshalRegistered(prev.Data)
		trustedconfig = data.(*common_structs.Config)
	}
	log.LLvlf2("ValidateConfigChain(): End")
	return nil
}

func (i *Identity) ValidateUpdateConfig(newconf *common_structs.Config) (bool, error) {
	log.LLvlf2("ValidateUpdateConfig(): Start")
	//trustedconfig := i.Config
	log.LLvlf2("%v", i.LatestID)
	msg, err := i.CothorityClient.Send(i.Cothority.RandomServerIdentity(), &GetUpdateChain{LatestID: i.LatestID, ID: i.ID})
	if err != nil {
		return false, err
	}
	reply := msg.Msg.(GetUpdateChainReply)
	blocks := reply.Update
	log.Lvlf2("skipblocks returned: %v, identity: %v", len(blocks), i.DeviceName)

	err = i.ValidateConfigChain(newconf, blocks)
	if err != nil {
		return false, err
	}

	i.LatestID = blocks[len(blocks)-1].Hash
	i.Latest = blocks[len(blocks)-1]
	i.Config = newconf
	i.Cert = reply.Cert
	log.Lvlf2("ValidateUpdateConfig(): DEVICE: %v, End with NUM_DEVICES: %v, THR: %v", i.DeviceName, len(i.Config.Device), i.Config.Threshold)
	//fmt.Println("Returning from ValidateUpdatecommon_structs.Config")
	return true, nil
}

// if h2==0, fetch all the skipblocks until the current head one
func (i *Identity) GetValidSbPath(id skipchain.SkipBlockID, h1 skipchain.SkipBlockID, h2 skipchain.SkipBlockID) ([]*skipchain.SkipBlock, error) {
	log.LLvlf2("GetValidSbPath(): Start")
	msg, err := i.CothorityClient.Send(i.Cothority.RandomServerIdentity(), &GetValidSbPath{ID: id, Hash1: h1, Hash2: h2})
	if err != nil {
		return nil, err
	}
	reply, _ := msg.Msg.(GetValidSbPathReply)
	sbs := reply.Skipblocks

	// check the trust delegation between each pair of subsequent skipblocks/configs
	_, data, err2 := network.UnmarshalRegistered(sbs[len(sbs)-1].Data)
	if err2 != nil {
		return nil, errors.New("Couldn't unmarshal subsequent skipblock's SkipBlockFix field")
	}
	newconf, ok := data.(*common_structs.Config)
	if !ok {
		return nil, errors.New("Couldn't get type '*Config'")
	}
	err = i.ValidateConfigChain(newconf, sbs)
	if err != nil {
		return nil, err
	}
	log.LLvlf2("GetValidSbPath(): End")
	return sbs, nil
}

// fetch the current valid certs for the site (not yet expired)
func (i *Identity) GetCert(id skipchain.SkipBlockID) (*common_structs.Cert, skipchain.SkipBlockID, error) {
	msg, err := i.CothorityClient.Send(i.Cothority.RandomServerIdentity(), &GetCert{ID: i.ID})
	if err != nil {
		return nil, nil, err
	}
	reply, _ := msg.Msg.(GetCertReply)
	cert := reply.Cert
	hash := reply.SbHash
	return cert, hash, nil
}

// for web servers (public key to be pushed to the cothority servers)
func (i *Identity) PushPublicKey(public abstract.Point, serverID *network.ServerIdentity) error {
	//log.LLvlf2("sidentity.API's PushPublicKey(): Start")
	roster := i.Cothority
	msg, err := i.CothorityClient.Send(i.Cothority.RandomServerIdentity(), &PushPublicKey{Roster: roster, Public: public, ServerID: serverID})
	if err != nil {
		return err
	}
	_, _ = msg.Msg.(PushPublicKeyReply)
	//log.LLvlf2("sidentity.API's PushPublicKey(): End")
	return nil
}

// for devices (public key to be pulled from one of the cothority servers)
func (i *Identity) PullPublicKey(serverID *network.ServerIdentity) (abstract.Point, error) {
	msg, err := i.CothorityClient.Send(i.Cothority.RandomServerIdentity(), &PullPublicKey{ServerID: serverID})
	if err != nil {
		return nil, err
	}
	reply, _ := msg.Msg.(PullPublicKeyReply)
	public := reply.Public
	return public, nil
}

/*
func (i *Identity) GetSkipblocks(id skipchain.SkipBlockID, latest *skipchain.SkipBlock) ([]*skipchain.SkipBlock, error) {
	msg, err := i.CothorityClient.Send(i.Cothority.RandomServerIdentity(), &GetSkipblocks{ID: i.ID, Latest: latest})
	if err != nil {
		return nil, err
	}
	reply, _ := msg.Msg.(GetSkipblocksReply)
	sbs := reply.Skipblocks

	// check the trust delegation between each pair of subsequent skipblocks/configs
	_, data, err2 := network.UnmarshalRegistered(sbs[len(sbs)-1].Data)
	if err2 != nil {
		return nil, errors.New("Couldn't unmarshal subsequent skipblock's SkipBlockFix field")
	}
	newconf, ok := data.(*common_structs.Config)
	if !ok {
		return nil, errors.New("Couldn't get type '*Config'")
	}
	err = i.ValidateConfigChain(newconf, sbs)
	if err != nil {
		return nil, err
	}

	return sbs, nil
}
*/

// NewIdentityFromCothority searches for a given cothority
func NewIdentityFromCothority(el *sda.Roster, id skipchain.SkipBlockID) (*Identity, error) {
	iden := &Identity{
		CothorityClient: sda.NewClient(ServiceName),
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
		CothorityClient: sda.NewClient(ServiceName),
		Data:            *id,
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
func (i *Identity) GetProposed() *common_structs.Config {
	if i.Proposed != nil {
		return i.Proposed
	}
	return i.Config.Copy()
}

func (i *Identity) ProposeUpVote() {
	log.ErrFatal(i.ProposeUpdate())
	log.ErrFatal(i.ProposeVote(true))
}

// updates the tls keypairs (as they appear into the 'proposedConf' config) of the web servers
// whose ServerIdentities are given via the argument 'serverIDs'
func (i *Identity) UpdateTLSKeypairs(proposedConf *common_structs.Config, serverIDs []*network.ServerIdentity) error {
	// configure the tls keypair of each web server (using its public key to encrypt its tls private key)
	for _, serverID := range serverIDs {
		// pull from the cothority the web server's public key
		public, _ := i.PullPublicKey(serverID)

		tls_keypair := config.NewKeyPair(network.Suite)
		tls_public := tls_keypair.Public
		tls_private := tls_keypair.Secret
		log.LLvlf2("serverID: %v", serverID)
		log.LLvlf2("tls_public: %v", tls_public)
		log.LLvlf2("tls_private: %v", tls_private)
		newstruct := common_structs.My_Scalar{Private: tls_private}
		tls_private_buf, _ := network.MarshalRegisteredType(&newstruct)
		//log.LLvlf2("tls_private_buf: %v", tls_private_buf)

		tls_private_buf1 := tls_private_buf[0:25]
		tls_private_buf2 := tls_private_buf[25:len(tls_private_buf)]

		suite := ed25519.NewAES128SHA256Ed25519(false)

		// ElGamal-encrypt a message (tls private key) using the public key of the web server.
		K1, C1, _ := common_structs.ElGamalEncrypt(suite, public, tls_private_buf1)
		K2, C2, _ := common_structs.ElGamalEncrypt(suite, public, tls_private_buf2)
		//log.LLvlf2("K1: %v, C1: %v", K1, C1)
		//log.LLvlf2("K1: %v, C1: %v", K2, C2)
		/*
			_, data, _ := network.UnmarshalRegistered(tls_private_buf)
			log.LLvlf2("data: %v", data)
			rec := data.(*common_structs.My_Scalar)
			tlsprivate_ := rec.Private
			log.LLvlf2("%v", tlsprivate_)
		*/
		key := fmt.Sprintf("tls:%v", serverID)
		proposedConf.Data[key].TLSPublic = tls_public
		proposedConf.Data[key].K1 = K1
		proposedConf.Data[key].C1 = C1
		proposedConf.Data[key].K2 = K2
		proposedConf.Data[key].C2 = C2

	}
	return nil
}
