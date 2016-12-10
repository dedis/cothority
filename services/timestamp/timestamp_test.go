package timestamp

import (
	"crypto/sha256"
	"strconv"
	"testing"
	"time"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/swupdate"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ed25519"
)

func TestRequestBuffer(t *testing.T) {
	rb := &requestPool{}
	for i := 0; i < 10; i++ {
		respC := make(chan *SignatureResponse)
		rb.Add([]byte("data_"+strconv.Itoa(i)), respC)
	}
	//fmt.Println(string(rb.requestData[0]))
	assert.Equal(t, len(rb.requestData), 10)
	rb.reset()
	assert.Equal(t, len(rb.requestData), 0)
}

var sk, pk []byte

func init() {
	pk, sk, _ = ed25519.GenerateKey(nil)
}

// mock the signing process to see if the main loop etc works fine (independent
// from sda etc)
func mockSign(m []byte) (signature []byte) {
	signature = ed25519.Sign(sk, m)
	return
}

func TestRunLoop(t *testing.T) {
	// test if main loop behaves as expected (without sda, cosi or network):
	s := &Service{
		requests:      requestPool{},
		EpochDuration: time.Millisecond * 3,
		// use ed25519 instead of cosi for a super quick test:
		signMsg:       mockSign,
		maxIterations: 1, // run iteration and quit
	}
	go s.runLoop()
	N := 10
	// send 3 consecutive "requests":
	for i := 0; i < N; i++ {
		s.requests.Add([]byte("random hashed data"+strconv.Itoa(i)), make(chan *SignatureResponse))
	}

	// wait on all responses
	log.Print("Waiting for repsonses ...")
	for i, respC := range s.requests.responseChannels {
		resp := <-respC
		// this is data we sent:
		leaf := []byte("random hashed data" + strconv.Itoa(i))
		// msg = treeroot||timestamp
		msg := RecreateSignedMsg(resp.Root, resp.Timestamp)

		assert.True(t, ed25519.Verify(pk, msg, resp.Signature),
			"Wrong signature")
		// Verify the inclusion proof:
		assert.True(t, resp.Proof.Check(sha256.New, resp.Root, leaf),
			"Wrong inclusion proof for "+string(i))
	}
	log.Print("Done one round.")
}

// run the whole framework (including network etc)
func TestTimestampRunLoopSDA(t *testing.T) {
	if testing.Short() {
		t.Skip("Running the Timestamp service using an epoch & networking takes too long.")
	}
	defer log.AfterTest(t)
	log.TestOutput(true, 1)
	local := sda.NewTCPTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, localRoster, _ := local.GenTree(5, true)
	defer local.CloseAll()

	// We need two different client instances, otherwise the service will
	// block on the first (timestamp)request:
	c0 := NewClient() // <- setup and run the stamper
	c1 := NewClient()
	c2 := NewClient()

	log.Lvl1("Sending request to service...")
	rootIdentity := localRoster.Get(0)
	numIterations := 1
	// init stamper and start it, too:
	_, err := c0.SetupStamper(localRoster, time.Millisecond*50, numIterations)

	log.ErrFatal(err, "Coulnd't init roster")
	log.Print("Setup done ...")
	res1 := make(chan *SignatureResponse)
	res2 := make(chan *SignatureResponse)
	origMsg1 := []byte("random hashed data" + strconv.Itoa(0))
	go func() {
		log.Print("Sending first request ...")
		res, err := c1.SignMsg(rootIdentity, origMsg1)
		log.ErrFatal(err, "Couldn't send")
		log.Lvl3("First request sent and received a response:", res.Proof)
		res1 <- res
	}()
	origMsg2 := []byte("random hashed data" + strconv.Itoa(1))
	go func() {
		log.Print("Sending second request ...")
		res, err := c2.SignMsg(rootIdentity, origMsg2)
		log.ErrFatal(err, "Couldn't send")
		log.Lvl3("Second request sent and received a response.")
		res2 <- res
	}()
	assert.NotEqual(t, origMsg1, origMsg2)

	log.Lvl1("Waiting on responses ...")
	resp1 := <-res1
	resp2 := <-res2
	log.Lvl1("... done waiting on responses.")

	// Do all kind of verifications:
	assert.Equal(t, resp1.Timestamp, resp2.Timestamp)
	assert.Equal(t, resp1.Root, resp2.Root)

	// re-create signed message:
	signedMsg1 := RecreateSignedMsg(resp1.Root, resp1.Timestamp)
	signedMsg2 := RecreateSignedMsg(resp2.Root, resp2.Timestamp)

	// verify signatures:
	var publics []abstract.Point
	for _, e := range localRoster.List {
		publics = append(publics, e.Public)
	}
	assert.NoError(t, swupdate.VerifySignature(network.Suite, publics,
		signedMsg1, resp1.Signature))
	assert.NoError(t, swupdate.VerifySignature(network.Suite, publics,
		signedMsg2, resp2.Signature))

	// check if proofs are what we expect:
	root, proofs := crypto.ProofTree(sha256.New, []crypto.HashID{origMsg1, origMsg2})
	assert.Equal(t, proofs[0], resp1.Proof)
	assert.Equal(t, proofs[1], resp2.Proof)
	assert.Equal(t, root, resp1.Root)
	assert.Equal(t, root, resp2.Root)

	// verify inclusion proofs (fix above problem first):
	log.Print("Received from channel:", resp1.Proof)
	assert.True(t, resp1.Proof.Check(sha256.New, resp1.Root, origMsg1),
		"Wrong inclusion proof for msg1")
	assert.True(t, resp2.Proof.Check(sha256.New, resp2.Root, origMsg2),
		"Wrong inclusion proof for msg2")
}
