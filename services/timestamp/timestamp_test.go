package timestamp

import (
	"crypto/sha256"
	"github.com/dedis/cothority/log"
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
	// test if main loop behaves as expected (without sda or network):
	s := &Service{
		requests:      requestPool{},
		EpochDuration: time.Millisecond * 3,
		signMsg:       mockSign,
	}
	go s.runLoop()

	// send 3 consecutive "requests":
	for i := 0; i < 3; i++ {
		s.requests.Add([]byte("random hashed data"+strconv.Itoa(i)), make(chan *SignatureResponse))
	}

	// wait on all responses
	log.Print("Waiting for repsonses ...")
	for i, respC := range s.requests.responseChannels {
		resp := <-respC
		//log.Print("Got repsonse")
		timeBuf := timestampToBytes(resp.Timestamp)
		leaf := []byte("random hashed data" + strconv.Itoa(i))
		msg := append(resp.Root, timeBuf...)
		if !ed25519.Verify(pk, msg, resp.Signature) {
			t.Error("Wrong signature")
		}
		assert.True(t, resp.Proof.Check(sha256.New, resp.Root, leaf), "Wrong inclusion proof for "+string(i))
		// TODO check for max expected time difference
		//got := time.Time(resp.Timestamp)
		//mine := time.Now()
	}
	log.Print("Done one round. TODO shut down main loop.")
}

func TestRunLoopSDA(t *testing.T) {
	if testing.Short() {
		t.Skip("Running the Timestamp service using an epoch & networking takes too long.")
	}
	// TODO either write such a test or delete this method
}
