package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/dedis/kyber.v2/suites"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func NewTestClient(l *onet.LocalTest) *Client {
	return &Client{Client: l.NewClient(ServiceName)}
}

func TestServiceStatus(t *testing.T) {
	local := onet.NewTCPTest(tSuite)

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
