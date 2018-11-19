// Package service implements a blsftcosi service for which clients can connect to
// and then sign messages.
package service

import (
	"encoding/hex"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/kyber/pairing/bn256"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/student_18_blsftcosi/blsftcosi/protocol"
)

// This file contains all the code to run a CoSi service. It is used to reply to
// client request for signing something using CoSi.
// As a prototype, it just signs and returns. It would be very easy to write an
// updated version that chains all signatures for example.

const propagationTimeout = 20 * time.Second
const protocolTimeout = 40 * time.Second

// ServiceName is the name to refer to the CoSi service
var ServiceID onet.ServiceID

const ServiceName = "blsftCoSiService"

func init() {
	ServiceID, _ = onet.RegisterNewService(ServiceName, newCoSiService)
	network.RegisterMessage(&SignatureRequest{})
	network.RegisterMessage(&SignatureResponse{})
}

// Service is the service that handles collective signing operations
type Service struct {
	*onet.ServiceProcessor
	suite             cosi.Suite
	pairingSuite      pairing.Suite
	private           kyber.Scalar
	public            kyber.Point
	pairingPublicKeys []kyber.Point
	wg                sync.WaitGroup
	Threshold         int
	NSubtrees         int
}

// SignatureRequest is what the Cosi service is expected to receive from clients.
type SignatureRequest struct {
	Message []byte
	Roster  *onet.Roster
}

// SignatureResponse is what the Cosi service will reply to clients.
type SignatureResponse struct {
	Hash      []byte
	Signature []byte
}

// SignatureRequest treats external request to this service.
func (s *Service) SignatureRequest(req *SignatureRequest) (network.Message, error) {
	// generate the tree
	nNodes := len(req.Roster.List)
	rooted := req.Roster.NewRosterWithRoot(s.ServerIdentity())
	if rooted == nil {
		return nil, errors.New("we're not in the roster")
	}
	tree := rooted.GenerateNaryTree(nNodes)
	if tree == nil {
		return nil, errors.New("failed to generate tree")
	}

	// configure the BlsFtCosi protocol
	pi, err := s.CreateProtocol(protocol.DefaultProtocolName, tree)
	if err != nil {
		return nil, errors.New("Couldn't make new protocol: " + err.Error())
	}
	p := pi.(*protocol.BlsFtCosi)
	p.CreateProtocol = s.CreateProtocol
	p.Msg = req.Message
	// We set NSubtrees to the square root of n to evenly distribute the load
	if s.NSubtrees == 0 {
		p.NSubtrees = int(math.Sqrt(float64(nNodes)))
	} else {
		p.NSubtrees = s.NSubtrees
	}
	if p.NSubtrees < 1 {
		p.NSubtrees = 1
	}
	p.Timeout = protocolTimeout

	// Complete Threshold
	p.Threshold = s.Threshold

	// Set the pairing keys
	p.PairingPrivate = s.private
	p.PairingPublic = s.public
	s.wg.Wait()
	p.PairingPublics = s.pairingPublicKeys

	// start the protocol
	log.Lvl3("Cosi Service starting up root protocol")
	if err = pi.Start(); err != nil {
		return nil, err
	}

	if log.DebugVisible() > 1 {
		log.Printf("%s: Signed a message.\n", time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"))
	}

	// wait for reply
	var sig []byte
	select {
	case sig = <-p.FinalSignature:
	case <-time.After(p.Timeout + time.Second):
		return nil, errors.New("protocol timed out")
	}

	// The hash is the message ftcosi actually signs, we recompute it the
	// same way as ftcosi and then return it.
	h := s.suite.Hash()
	h.Write(req.Message)
	return &SignatureResponse{h.Sum(nil), sig}, nil
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (s *Service) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3("Cosi Service received on", s.ServerIdentity(), "received new protocol event-", tn.ProtocolName())
	switch tn.ProtocolName() {
	case protocol.DefaultProtocolName:
		log.Lvl3("IT SHOULD NEVER COME HERE")
		pi, err := protocol.NewDefaultProtocol(tn)
		if err != nil {
			return nil, err
		}
		blsftcosi := pi.(*protocol.BlsFtCosi)
		blsftcosi.PairingPrivate = s.private
		blsftcosi.PairingPublic = s.public
		s.wg.Wait()
		blsftcosi.PairingPublics = s.pairingPublicKeys
		return blsftcosi, nil
	case protocol.DefaultSubProtocolName:
		pi, err := protocol.NewDefaultSubProtocol(tn)
		if err != nil {
			return nil, err
		}
		subblsftcosi := pi.(*protocol.SubBlsFtCosi)
		subblsftcosi.PairingPrivate = s.private
		subblsftcosi.PairingPublic = s.public
		s.wg.Wait()
		subblsftcosi.PairingPublics = s.pairingPublicKeys
		return subblsftcosi, nil
	case protocol.DefaultKDProtocolName:
		pi, err := protocol.NewBlsKeyDist(tn)
		if err != nil {
			return nil, err
		}
		blskeydist := pi.(*protocol.BlsKeyDist)
		blskeydist.PairingPublic = s.public
		blskeydist.Timeout = propagationTimeout
		go s.getPublicKeys(blskeydist.PairingPublics)
		return blskeydist, nil
	}
	return nil, errors.New("no such protocol " + tn.ProtocolName())
}

func (s *Service) getPublicKeys(pairingPublics chan []kyber.Point) {
	s.pairingPublicKeys = <-pairingPublics
	s.wg.Done()
}

func (s *Service) GetPairingPublicKeys() []kyber.Point {
	return s.pairingPublicKeys
}

func newCoSiService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		suite:            cothority.Suite,
		pairingSuite:     bn256.NewSuite(),
	}
	s.wg.Add(1)

	if err := s.RegisterHandler(s.SignatureRequest); err != nil {
		log.Error("couldn't register message:", err)
		return nil, err
	}

	return s, nil
}

func (s *Service) SetPairingKeys(index int, hosts int, tree *onet.Tree) {

	/*
		// Generate bn256 keys for the service.
		private, public := bls.NewKeyPair(s.pairingSuite, random.New())
		s.private = private
		s.public = public

		// Go BlsKD on the nodes
		pi, err := s.CreateProtocol(protocol.DefaultKDProtocolName, tree)
		blskeydist := pi.(*protocol.BlsKeyDist)
		blskeydist.PairingPublic = s.public
		blskeydist.Timeout = propagationTimeout
		if err := pi.Start(); err != nil {
			return nil, err
		}
		log.Lvl3("Started BlsKG-protocol - waiting for done", len(req.Roster.List))

		go s.getPublicKeys(blskeydist.PairingPublics)
	*/

	// set keys using seed
	stream := deterstream{0}
	publicKeys := make([]kyber.Point, hosts)
	for i := 0; i < hosts; i++ {

		private := s.pairingSuite.G2().Scalar().Pick(&stream)
		publicKeys[i] = s.pairingSuite.G2().Point().Mul(private, nil)
		if i == index {
			s.private = private
			s.public = publicKeys[i]
		}
	}
	s.pairingPublicKeys = publicKeys
	s.wg.Done()
}

type deterstream struct {
	Index int
}

func (r *deterstream) XORKeyStream(dst, src []byte) {

	l := len(dst)
	if len(src) != l {
		panic("XORKeyStream: mismatched buffer lengths")
	}

	buf, _ := hex.DecodeString("4F0529EE7B8EAD6A20FBE1DBCB15F67C042FEAB1C0A00A583FF34BF7473E0587CE879087CF65E6B0B06BE5BFD43FA2747501BD7C05C39417558E460E6E3DA886D253CCCDF156E0905AB555BC6F0F7D7CA25A8C7F38BEB7B565E08CF20A7BC5425E90C3C9C8CC7105950129EA79DD5EEAFB1DA59A3F3DC0AB094FEA020561E0B40B03A6461DC2021ABDC9AD77D0AD6B07F6571EB4CDBA72EDBA6C4DAE5EF31E5C1D268ABFE7024DEAD733D8D23FC80D2C072F61719DC0BF6E66C4C46A49567669945EE24540318C63F41B5CDED3F5A77FE97F2A463B91EF20893FE75DFBD5FC02FAEDDC138FA1C8C7EED189BAAF301A4177CE3B556E18D7472E9BB0A127B7CA2E990185896E5B2CE8073663FD29DB9922413E458006506382E23BB7A0E5FE662227FE48B9BC3BD1D83D42E626E6A07E102640A1BAEDCE8B69615C30FCBD598BE95BC92CD2382D8CC36EC198211B3B0388C5586DD578AAC4C91A4B2CC9835FA13785D3A56284DCFDA3D5A3D0304A52A6ACA0703A4FF940FE18BD8BF6A83739D8A30351EEC661C25357C2C93A0D755E25379B4CD704256D1FE9E3AC9076E2C373BC45AF53823939708073B8250E723F7121EC20320E77BEDFA9CEE9D115A2155310DFFB87BD4B8219E41C217C09A293693928874F91A8F03645F15D7AF139A5C05D873F4F83253B94E206AA55BE23B982DEB9CA7B884F0529EE7B8EAD6A20FBE1DBCB15F67C042FEAB1C0A00A583FF34BF7473E0587CE879087CF65E6B0B06BE5BFD43FA2747501BD7C05C39417558E460E6E3DA886D253CCCDF156E0905AB555BC6F0F7D7CA25A8C7F38BEB7B565E08CF20A7BC5425E90C3C9C8CC7105950129EA79DD5EEAFB1DA59A3F3DC0AB094FEA020561E0B40B03A6461DC2021ABDC9AD77D0AD6B07F6571EB4CDBA72EDBA6C4DAE5EF31E5C1D268ABFE7024DEAD733D8D23FC80D2C072F61719DC0BF6E66C4C46A49567669945EE24540318C63F41B5CDED3F5A77FE97F2A463B91EF20893FE75DFBD5FC02FAEDDC138FA1C8C7EED189BAAF301A4177CE3B556E18D7472E9BB0A127B7CA2E990185896E5B2CE8073663FD29DB9922413E458006506382E23BB7A0E5FE662227FE48B9BC3BD1D83D42E626E6A07E102640A1BAEDCE8B69615C30FCBD598BE95BC92CD2382D8CC36EC198211B3B0388C5586DD578AAC4C91A4B2CC9835FA13785D3A56284DCFDA3D5A3D0304A52A6ACA0703A4FF940FE18BD8BF6A83739D8A30351EEC661C25357C2C93A0D755E25379B4CD704256D1FE9E3AC9076E2C373BC45AF53823939708073B8250E723F7121EC20320E77BEDFA9CEE9D115A2155310DFFB87BD4B8219E41C217C09A293693928874F91A8F03645F15D7AF139A5C05D873F4F83253B94E206AA55BE23B982DEB9CA7B88")

	for i := 0; i < l; i++ {
		dst[i] = src[i] ^ buf[(r.Index)%len(buf)]
		r.Index++
		if r.Index > len(buf) {
			r.Index -= len(buf)
		}
	}
}
