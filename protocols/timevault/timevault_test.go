package timevault_test

import (
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/timevault"
)

func TestTimeVault(t *testing.T) {

	// Setup parameters
	var name string = "TimeVault" // Protocol name
	var nodes uint32 = 5          // Number of nodes
	msg := []byte("Hello World!") // Message to-be-sealed

	local := sda.NewLocalTest()
	_, _, tree := local.GenTree(int(nodes), false, true, true)
	defer local.CloseAll()

	dbg.TestOutput(true, 2)

	dbg.Lvl1("TimeVault - starting")
	leader, err := local.CreateNewNodeName(name, tree)
	if err != nil {
		t.Fatal("Couldn't initialise protocol tree:", err)
	}
	tv := leader.ProtocolInstance().(*timevault.TimeVault)
	leader.StartProtocol()
	dbg.Lvl1("TimeVault - setup done")

	tv.Seal(msg, time.Second)

}
