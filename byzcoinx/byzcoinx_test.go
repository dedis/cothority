package byzcoinx

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v4/cosuite"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/ciphersuite"
	"go.dedis.ch/onet/v4/log"
)

var defaultTimeout = 20 * time.Second
var testSuite = cosuite.NewBdnSuite()

func makeBuilder() onet.Builder {
	builder := onet.NewLocalBuilder(onet.NewDefaultBuilder())
	builder.SetSuite(testSuite)
	return builder
}

type Counter struct {
	veriCount   int
	refuseIndex int
	sync.Mutex
}

type Counters struct {
	counters []*Counter
	sync.Mutex
}

func (co *Counters) add(c *Counter) {
	co.Lock()
	co.counters = append(co.counters, c)
	co.Unlock()
}

func (co *Counters) size() int {
	co.Lock()
	defer co.Unlock()
	return len(co.counters)
}

func (co *Counters) get(i int) *Counter {
	co.Lock()
	defer co.Unlock()
	return co.counters[i]
}

var counters = &Counters{}

// verify function that returns true if the length of the data is 1.
func verify(msg, data []byte) bool {
	c, err := strconv.Atoi(string(msg))
	if err != nil {
		log.Error("Failed to cast msg", msg)
		return false
	}

	if len(data) == 0 {
		log.Error("Data is empty.")
		return false
	}

	counter := counters.get(c)
	counter.Lock()
	counter.veriCount++
	log.Lvl4("Verification called", counter.veriCount, "times")
	counter.Unlock()
	if len(msg) == 0 {
		log.Error("Didn't receive correct data")
		return false
	}
	return true
}

// verifyRefuse will refuse the refuseIndex'th calls
func verifyRefuse(msg, data []byte) bool {
	c, err := strconv.Atoi(string(msg))
	if err != nil {
		log.Error("Failed to cast", msg)
		return false
	}

	counter := counters.get(c)
	counter.Lock()
	defer counter.Unlock()
	defer func() { counter.veriCount++ }()
	if counter.veriCount == counter.refuseIndex {
		log.Lvl2("Refusing for count==", counter.refuseIndex)
		return false
	}
	log.Lvl3("Verification called", counter.veriCount, "times")
	if len(msg) == 0 {
		log.Error("Didn't receive correct data")
		return false
	}
	return true
}

// ack is a dummy
func ack(a, b []byte) bool {
	return true
}

func TestMain(m *testing.M) {
	flag.Parse()
	log.MainTest(m)
}

func TestBftCoSi(t *testing.T) {
	const protoName = "TestBftCoSi"

	err := GlobalInitBFTCoSiProtocol(testSuite, verify, ack, protoName)
	require.Nil(t, err)

	for _, n := range []int{1, 2, 4, 9, 20} {
		runProtocol(t, n, 0, 0, protoName)
	}
}

func TestBftCoSiRefuse(t *testing.T) {
	const protoName = "TestBftCoSiRefuse"

	err := GlobalInitBFTCoSiProtocol(testSuite, verifyRefuse, ack, protoName)
	require.Nil(t, err)

	// the refuseIndex has both leaf and sub leader failure
	configs := []struct{ n, f, r int }{
		{4, 0, 3},
		{4, 0, 1},
		{9, 0, 9},
		{9, 0, 1},
	}
	for _, c := range configs {
		runProtocol(t, c.n, c.f, c.r, protoName)
	}
}

func TestBftCoSiFault(t *testing.T) {
	const protoName = "TestBftCoSiFault"

	err := GlobalInitBFTCoSiProtocol(testSuite, verify, ack, protoName)
	require.Nil(t, err)

	configs := []struct{ n, f, r int }{
		{4, 1, 0},
		{9, 2, 0},
		{10, 3, 0},
	}
	for _, c := range configs {
		runProtocol(t, c.n, c.f, c.r, protoName)
	}
}

func runProtocol(t *testing.T, nbrHosts int, nbrFault int, refuseIndex int, protoName string) {
	log.Lvlf1("Starting with %d hosts with %d faulty ones and refusing at %d. Protocol name is %s",
		nbrHosts, nbrFault, refuseIndex, protoName)
	local := onet.NewLocalTest(makeBuilder())
	defer local.CloseAll()

	servers, roster, tree := local.GenTree(nbrHosts, false)
	require.NotNil(t, roster)

	pi, err := local.CreateProtocol(protoName, tree)
	require.Nil(t, err)

	bftCosiProto := pi.(*ByzCoinX)
	bftCosiProto.CreateProtocol = local.CreateProtocol
	bftCosiProto.FinalSignatureChan = make(chan FinalSignature, 1)

	counter := &Counter{refuseIndex: refuseIndex}
	counters.add(counter)
	proposal := []byte(strconv.Itoa(counters.size() - 1))
	bftCosiProto.Msg = proposal
	bftCosiProto.Data = []byte("hello world")
	bftCosiProto.Timeout = defaultTimeout
	bftCosiProto.Threshold = nbrHosts - nbrFault
	if refuseIndex > 0 {
		bftCosiProto.Threshold--
	}
	log.Lvl3("Added counter", counters.size()-1, refuseIndex)

	require.Equal(t, bftCosiProto.nSubtrees, int(math.Pow(float64(nbrHosts), 1.0/3.0)))

	// kill the leafs first
	nbrFault = min(nbrFault, len(servers))
	for i := len(servers) - 1; i > len(servers)-nbrFault-1; i-- {
		log.Lvl3("Pausing server:", servers[i].ServerIdentity.PublicKey, servers[i].Address())
		servers[i].Pause()
	}

	err = bftCosiProto.Start()
	require.Nil(t, err)

	// verify signature
	err = getAndVerifySignature(bftCosiProto.FinalSignatureChan, bftCosiProto.PublicKeys(), proposal)
	require.Nil(t, err)

	// check the counters
	counter.Lock()
	defer counter.Unlock()

	// We use <= because the verification function may be called more than
	// once on the same node if a sub-leader in ftcosi fails and the tree is
	// re-generated.
	require.True(t, nbrHosts-nbrFault <= counter.veriCount)
}

func getAndVerifySignature(sigChan chan FinalSignature, publics []ciphersuite.PublicKey, proposal []byte) error {
	var sig FinalSignature
	timeout := defaultTimeout + time.Second
	select {
	case sig = <-sigChan:
	case <-time.After(timeout):
		return fmt.Errorf("didn't get commitment after a timeout of %v", timeout)
	}

	// verify signature
	if sig.Sig == nil {
		return fmt.Errorf("signature is nil")
	}
	if bytes.Compare(sig.Msg, proposal) != 0 {
		return fmt.Errorf("message in the signature is different from proposal")
	}
	aggKey, err := testSuite.AggregatePublicKeys(publics, sig.Sig)
	if err != nil {
		return err
	}
	err = testSuite.Verify(aggKey, sig.Sig, proposal)
	if err != nil {
		return fmt.Errorf("didn't get a valid signature: %s", err)
	}
	log.Lvl2("Signature correctly verified!")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
