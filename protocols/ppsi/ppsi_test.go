package main

import (
	"fmt"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/ppsi_crypto_utils"
	"testing"
)

func TestPPSI(t *testing.T) {

	set1 := []string{"543323345", "543323045", "843323345", "213323045", "843323345"}
	set2 := []string{"543323345", "543323045", "843343345", "213323045", "843323345"}
	set3 := []string{"543323345", "543323045", "843323345", "213323045", "843323345"}
	set4 := []string{"543323345", "543323045", "843333345", "548323032", "213323045"}
	set5 := []string{"543323345", "543323045", "843323345", "543323245", "213323045"}
	set6 := []string{"543323345", "543323045", "843333345", "543323032", "213323045"}

	setsToEncrypt := [][]string{set1, set2, set3, set4, set5, set6}
	local := sda.NewLocalTest()
	hosts, el, tree := local.GenBigTree(6, 6, 5, true)
	suite := hosts[0].Suite()
	publics := el.Publics()
	ppsi := ppsi_crypto_utils.NewPPSI2(suite, publics, 6)
	EncPhones := ppsi.EncryptPhones(setsToEncrypt, 6)

	done := make(chan bool)
	// IdsToInterset  := []int{0,1,2}

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
