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
	UID := []byte{'U', 'S', 'E', 'R'}
	Epoch := []byte{'E', 'P', 'O', 'C', 'H'}
	msg := []byte{'H', 'e', 'l', 'l', 'o'}
	Hzi, _ := client.GetGuard(el.List[0], UID, Epoch, msg)
	//lol1 := make([]string, len(Hzi))
	//for i := 0; i < len(lol1); i++ {
	//	lol1[i] = string(Hzi[i].Msg)
	//}
	Hz2, _ := client.GetGuard(el.List[0], UID, Epoch, msg)
	//lol2 := make([]string, len(Hz2))
	//
	//for i := 0; i < len(lol2); i++ {
	//	lol2[i] = string(Hz2[i].Msg)
	//}
	//secret1 := s.Combine(lol1)
	//secret2 := s.Combine(lol2)
	fmt.Println(Hzi)
	fmt.Println(Hz2)

	assert.Equal(t, Hzi, Hz2)

}
