package main

import (
	"fmt"
	"testing"
	"github.com/dedis/crypto/edwards/ed25519"
	//"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)


func TestRand(t *testing.T) {

	net := newChanNet()
	suite := ed25519.NewAES128SHA256Ed25519(false)
	rand := random.Stream

	nservers := 2
	srv := make([]Server, nservers)
	//srvkeys := make([]abstract.Point, nservers)
	srvname := make([]string, nservers)
	for i := 0; i < nservers; i++ {
		//pri := suite.Secret().Pick(rand)
		//srvkeys[i] = servers[i].pubKey
		srvname[i] = fmt.Sprintf("server%d", i)
		host := newChanHost(net, srvname[i])
		srv[i].init(host, suite)
		go srv[i].run()
	}

	cli := Client{}
	//cpri := suite.Secret().Pick(rand)
	chost := newChanHost(net, "client")
	cli.init(chost, suite, rand, srvname)

	if err := cli.run(); err != nil {
		panic(err)
	}
}

