package timevault_test

import (
	"bytes"
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
	leader, err := local.CreateProtocol(name, tree)
	if err != nil {
		t.Fatal("Couldn't initialise protocol tree:", err)
	}
	tv := leader.(*timevault.TimeVault)
	leader.Start()
	dbg.Lvl1("TimeVault - setup done")

	sid, key, c, err := tv.Seal(msg, time.Second*2)
	if err != nil {
		dbg.Fatal(err)
	}

	// This should fail because the timer has not yet expired
	m, err := tv.Open(sid, key, c)
	if err != nil {
		dbg.Lvl2(err)
	}

	<-time.After(time.Second * 5)

	// Now we should be able to open the secret and decrypt the ciphertext
	m, err = tv.Open(sid, key, c)
	if err != nil {
		dbg.Lvl2(err)
	}
	if !bytes.Equal(m, msg) {
		dbg.Fatal("Error, decryption failed")
	}
}
