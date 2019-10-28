package status

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.dedis.ch/cothority/v4/cosuite"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
)

var testSuite = cosuite.NewBdnSuite()

func makeBuilder() *onet.DefaultBuilder {
	builder := onet.NewDefaultBuilder()
	builder.SetSuite(testSuite)
	builder.SetService(ServiceName, nil, newStatService)
	return builder
}

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func NewTestClient(l *onet.LocalTest) *Client {
	return &Client{Client: l.NewClient(ServiceName)}
}

// Sets up a set of nodes and checks the connectivity. Then pauses one node, and
// makes sure that the connectivity test fails, and that the `findFaulty` finds
// the paused node.
func TestStat_Connectivity(t *testing.T) {
	local := onet.NewLocalTest(onet.NewLocalBuilder(makeBuilder()))

	servers, ro, _ := local.GenTree(5, false)
	defer local.CloseAll()

	// Send a request to the service
	cl := NewClient(testSuite)
	sk := servers[0].ServerIdentity.GetPrivate()

	repl, err := cl.CheckConnectivity(sk, ro.List, time.Second, false)
	require.NoError(t, err)
	require.Equal(t, len(ro.List), len(repl))
	for i := range ro.List {
		require.True(t, ro.List[i].Equal(repl[i]))
	}
	require.NoError(t, local.WaitDone(time.Second))

	servers[2].Pause()
	repl, err = cl.CheckConnectivity(sk, ro.List, time.Second, false)
	require.Error(t, err)

	repl, err = cl.CheckConnectivity(sk, ro.List, time.Second, true)
	require.NoError(t, err)
	for i := range append(ro.List[0:2], ro.List[3:]...) {
		require.True(t, ro.List[i].Equal(repl[i]))
	}
	local.Check = onet.CheckNone
}

func TestStat_Request(t *testing.T) {
	local := onet.NewLocalTest(makeBuilder())

	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, _ := local.GenTree(5, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewTestClient(local)
	log.Lvl1("Sending request to service...")
	stat, err := client.Request(el.List[0])
	log.ErrFatal(err)
	log.Lvl1(stat)
	assert.NotEmpty(t, stat.Status["Generic"].Field["Available_Services"])
}
