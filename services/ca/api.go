package ca

import (
	//"bytes"
	"errors"
	//"io"
	//"strings"

	//"fmt"
	//"io/ioutil"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/common_structs"
	"github.com/dedis/cothority/services/skipchain"
	//"github.com/dedis/crypto/abstract"
	//"github.com/dedis/crypto/config"
	//"github.com/dedis/crypto/cosi"
)

/*
This is the external API to access the ca-service. It shows the methods
used to ask for a new cert
*/

func init() {
	for _, s := range []interface{}{
		// Structures
		&common_structs.Config{},
		&Site{},
		&CA{},
		// API messages
		&CSR{},
		&CSRReply{},
		&GetPublicKey{},
		&GetPublicKeyReply{},
	} {
		network.RegisterPacketType(s)
	}
}

type CSRDispatcher struct {
	// Client is included for easy `Send`-methods.
	CAClient *sda.Client
	// Data holds all the data related to this identity
	// It can be stored and loaded from a config file.
	Data
}

type Data struct {
	// ID of the site skipchain for which the cert is going to be signed
	ID skipchain.SkipBlockID
	// Proposed is the new configuration that has not already signed by a CA
	Proposed *common_structs.Config
	CAs      []common_structs.CAInfo
	// The available certs
	Certs []*common_structs.Cert
}

func NewCSRDispatcher() *CSRDispatcher {
	return &CSRDispatcher{
		CAClient: sda.NewClient(ServiceCAName),
	}
}

func (d *CSRDispatcher) SignCert(config, prevconfig *common_structs.Config, id skipchain.SkipBlockID) ([]*common_structs.Cert, error) {
	log.LLvlf2("CSRDispatcher(): Start")
	d.Certs = make([]*common_structs.Cert, 0)
	d.Data.ID = id // id of the site (hash of the genesis block)
	d.Data.Proposed = config
	d.Data.CAs = config.CAs
	log.LLvlf2("SignCert(): Start with %v certs", len(d.Certs))
	// Dispatch the CSR to all the listed CAs
	for _, ca := range d.CAs {
		public := ca.Public
		//log.Lvlf2("public: %v", public)
		//log.Print("CSRDispatcher(): 1")
		//log.Print(ca.Public)
		//log.Print(ca.ServerID)

		msg, err := d.CAClient.Send(ca.ServerID, &CSR{ID: d.ID, Config: d.Proposed, PrevConfig: prevconfig})
		if err != nil {
			return nil, err
		}

		cert := msg.Msg.(CSRReply).Cert
		//log.Lvlf2("cert with ID: %v, Hash: %v, Sig: %v, Public: %v", cert.ID, cert.Hash, *cert.Signature, cert.Public)
		//log.Print("CSRDispatcher(): 3")
		// Verify that the chosen CA (having public key 'public') has properly signed the cert
		hash, _ := d.Proposed.Hash()
		//log.Lvlf2("hash: %v", hash)
		//log.Lvlf2("public: %v", public)
		err = crypto.VerifySchnorr(network.Suite, public, hash, *cert.Signature)
		if err != nil {
			log.LLvlf2("CA's signature doesn't verify (CA's public key: %v", public)
			return nil, errors.New("CA's signature doesn't verify")
		}
		//log.Print("CSRDispatcher(): 4")
		d.Certs = append(d.Certs, cert)
		//log.Print("CSRDispatcher(): 5")
	}
	log.LLvlf2("CSRDispatcher(): End: %v certs signed properly", len(d.Certs))
	return d.Certs, nil
}
