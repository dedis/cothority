package ppsi

import (
	"fmt"
	"testing"
)

func TestPPSI(t *testing.T) {

	set1 := []string{"543323345", "543323345", "843323345"}
	set2 := []string{"543323345", "543323045", "843323375"}
	set3 := []string{"543323345", "543323045", "843323345"}

	suite := hosts[0].Suite()
	publics := el.Publics()
	ppsi := ppsi_crypto_utils.NewPPSINP(suite, publics, nodes-1)
	EncPhones := ppsi.EncryptPhones(setsToEncrypt, nodes-1)

	done := make(chan bool)
	// IdsToInterset  := []int{0,1,2}
	local := sda.NewLocalTest()
	_, _, tree := local.GenBigTree(3, 3, 2, true)

	defer local.CloseAll()

	doneFunc := func() {

		done <- true
	}

	var root *PPSI

	p, err := local.CreateProtocol("PPSI", tree)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	root = p.(*PPSI)
	//root.IdsToInterset=IdsToInterset
	root.EncryptedSets = EncPhones
	root.RegisterSignatureHook(doneFunc)
	go root.StartProtocol()

	select {
	case <-done:
		fmt.Printf("%v\n", root.finalIntersection)
		//case <-time.After(time.Second * 2):
		//	t.Fatal("Could not get intersection done in time")
	}

}
