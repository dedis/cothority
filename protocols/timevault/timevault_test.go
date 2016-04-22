package timevault_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/timevault"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/random"
)

func ElGamalEncrypt(suite abstract.Suite, msg []byte, key abstract.Point) (abstract.Point, abstract.Point) {
	kp := config.NewKeyPair(suite)
	m, _ := suite.Point().Pick(msg, random.Stream) // can take at most 29 bytes in one step
	c := suite.Point().Add(m, suite.Point().Mul(key, kp.Secret))
	return c, kp.Public
}

func ElGamalDecrypt(suite abstract.Suite, c abstract.Point, key abstract.Point) ([]byte, error) {
	return suite.Point().Sub(c, key).Data()
}

func TestTimeVault(t *testing.T) {

	// Setup parameters
	var name string = "TimeVault" // Protocol name
	var nodes uint32 = 5          // Number of nodes
	msg := []byte("Hello World!") // Message to-be-sealed

	local := sda.NewLocalTest()
	_, _, tree := local.GenTree(int(nodes), false, true, true)
	defer local.CloseAll()

	dbg.TestOutput(testing.Verbose(), 1)

	dbg.Lvl1("TimeVault - starting")
	leader, err := local.CreateProtocol(name, tree)
	if err != nil {
		t.Fatal("Couldn't initialise protocol tree:", err)
	}
	tv := leader.(*timevault.TimeVault)
	leader.Start()
	dbg.Lvl1("TimeVault - setup done")

	sid, key, err := tv.Seal(time.Second * 2)
	if err != nil {
		dbg.Fatal(err)
	}
	dbg.Lvl1("TimeVault - sealing done")

	// Do ElGamal encryption
	c, eKey := ElGamalEncrypt(tv.Suite(), msg, key)

	<-time.After(time.Second * 5)

	// Now we should be able to open the secret and decrypt the ciphertext
	x, err := tv.Open(sid)
	if err != nil {
		dbg.Fatal(err)
	}
	X := tv.Suite().Point().Mul(eKey, x)
	m, err := ElGamalDecrypt(tv.Suite(), c, X)
	if err != nil {
		dbg.Fatal(err)
	}
	if !bytes.Equal(m, msg) {
		dbg.Fatal("Error, decryption failed")
	}
	dbg.Lvl1("TimeVault - decryption successful")
}
