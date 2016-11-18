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
	Certs []*Cert
}

func NewCSRDispatcher() *CSRDispatcher {
	return &CSRDispatcher{
		CAClient: sda.NewClient(ServiceCAName),
	}
}

func (d *CSRDispatcher) SignCert(config *common_structs.Config, id skipchain.SkipBlockID) ([]*Cert, error) {
	log.Print("CSRDispatcher(): Start")

	d.Data.ID = id
	d.Data.Proposed = config
	d.Data.CAs = config.CAs

	// Dispatch the CSR to all the listed CAs
	for _, ca := range d.CAs {
		public := ca.Public
		//log.Printf("public: %v", public)
		//log.Print("CSRDispatcher(): 1")
		//log.Print(ca.Public)
		//log.Print(ca.ServerID)
		msg, err := d.CAClient.Send(ca.ServerID, &CSR{ID: d.ID, Config: d.Proposed})
		if err != nil {
			return nil, err
		}
		//log.Print("CSRDispatcher(): 2")
		cert := msg.Msg.(CSRReply).Cert
		//log.Printf("cert with ID: %v, Hash: %v, Sig: %v, Public: %v", cert.ID, cert.Hash, *cert.Signature, cert.Public)
		//log.Print("CSRDispatcher(): 3")
		// Verify that the chosen CA (having public key 'public') has properly signed the cert
		hash, _ := d.Proposed.Hash()
		//log.Printf("hash: %v", hash)
		//log.Printf("public: %v", public)
		err = crypto.VerifySchnorr(network.Suite, public, hash, *cert.Signature)
		if err != nil {
			return nil, errors.New("CA's signature doesn't verify")
		}
		//log.Print("CSRDispatcher(): 4")
		d.Certs = append(d.Certs, cert)
		//log.Print("CSRDispatcher(): 5")
	}

	return d.Certs, nil
}
