package sidentity

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/dedis/cothority/dns_id/common_structs"
	"github.com/dedis/cothority/dns_id/skipchain"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/ed25519"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
	"github.com/dedis/cothority/bftcosi"
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
		&CreateIdentityLight{},
		&CreateIdentityLightReply{},
		&ConfigUpdate{},
		&ConfigUpdateReply{},
		&ProposeSend{},
		&ProposeSendChain{},
		&ProposeUpdate{},
		&ProposeUpdateReply{},
		&ProposeVote{},
		&Data{},
		&GetValidSbPath{},
		&GetValidSbPathReply{},
		&GetValidSbPathLight{},
		&GetValidSbPathLightReply{},
		&PropagateIdentity{},
		&PropagateIdentityLight{},
		&PropagateCert{},
		&PropagatePoF{},
		&UpdateSkipBlock{},
		&PushPublicKey{},
		&PushPublicKeyReply{},
		&PullPublicKey{},
		&PullPublicKeyReply{},
		&GetCert{},
		&GetCertReply{},
		&GetPoF{},
		&GetPoFReply{},
		&LockIdentities{},

		&common_structs.PushedPublic{},
		&bftcosi.BFTSignature{},
	} {
		network.RegisterMessage(s)
	}
}

// Identity structure holds the data necessary for a client/device to use the
// identity-service. Each identity-skipchain is tied to a roster that is defined
// in 'Cothority'
type Identity struct {
	// Client is included for easy `Send`-methods.
	CothorityClient *onet.Client
	// IdentityData holds all the data related to this identity
	// It can be stored and loaded from a config file.
	Data
}

// Data contains the data that will be stored / loaded from / to a file
// that enables a client to use the Identity service.
type Data struct {
	sync.Mutex
	// Private key for that device.
	Private abstract.Scalar
	// Public key for that device - will be stored in the identity-skipchain.
	Public abstract.Point
	// Client type {"device" or "ws"}
	Ctype string
	// ID of the skipchain this device is tied to.
	ID []byte
	// ID of the latest known skipblock
	LatestID []byte
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
	Cothority *onet.Roster
	// The current valid cert
	Cert *common_structs.Cert
}

// NewIdentity starts a new identity that can contain multiple managers with
// different accounts
// For the first device to be added (that will vote for the genesis config) the 'data'
// argument should contain the ServerIdentities of the web servers to be attached.
// For the subsequent devices, the 'data' field can contain garbage (its values are
// not taken into account in the code)
//func NewIdentity(cothority *onet.Roster, threshold int, owner string, pinstate *common_structs.PinState, cas []common_structs.CAInfo, data map[string]*common_structs.WSconfig) *Identity {
//	switch pinstate.Ctype {
func NewIdentity(cothority *onet.Roster, fqdn string, threshold int, owner string, ctype string, data map[string]*common_structs.WSconfig) *Identity {
	switch ctype {
	case "device":
		for _, server := range cothority.List {
			log.Lvlf3("---------------%v", server)
		}

		kp := config.NewKeyPair(network.Suite)
		return &Identity{
			CothorityClient: onet.NewClient(ServiceName),
			Data: Data{
				Private:    kp.Secret,
				Public:     kp.Public,
				Ctype:      ctype,
				Config:     common_structs.NewConfig(network.Suite, fqdn, threshold, kp.Public, cothority, owner, data),
				DeviceName: owner,
				Cothority:  cothority,
			},
		}
	case "ws":
		return &Identity{
			CothorityClient: onet.NewClient(ServiceName),
			Data: Data{
				Ctype: ctype,
				// Cothority roster should be given before attempting to reach the service!
			},
		}
	}
	return nil
}


func (i *Identity) CreateIdentityLight() error {
	log.Lvlf2("CreateIdentityLight(): Start")
	_ = i.Config.SetNowTimestamp()
	i.Config.BLink =[]byte{0}

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
	log.Lvlf3("Proposed (genesis) config's hash: %v", hash)

	sig, _ := crypto.SignSchnorr(network.Suite, i.Private, hash)
	i.Config.Device[i.DeviceName].Vote = &sig

	air := &CreateIdentityLightReply{}
	cerr := i.CothorityClient.SendProtobuf(i.Cothority.RandomServerIdentity(), &CreateIdentityLight{i.Config, i.Cothority}, air)
	if cerr != nil {
		return cerr
	}

	i.ID = hash
	i.LatestID = hash
	err := i.ConfigUpdateLight()
	if err != nil {
		return err
	}
	return nil
}


func NewIdentityMultDevs(cothority *onet.Roster, fqdn string, threshold int, owners []string, ctype string, data map[string]*common_structs.WSconfig) ([]*Identity, error) {
	log.Lvlf2("NewIdentityMultDevs(): Start")
	ids := make([]*Identity, len(owners))
	for index, owner := range owners {
		if index == 0 {
			ids[0] = NewIdentity(cothority, fqdn, threshold, owner, ctype,  data)
		} else {
			ids[index] = NewIdentity(cothority, fqdn, threshold, owner, ctype, data)
			if _, exists := ids[0].Config.Device[owner]; exists {
				return nil, errors.New("NewIdentityMultDevs(): Adding with an existing account-name")
			}
			ids[0].Config.Device[owner] = &common_structs.Device{Point: ids[index].Public, Vote: nil}
		}
	}
	return ids, nil
}


// AttachToIdentity proposes to attach a device to an existing Identity
func (i *Identity) AttachToIdentity(ID []byte) error {
	i.ID = ID
	i.LatestID = ID
	i.ConfigUpdateLight()

	var err error

	switch i.Ctype {
	case "device":
		if _, exists := i.Config.Device[i.DeviceName]; exists {
			log.Lvlf2("AttachToIdentity(): Adding with an existing account-name: %v", i.DeviceName)
			return errors.New("AttachToIdentity(): Adding with an existing account-name")
		}

		if i.Config == nil {
			log.Lvlf2("AttachToIdentity(): Nil config")
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
// Devices to be revoked regarding the proposed config should NOT vote upon their revocation
// (or, in the case of voting, a negative vote is the only one accepted)
func (i *Identity) ProposeConfig(add, revoke map[string]abstract.Point, thr int, serverIDs []*network.ServerIdentity) error {
	log.Lvlf2("ProposeConfig")
	var err error
	err = i.ConfigUpdateLight()
	if err != nil {
		return err
	}
	confPropose := i.Config.Copy()
	confPropose.BLink, _ = i.Config.Hash()

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

	if serverIDs != nil {
		_ = i.UpdateTLSKeypairs(confPropose, serverIDs)
	}

	err = i.ProposeSend(confPropose)
	if err != nil {
		return err
	}
	return nil
}

func (i *Identity) EvolveChain(add, revoke map[string]abstract.Point, thr int, serverIDs []*network.ServerIdentity, num_blocks int) error {
	log.Lvlf2("EvolveChain")
	var err error
	err = i.ConfigUpdateLight()
	if err != nil {
		return err
	}

	blocks := make([]*common_structs.Config, 0)
	for idx:=0; idx<num_blocks; idx++ {
		log.LLvlf2("evol: %v", idx+1)
		confPropose := i.Config.Copy()
		confPropose.BLink, _ = i.Config.Hash()

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

		if serverIDs != nil {
			_ = i.UpdateTLSKeypairs(confPropose, serverIDs)
		}
		confPropose.SetNowTimestamp()

		var hash []byte
		hash, err = confPropose.Hash()
		if err != nil {
			return err
		}

		sig, err := crypto.SignSchnorr(network.Suite, i.Private, hash)
		if err != nil {
			return err
		}

		confPropose.Device[i.DeviceName].Vote = &sig

		i.Config = confPropose.Copy()
		blocks = append(blocks, confPropose.Copy())
	}

	if num_blocks>0 {
		err = i.CothorityClient.SendProtobuf(i.Cothority.RandomServerIdentity(), &ProposeSendChain{ID: i.ID, Blocks: blocks}, nil)
		if err != nil {
			log.ErrFatal(err)
		}
	}
	return nil
}



// ProposeSend sends the new proposition of this identity
func (i *Identity) ProposeSend(il *common_structs.Config) error {
	log.LLvlf2("Device: %v proposes a config", i.DeviceName)
	err := il.SetNowTimestamp()
	if err != nil {
		return err
	}
	err = i.CothorityClient.SendProtobuf(i.Cothority.RandomServerIdentity(), &ProposeSend{i.ID, il}, nil)
	if err != nil {
		log.ErrFatal(err)
	}
	return nil
}


// ProposeUpdate verifies if there is a new configuration awaiting that
// needs approval from clients
func (i *Identity) ProposeUpdate() error {
	log.Lvl2("ProposeUpdate")
	reply := &ProposeUpdateReply{}
	cerr := i.CothorityClient.SendProtobuf(i.Cothority.RandomServerIdentity(), &ProposeUpdate{
		ID: i.ID,
	}, reply)
	if cerr != nil {
		return cerr
	}
	i.Proposed = reply.Propose.Copy()
	log.Lvlf2("Proposal received: %v", i.Proposed)
	return nil
}


// ProposeVote calls the 'accept'-vote on the current propose-configuration
func (i *Identity) ProposeVote(accept bool) error {
	log.Lvlf2("ProposeVote(): device: %v", i.DeviceName)
	if i.Proposed == nil {
		return errors.New("No proposed config")
	}

	if !accept {
		return nil
	}

	// Check whether our clock is relatively close or not to the proposed timestamp
	err := i.Proposed.CheckTimeDiff(maxdiff_sign)
	if err != nil {
		log.Lvlf2("Device: %v %v", i.DeviceName, err)
		return err
	}

	var hash []byte
	hash, err = i.Proposed.Hash()
	if err != nil {
		return err
	}

	sig, err := crypto.SignSchnorr(network.Suite, i.Private, hash)
	if err != nil {
		return err
	}
	log.Lvl2("ProposeVote(): before sending vote to one of the timestampers")
	i.CothorityClient.SendProtobuf(i.Cothority.RandomServerIdentity(), &ProposeVote{
		ID:        i.ID,
		Signer:    i.DeviceName,
		Signature: &sig,
	}, nil)

	log.Lvl2("ProposeVote() ends")
	return nil
}




func (i *Identity) ConfigUpdateLight() error {
	log.Lvlf2("ConfigUpdateLight")
	if i.Ctype == "device" {
		log.Lvlf3("ConfigUpdate(): We are device: %v", i.DeviceName)
	}
	if i.Cothority == nil || len(i.Cothority.List) == 0 {
		return errors.New("Didn't find any list in the cothority")
	}
	log.Lvlf2("device %v, latest block's hash: %v", i.DeviceName, i.LatestID)
	blocks, cert, _, _, _, err := i.GetValidSbPathLight(i.ID, i.LatestID, []byte{0})
	if err != nil {
		return err
	}

	err = i.ValidateConfigChainLight(blocks)
	if err != nil {
		return err
	}

	i.Config = blocks[len(blocks)-1].Copy()
	i.LatestID,_ = i.Config.Hash()
	i.Cert = cert
	return nil
}



func (i *Identity) ValidateConfigChainLight(blocks []*common_structs.Config) error {
	log.Lvl2("ValidateConfigChainLight starts")
	var err error
	for index, b := range blocks {
		log.Lvlf3("%v block with hash: %v", index, b.Hash)
	}

	// Check that the hash of the first block/config of the returned list is the latest known
	// to us so far
	trustedconfig := blocks[0].Copy()
	hash,_ := trustedconfig.Hash()
	if !bytes.Equal(hash, i.LatestID) {
		log.Lvlf2("%v %v: Returned chain of skipblocks starts with wrong skipblock hash (prev.Hash=%v, i.LatestID=%v)", i.Ctype, i.DeviceName, hash, i.LatestID)
		return errors.New("Returned chain of skipblocks starts with wrong skipblock hash")
	}

	// Check the validity of each skipblock hop
	for index, block := range blocks {
		newconfig := block
		if index > 0 {
			log.Lvlf2("Checking trust delegation: %v -> %v", index-1, index)
			cnt := 0
			for key, newdevice := range newconfig.Device {
				if _, exists := trustedconfig.Device[key]; exists {
					b1, _ := network.Marshal(newdevice.Point)
					b2, _ := network.Marshal(trustedconfig.Device[key].Point)
					if bytes.Equal(b1, b2) {
						if newdevice.Vote != nil {
							var hash []byte
							hash, err = newconfig.Hash()
							if err != nil {
								log.Lvlf2("Couldn't get hash")
								return errors.New("Couldn't get hash")
							}

							err = crypto.VerifySchnorr(network.Suite, newdevice.Point, hash, *newdevice.Vote)
							if err != nil {
								log.Lvlf2("Wrong signature")
								return errors.New("Wrong signature")
							}
							cnt++
						}
					}
				}
			}
			if cnt < trustedconfig.Threshold {
				log.Lvlf1("number of votes: %v, threshold: %v", cnt, trustedconfig.Threshold)
				return errors.New("No sufficient threshold of trusted devices' votes")
			}
		}
		trustedconfig = newconfig.Copy()
	}
	log.Lvlf3("ValidateConfigChain(): End")
	return nil
}



func (i *Identity) GetValidSbPathLight(id []byte, h1 []byte, h2 []byte) ([]*common_structs.Config, *common_structs.Cert, []byte, *common_structs.SignatureResponse, *network.ServerIdentity, error) {
	log.Lvlf2("GetValidSbPathLight(): Start")
	reply := &GetValidSbPathLightReply{}
	sendTo := i.Cothority.RandomServerIdentity()
	cerr := i.CothorityClient.SendProtobuf(sendTo, &GetValidSbPathLight{ID: id, Hash1: h1, Hash2: h2}, reply)
	if cerr != nil {
		return nil, nil, nil, nil, sendTo, cerr
	}
	log.Lvlf2("GetValidSbPathLight(): Received %v blocks from cothority", len(reply.Configblocks))
	sbs := reply.Configblocks
	cert := reply.Cert
	hash := reply.Hash
	pof := reply.PoF

	// check the trust delegation between each pair of subsequent skipblocks/configs	_
	err := i.ValidateConfigChainLight(sbs)
	if err != nil {
		return nil, nil, nil, nil, sendTo, err
	}
	log.Lvlf2("GetValidSbPathLight(): End")
	return sbs, cert, hash, pof, sendTo, nil
}

// fetch the current valid cert for the site (not yet expired)
func (i *Identity) GetCert(id []byte) (*common_structs.Cert, []byte, error) {
	reply := &GetCertReply{}
	cerr := i.CothorityClient.SendProtobuf(i.Cothority.RandomServerIdentity(), &GetCert{ID: i.ID}, reply)
	if cerr != nil {
		return nil, nil, cerr
	}

	cert := reply.Cert
	hash := reply.SbHash
	return cert, hash, nil
}

// fetch the latest PoF for the (latest) skipblock of the site
func (i *Identity) GetPoF(id []byte) (*common_structs.SignatureResponse, []byte, error) {
	reply := &GetPoFReply{}
	cerr := i.CothorityClient.SendProtobuf(i.Cothority.RandomServerIdentity(), &GetPoF{ID: i.ID}, reply)
	if cerr != nil {
		return nil, nil, cerr
	}

	pof := reply.PoF
	hash := reply.SbHash
	return pof, hash, nil
}

// for web servers (public key to be pushed to the cothority servers)
func (i *Identity) PushPublicKey(public abstract.Point, serverID *network.ServerIdentity) error {
	roster := i.Cothority
	cerr := i.CothorityClient.SendProtobuf(i.Cothority.RandomServerIdentity(), &PushPublicKey{Roster: roster, Public: public, ServerID: serverID}, nil)
	if cerr != nil {
		return cerr
	}
	return nil
}

// for devices (public key to be pulled from one of the cothority servers)
func (i *Identity) PullPublicKey(serverID *network.ServerIdentity) (abstract.Point, error) {
	reply := &PullPublicKeyReply{}
	cerr := i.CothorityClient.SendProtobuf(i.Cothority.RandomServerIdentity(), &PullPublicKey{ServerID: serverID}, reply)
	if cerr != nil {
		return nil, cerr
	}

	public := reply.Public
	return public, nil
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
		log.Lvlf2("serverID: %v", serverID)
		log.Lvlf2("tls_public: %v", tls_public)
		log.Lvlf2("tls_private: %v", tls_private)
		newstruct := common_structs.My_Scalar{Private: tls_private}
		tls_private_buf, _ := network.Marshal(&newstruct)

		tls_private_buf1 := tls_private_buf[0:25]
		tls_private_buf2 := tls_private_buf[25:len(tls_private_buf)]

		suite := ed25519.NewAES128SHA256Ed25519(false)

		// ElGamal-encrypt a message (tls private key) using the public key of the web server.
		K1, C1, _ := common_structs.ElGamalEncrypt(suite, public, tls_private_buf1)
		K2, C2, _ := common_structs.ElGamalEncrypt(suite, public, tls_private_buf2)

		key := fmt.Sprintf("tls:%v", serverID)
		proposedConf.Data[key].TLSPublic = tls_public
		proposedConf.Data[key].K1 = K1
		proposedConf.Data[key].C1 = C1
		proposedConf.Data[key].K2 = K2
		proposedConf.Data[key].C2 = C2

	}
	return nil
}
