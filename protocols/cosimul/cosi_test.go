package cosimul

import (
	"testing"
	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/cosi"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestCoSimul(t *testing.T) {
	for VerifyResponse = 0; VerifyResponse < 3; VerifyResponse++ {
		for _, nbrHosts := range []int{1, 3, 13} {
			log.Lvl2("Running cosi with", nbrHosts, "hosts")
			local := sda.NewLocalTest()

			hosts, _, tree := local.GenBigTree(nbrHosts, nbrHosts, 3, true)
			log.Lvl2(tree.Dump())

			done := make(chan bool)
			// create the message we want to sign for this round
			msg := []byte("Hello World Cosi")

			// Register the function generating the protocol instance
			var root *CoSimul
			// function that will be called when protocol is finished by the root
			doneFunc := func(sig []byte) {
				suite := hosts[0].Suite()
				if err := cosi.VerifySignature(suite, root.Publics(),
					msg, sig); err != nil {
					t.Fatal("error verifying signature:", err)
				} else {
					log.Lvl1("Verification OK")
				}
				done <- true
			}

			// Start the protocol
			p, err := local.CreateProtocol(Name, tree)
			if err != nil {
				t.Fatal("Couldn't create new node:", err)
			}
			root = p.(*CoSimul)
			root.Message = msg
			root.RegisterSignatureHook(doneFunc)
			go root.StartProtocol()
			select {
			case <-done:
			case <-time.After(time.Second * 2):
				t.Fatal("Could not get signature verification done in time")
			}
			local.CloseAll()
		}
	}
}
