// timestamp contains a simplified timestamp server. It collects statements from
// clients, waits EpochDuration time and responds with a signature of the
// requested data.
package timestamp

import (
	"fmt"
	"time"

	"crypto/sha256"
	"os"
	//	"path/filepath"
	"sync"

	"encoding/binary"
	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/swupdate"
	"github.com/dedis/cothority/sda"
)

// ServiceName can be used to refer to the name of the timestamp service
const ServiceName = "Timestamp"

// TODO make this a config parameter:
const EpochDuration = (time.Second * 10)

const groupFileName = "group.toml"

var timestampSID sda.ServiceID

var dummyVerfier = func(data []byte) bool {
	log.Print("Got time", string(data))
	return true
}

func init() {
	sda.RegisterNewService(ServiceName, newTimestampService)
	timestampSID = sda.ServiceFactory.ServiceID(ServiceName)
	network.RegisterPacketType(&SignatureRequest{})
	network.RegisterPacketType(&SignatureResponse{})
}

type Service struct {
	*sda.ServiceProcessor
	// Epoch is is the time that needs to pass until
	// the timestamp service attempts to collectively sign the batches
	// of statements collected. Reasonable choices would be from 10 seconds
	// upto some hours.
	EpochDuration time.Duration

	// config path for service:
	path string
	// collected data for one epoch:
	requests requestPool
	roster   *sda.Roster
	// easy to change from one signer (cosi) to another (mock/BFTcosi):
	signMsg func(m []byte) []byte
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (s *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	log.Lvl2("Timestamp Service received New Protocol event")
	var pi sda.ProtocolInstance
	var err error
	// TODO does this work? Maybe each node should have a unique protocol
	// name instead
	sda.ProtocolRegisterName("UpdateCosi", func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		// XXX for now we provide a dummy verification function. It
		// just prints out the timestamp, received in the Announcement.
		return swupdate.NewCoSiUpdate(n, dummyVerfier)
	})
	return pi, err
}

// SignatureRequest will be requested by clients.
type SignatureRequest struct {
	// Message should be a hashed nonce for the timestamp server.
	Message []byte
	// Different requests will be signed by the same roster
	// Hence, it doesn't make sense for every client to send his Roster
	// Roster  *sda.Roster
}

// SignatureResponse is what the Cosi service will reply to clients.
type SignatureResponse struct {
	// The time in seconds when the request was started:
	Timestamp int64
	// The tree root that was signed:
	Root crypto.HashID
	// Proof is an Inclusion proof for the data the client requested:
	Proof crypto.Proof
	// Collective signature on Timestamp||hash(treeroot):
	Signature []byte

	// TODO should we return the roster used to sign this message?
}

// SignatureRequest treats external request to this service.
func (s *Service) SignatureRequest(si *network.ServerIdentity, req *SignatureRequest) (network.Body, error) {

	// on every request:
	// 1) If has the length of hashed nonce, add it to the local buffer of
	//    of the service:
	respC := make(chan *SignatureResponse)
	s.requests.Add(req.Message, respC)
	// 2) At epoch time: create the merkle tree
	// see runLoop
	// 3) run *one* cosi round on treeroot||timestamp
	// see runLoop
	// 4) return to each client: above signature, (inclusion) Proof, timestamp
	// see runLoop

	// wait on my signature:
	log.Print("Waiting on epoch end.")
	resp := <-respC
	return resp, nil
}

func (s *Service) cosiSign(msg []byte) []byte {
	log.Print("Service?", s)
	log.Print("Roster?", s.roster)
	sdaTree := s.roster.GenerateBinaryTree()

	tni := s.NewTreeNodeInstance(sdaTree, sdaTree.Root, swupdate.ProtcolName)
	pi, err := swupdate.NewCoSiUpdate(tni, dummyVerfier)
	if err != nil {
		panic("Couldn't make new protocol: " + err.Error())
	}
	s.RegisterProtocolInstance(pi)

	pi.SigningMessage(msg)
	// Take the raw message (already expecting a hash for the timestamp
	// service)
	response := make(chan []byte)
	pi.RegisterSignatureHook(func(sig []byte) {
		response <- sig
	})
	go pi.Dispatch()
	go pi.Start()
	return <-response

}

// main loop
func (s *Service) runLoop() {
	c := time.Tick(s.EpochDuration)
	for now := range c /*TODO interrupt the main loop must be possible*/ {
		// only sign something if there was some data/requests:
		numRequests := len(s.requests.GetData())
		if numRequests > 0 {
			log.Print("Signin tree root with timestampt:", now, "got", numRequests, "requests")

			// create merkle tree and message to be signed:
			root, proofs := crypto.ProofTree(sha256.New, s.requests.GetData())
			timeBuf := timestampToBytes(now.Unix())
			// message to be signed: treeroot||timestamp
			msg := append(root, timeBuf...)

			signature := s.signMsg(msg)
			fmt.Printf("%s: Signed a message.\n", time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"))
			// Give (individual) response to anyone waiting:
			for i, respC := range s.requests.responseChannels {
				respC <- &SignatureResponse{
					Timestamp: now.Unix(),
					Proof:     proofs[i],
					Root:      root,
					// Collective signature on Timestamp||hash(treeroot)
					Signature: signature,
				}
			}
			s.requests.reset()
		} else {
			log.Print("No requests at epoch:", time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"))
		}
	}

}

func timestampToBytes(t int64) []byte {
	timeBuf := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(timeBuf, t)
	return timeBuf
}
func newTimestampService(c *sda.Context, path string) sda.Service {
	// r, err := readRoster(filepath.Join(path, groupFileName))
	//if err != nil {
	//	log.ErrFatal(err,
	//		"Couldn't read roster from config. Put a valid roster definition in",
	//		filepath.Join(path, groupFileName))
	//}
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
		requests:         requestPool{},
		EpochDuration:    EpochDuration,
		//	roster:           r,
	}
	s.signMsg = s.cosiSign
	err := s.RegisterMessage(s.SignatureRequest)
	if err != nil {
		log.ErrFatal(err, "Couldn't register message:")
	}

	// start main loop:
	go s.runLoop()

	return s
}

type tree struct {
	proofs []crypto.Proof
	root   crypto.HashID
}

type requestPool struct {
	sync.Mutex
	requestData      []crypto.HashID
	responseChannels []chan *SignatureResponse
}

func (rb *requestPool) reset() {
	rb.Lock()
	defer rb.Unlock()
	rb.requestData = nil
	// XXX do we need to close each channel separately?
	rb.responseChannels = nil
	log.Print("Reset")
}

func (rb *requestPool) Add(data []byte, responseChan chan *SignatureResponse) {
	rb.Lock()
	defer rb.Unlock()
	rb.requestData = append(rb.requestData, data)
	log.Print("Added request", len(rb.requestData))
	rb.responseChannels = append(rb.responseChannels, responseChan)
}

func (rb *requestPool) GetData() []crypto.HashID {
	rb.Lock()
	defer rb.Unlock()
	return rb.requestData
}

// XXX this should probably be in some independent util package instead
func readRoster(tomlFile string) (*sda.Roster, error) {
	f, err := os.Open(tomlFile)
	if err != nil {
		return nil, err
	}
	el, err := config.ReadGroupToml(f)
	if err != nil {
		return nil, err
	}
	return el, nil
}
