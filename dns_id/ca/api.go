package ca

import (
	"errors"

	"github.com/dedis/cothority/dns_id/common_structs"
	"github.com/dedis/cothority/dns_id/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/crypto"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
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
	CAClient *onet.Client
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
		CAClient: onet.NewClient(ServiceCAName),
	}
}

func (d *CSRDispatcher) SignCert(config, prevconfig *common_structs.Config, id skipchain.SkipBlockID) ([]*common_structs.Cert, error) {
	log.Lvlf2("CSRDispatcher(): Start")
	d.Certs = make([]*common_structs.Cert, 0)
	d.Data.ID = id // id of the site (hash of the genesis block)
	d.Data.Proposed = config
	d.Data.CAs = config.CAs
	log.Lvlf2("SignCert(): Start with %v certs", len(d.Certs))
	// Dispatch the CSR to all the listed CAs
	for _, ca := range d.CAs {
		public := ca.Public
		reply := &CSRReply{}
		cerr := d.CAClient.SendProtobuf(ca.ServerID, &CSR{ID: d.ID, Config: d.Proposed, PrevConfig: prevconfig}, reply)
		if cerr != nil {
			return nil, cerr
		}

		cert := reply.Cert

		// Verify that the chosen CA (having public key 'public') has properly signed the cert
		hash, _ := d.Proposed.Hash()

		err := crypto.VerifySchnorr(network.Suite, public, hash, *cert.Signature)
		if err != nil {
			log.Lvlf2("CA's signature doesn't verify (CA's public key: %v", public)
			return nil, errors.New("CA's signature doesn't verify")
		}

		d.Certs = append(d.Certs, cert)

	}
	log.Lvlf2("CSRDispatcher(): End: %v certs signed properly", len(d.Certs))
	return d.Certs, nil
}
