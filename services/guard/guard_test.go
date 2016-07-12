package guard

import (
	"testing"

	"fmt"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/assert"
)

func TestServiceGuard(t *testing.T) {
	defer log.AfterTest(t)
	log.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, _ := local.GenTree(5, false, true, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewClient()
	log.Lvl1("Sending request to service...")
	UID := []byte{"USER"}
	Epoch := []byte{"EPOCH"}
	msg := []byte{"Hello"}
	Hzi, _ := client.GetGuard(el.List[0], UID, Epoch, msg)

	Hz2, _ := client.GetGuard(el.List[0], UID, Epoch, msg)
	fmt.Println(Hzi)
	fmt.Println(Hz2)

	assert.Equal(t, Hzi, Hz2)

}
