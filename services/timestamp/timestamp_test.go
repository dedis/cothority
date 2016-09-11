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
		signMsg: mockSign,
	}
	go s.runLoop()
	N := 2
	// send 3 consecutive "requests":
	for i := 0; i < N; i++ {
		s.requests.Add([]byte("random hashed data"+strconv.Itoa(i)), make(chan *SignatureResponse))
	}

	// wait on all responses
	log.Print("Waiting for repsonses ...")
	for i, respC := range s.requests.responseChannels {
		resp := <-respC
		//log.Print("Got response")
		timeBuf := timestampToBytes(resp.Timestamp)
		// this is data we sent:
		leaf := []byte("random hashed data" + strconv.Itoa(i))
		// msg = treeroot||timestamp
		msg := append(resp.Root, timeBuf...)
		assert.True(t, ed25519.Verify(pk, msg, resp.Signature), "Wrong signature")

		// Verify the inclusion proof:
		assert.True(t, resp.Proof.Check(sha256.New, resp.Root, leaf), "Wrong inclusion proof for "+string(i))
	}
	log.Print("Done one round. TODO shut down main loop (otherwise it leaks).")
}

func TestTimestampRunLoopSDA(t *testing.T) {
	if testing.Short() {
		t.Skip("Running the Timestamp service using an epoch & networking takes too long.")
	}
	defer log.AfterTest(t)
	log.TestOutput(false, 1)
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, localRoster, _ := local.GenTree(5, false, true, false)
	defer local.CloseAll()

	// We need two different client instances, otherwise the service will
	// block on the first (timestamp)request:
	c0 := NewClient()
	c1 := NewClient()
	c2 := NewClient()
	//msg := "random hashed data"
	log.Lvl1("Sending request to service...")
	rootIdentity := localRoster.Get(0)
	_, err := c0.SetupStamper(rootIdentity, localRoster, time.Millisecond*250)
	log.ErrFatal(err, "Coulnd't init roster")
	log.Print("Setup done ...")
	res1 := make(chan *SignatureResponse)
	res2 := make(chan *SignatureResponse)
	origMsg1 := []byte("random hashed data" + strconv.Itoa(0))
	go func() {
		log.Print("Sending first request ...")
		res, err := c1.SignMsg(rootIdentity, origMsg1)
		log.ErrFatal(err, "Couldn't send")
		log.LLvl3("First request sent and received a response.")
		res1 <- res
	}()
	time.Sleep(time.Millisecond * 10)
	origMsg2 := []byte("random hashed data" + strconv.Itoa(1))
	go func() {
		log.Print("Sending second request ...")
		res, err := c2.SignMsg(rootIdentity, origMsg2)
		log.ErrFatal(err, "Couldn't send")
		log.LLvl3("Second request sent and received a response.")
		res2 <- res
	}()
	assert.NotEqual(t, origMsg1, origMsg2)

	time.Sleep(time.Millisecond * 100)
	log.LLvl1("Waiting on responses ...")
	resp1 := <-res1
	resp2 := <-res2
	log.LLvl1("... done waiting on responses.")
	assert.Equal(t, resp1.Timestamp, resp2.Timestamp)
	assert.Equal(t, resp1.Root, resp2.Root)

	timeBuf1 := timestampToBytes(resp1.Timestamp)
	timeBuf2 := timestampToBytes(resp2.Timestamp)

	signedMsg1 := append(resp1.Root, timeBuf1...)
	signedMsg2 := append(resp2.Root, timeBuf2...)

	// verify signatures:
	var publics []abstract.Point
	for _, e := range localRoster.List {
		publics = append(publics, e.Public)
	}
	assert.NoError(t, swupdate.VerifySignature(network.Suite, publics, signedMsg1, resp1.Signature))
	assert.NoError(t, swupdate.VerifySignature(network.Suite, publics, signedMsg2, resp2.Signature))

	// check if proofs are what we expect:
	root, proofs := crypto.ProofTree(sha256.New, []crypto.HashID{origMsg1, origMsg2})
	assert.True(t, proofs[0].Check(sha256.New, root, origMsg1))
	assert.True(t, proofs[1].Check(sha256.New, root, origMsg2))

	// FIXME the proofs don't survive sending them them via SDA ?!
	assert.Equal(t, proofs[0], resp1.Proof)
	assert.Equal(t, proofs[1], resp2.Proof)

	// verify inclusion proofs (fix above problem first):
	assert.True(t, resp1.Proof.Check(sha256.New, resp1.Root,
		[]byte("random hashed data"+strconv.Itoa(0))),
		"Wrong inclusion proof for msg1")
	assert.True(t, resp2.Proof.Check(sha256.New, resp2.Root,
		[]byte("random hashed data"+strconv.Itoa(1))),
		"Wrong inclusion proof for msg2")
}
