package jvss

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestSID(t *testing.T) {
	sl := newSID(LTSS)
	ss := newSID(STSS)

	assert.True(t, sl.IsLTSS())
	assert.False(t, sl.IsSTSS())
	assert.NotEqual(t, sl, ss)

	assert.True(t, ss.IsSTSS())
	assert.False(t, ss.IsLTSS())
}

func TestJVSS(t *testing.T) {
	// Setup parameters
	var name string = "JVSS"      // Protocol name
	var nodes uint32 = 16         // Number of nodes
	var rounds int = 1            // Number of rounds
	msg := []byte("Hello World!") // Message to-be-signed

	local := sda.NewLocalTest()
	_, _, tree := local.GenTree(int(nodes), true)

	defer local.CloseAll()

	log.Lvl1("JVSS - starting")
	leader, err := local.CreateProtocol(name, tree)
	if err != nil {
		t.Fatal("Couldn't initialise protocol tree:", err)
	}
	jv := leader.(*JVSS)
	leader.Start()
	log.Lvl1("JVSS - setup done")

	for i := 0; i < rounds; i++ {
		log.Lvl1("JVSS - starting round", i)
		log.Lvl1("JVSS - requesting signature")
		sig, err := jv.Sign(msg)
		if err != nil {
			t.Fatal("Error signature failed", err)
		}
		log.Lvl1("JVSS - signature received")
		err = jv.Verify(msg, sig)
		if err != nil {
			t.Fatal("Error signature verification failed", err)
		}
		log.Lvl1("JVSS - signature verification succeded")
	}

}
