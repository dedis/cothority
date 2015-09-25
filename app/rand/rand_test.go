package main

import (
	"fmt"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards/ed25519"
	"github.com/dedis/crypto/random"
	"github.com/dedis/crypto/sig"
	"testing"
)

func TestRand(t *testing.T) {

	net := newChanNet()
	suite := ed25519.NewAES128SHA256Ed25519(false)
	rand := random.Stream

	// Signing keypair for client
	clisec := sig.SchnorrScheme{Suite: suite}.SecretKey()
	clisec.Pick(rand)
	clipub := clisec.PublicKey()

	nservers := 10
	srv := make([]Server, nservers)
	group := make([]abstract.Point, nservers)
	srvname := make([]string, nservers)
	for i := 0; i < nservers; i++ {
		//pri := suite.Secret().Pick(rand)
		srvname[i] = fmt.Sprintf("server%d", i)
		host := newChanHost(net, srvname[i], srv[i].serve)
		srv[i].init(host, suite, clipub, group, i)
		group[i] = srv[i].keypair.Public
	}

	cli := Client{}
	//cpri := suite.Secret().Pick(rand)
	chost := newChanHost(net, "client", nil)
	cli.init(chost, suite, rand, clisec, srvname, group)

	if err := cli.run(); err != nil {
		panic(err)
	}
}
