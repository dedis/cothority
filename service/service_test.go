package service

import (
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onchain-secrets"
	"github.com/dedis/onchain-secrets/darc"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m, 2)
}

func TestService_proof(t *testing.T) {
	local := onet.NewTCPTest(tSuite)
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, roster, _ := local.GenTree(5, true)
	defer local.CloseAll()

	services := local.GetServices(hosts, templateID)
	service := services[0].(*Service)

	// Initializing onchain-secrets skipchain
	writer := darc.NewEd25519Signer(nil, nil)
	writerS := &darc.Signer{Ed25519: writer}
	writerI, err := darc.NewIdentity(nil, darc.NewEd25519Identity(writer.Point))
	log.ErrFatal(err)
	readers := darc.NewDarc(nil, nil, nil)
	readers.AddOwner(writerI)
	readers.AddUser(writerI)
	sc, err := service.CreateSkipchains(&ocs.CreateSkipchainsRequest{
		Roster:  *roster,
		Writers: *readers,
	})
	log.ErrFatal(err)

	// Creating a write request
	encKey := []byte{1, 2, 3}
	write := ocs.NewWrite(cothority.Suite, sc.OCS.Hash, sc.X, readers, encKey)
	write.Data = []byte{}
	sigPath := darc.NewSignaturePath([]*darc.Darc{readers}, *writerI, darc.User)
	sig, err := darc.NewDarcSignature(write.Reader.GetID(), sigPath, writerS)
	log.ErrFatal(err)
	wr, err := service.WriteRequest(&ocs.WriteRequest{
		OCS:       sc.OCS.Hash,
		Write:     *write,
		Signature: *sig,
		Readers:   readers,
	})
	log.ErrFatal(err)

	// Making a read request
	sigRead, err := darc.NewDarcSignature(wr.SB.Hash, sigPath, writerS)
	log.ErrFatal(err)
	read := ocs.Read{
		DataID:    wr.SB.Hash,
		Signature: *sigRead,
	}
	rr, err := service.ReadRequest(&ocs.ReadRequest{
		OCS:  sc.OCS.Hash,
		Read: read,
	})
	log.ErrFatal(err)

	// Decoding the file
	symEnc, err := service.DecryptKeyRequest(&ocs.DecryptKeyRequest{
		Read: rr.SB.Hash,
	})
	log.ErrFatal(err)
	sym, err2 := ocs.DecodeKey(cothority.Suite, sc.X, write.Cs, symEnc.XhatEnc, writer.Secret)
	log.ErrFatal(err2)
	require.Equal(t, encKey, sym)

	// GetReadRequests
	requests, err := service.GetReadRequests(&ocs.GetReadRequests{
		Start: wr.SB.Hash,
		Count: 0,
	})
	log.ErrFatal(err)
	require.Equal(t, 1, len(requests.Documents))
}
