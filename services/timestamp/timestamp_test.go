package timestamp

import (
	"crypto/sha256"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ed25519"
	"strconv"
	"testing"
	"time"
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
	N := 5
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
		if !ed25519.Verify(pk, msg, resp.Signature) {
			t.Error("Wrong signature")
		}
		// Verify the inclusion proof:
		assert.True(t, resp.Proof.Check(sha256.New, resp.Root, leaf), "Wrong inclusion proof for "+string(i))
		// TODO check for max expected time difference?
		//got := time.Time(resp.Timestamp)
		//mine := time.Now()
	}
	log.Print("Done one round. TODO shut down main loop (otherwise it leaks).")
}

func TestTimestampRunLoopSDA(t *testing.T) {
	if testing.Short() {
		t.Skip("Running the Timestamp service using an epoch & networking takes too long.")
	}
	defer log.AfterTest(t)
	log.TestOutput(testing.Verbose(), 2)
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, _ := local.GenTree(5, true, true, true)
	defer local.CloseAll()

	// Send a request to the service
	client := NewClient()
	msg := "hello cosi service"
	log.Lvl1("Sending request to service...")
	rootIdentity := el.Get(0)
	res1 := make(chan *SignatureResponse)
	res2 := make(chan *SignatureResponse)
	// FIXME SDA blocks on the first request ...
	//
	// If service is blocking on client requests:
	// It isn't possible to write a simple timestamper without either
	// rewriting the Processor/Dispatcher layer to be non-blocking or (not
	// sure if this works) the service returns with some dummy ACK msg and
	// buffers each request together with its connection and
	go func() {
		log.Print("Sending first request:")
		res, err := client.SignMsg(rootIdentity, []byte(msg+strconv.Itoa(0)))
		log.ErrFatal(err, "Couldn't send")
		log.LLvl1("First request sent.")
		res1 <- res
	}()
	// You will never see the second request in the same epoch
	go func() {
		log.Print("Sending second request:")
		res, err := client.SignMsg(rootIdentity, []byte(msg+strconv.Itoa(1)))
		log.ErrFatal(err, "Couldn't send")
		log.LLvl1("Second request sent.")
		res2 <- res
	}()
	log.LLvl1("Waiting on responses ...")
	<-res1
	<-res2
	log.LLvl1("... done")
	// TODO verify the responses
}
