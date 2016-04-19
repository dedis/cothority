package sshks_test

import (
	"testing"

	"bytes"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/services/sshks"
)

func TestNetworkFunctions(t *testing.T) {
	client, servers := newTest(1)
	defer closeServers(t, servers)
	srv := servers[0].This

	// Adding ourselves and first server
	dbg.Lvl2("Public key", client.This.Entity.Public)
	client.NewConfig = sshks.NewConfig(1)
	client.Cosi = cosi.NewCosi(network.Suite, client.Private)
	dbg.ErrFatal(client.NewConfig.AddClient(client.This))
	dbg.ErrFatal(client.NewConfig.AddServer(srv))
	dbg.ErrFatal(client.NetworkSendFirstCommit(srv))
	dbg.ErrFatal(client.NetworkSendNewConfig(srv))
	dbg.ErrFatal(client.ConfirmNewConfig(srv))

	// Verify the configuration
	conf := client.Config
	if conf.Version != 1 {
		t.Fatal("First version should be 1, not", conf.Version)
	}
	if conf.VerifySignature() != nil {
		t.Fatal("Signature should be valid")
	}
	if len(conf.Clients) != 1 {
		t.Fatal("Should have 1 client signed up")
	}
	dbg.Lvl2(conf.Clients, *client.This)
	if conf.Clients[client.This.Id()].Entity.ID != client.This.Entity.ID {
		t.Fatal("First stored client should be us")
	}
	if conf.Servers[srv.Id()].Entity.ID != srv.Entity.ID {
		t.Fatal("First stored server should be this one")
	}
}

func TestFirstClient(t *testing.T) {
	client, servers := newTest(2)
	defer closeServers(t, servers)
	srv1, srv2 := servers[0].This, servers[1].This

	// Adding ourselves and first server
	dbg.ErrFatal(client.AddClient(client.This))
	dbg.ErrFatal(client.AddServer(srv1))

	// Verify the configuration
	conf := servers[0].Config
	if conf.Version != 1 {
		t.Fatal("First version should be 1, not", conf.Version)
	}
	if conf.VerifySignature() != nil {
		t.Fatal("Signature should be valid")
	}
	if len(conf.Clients) != 1 {
		t.Fatal("Should have 1 client signed up")
	}
	if conf.Clients[client.This.Id()].Entity.ID != client.This.Entity.ID {
		t.Fatal("First stored client should be us")
	}
	if conf.Servers[srv1.Id()].Entity.ID != srv1.Entity.ID {
		t.Fatal("First stored server should be this one")
	}

	// Second server also added automatically
	err := client.AddServer(srv2)
	dbg.ErrFatal(err)
	conf = client.Config
	if conf.Version != 2 {
		t.Fatal("Version should be 2 now")
	}
	if len(conf.Servers) != 2 {
		t.Fatal("Should already have two servers")
	}
}

func TestMoreClients(t *testing.T) {
	cks1, servers := newTest(1)
	defer closeServers(t, servers)
	srv1 := servers[0].This

	// Setup first client and server
	dbg.ErrFatal(cks1.AddClient(cks1.This))
	dbg.ErrFatal(cks1.AddServer(srv1))
	if len(servers[0].Config.Clients) != 1 {
		t.Fatal("Should have 1 client now")
	}

	// Add a second client
	cks2 := newClient(1)
	dbg.ErrFatal(cks1.AddClient(cks2.This))
	conf := servers[0].Config
	dbg.ErrFatal(conf.VerifySignature())
	if len(conf.Clients) != 2 {
		t.Fatal("Should have 2 clients now")
	}
	if conf.Version != 2 {
		t.Fatal("Should be version 2 now")
	}

	// Update second client
	dbg.ErrFatal(cks2.Update(srv1))
	dbg.ErrFatal(cks2.Config.VerifySignature())
	if !bytes.Equal(cks1.Config.Hash(), cks2.Config.Hash()) {
		t.Fatal("Both configs should be the same")
	}

	// And add a third client - needs two signatures, now
	cks3 := newClient(2)
	dbg.ErrFatal(cks1.AddClient(cks3.This))
	if cks2.NewConfig != nil {
		t.Fatal("Client 2 should not have a NewConfig for now")
	}
	dbg.ErrFatal(cks2.Update(srv1))
	if cks2.NewConfig == nil {
		t.Fatal("Client 2 should have a NewConfig now")
	}
	dbg.ErrFatal(cks2.ConfirmNewConfig(nil))

	//dbg.ErrFatal(cks2.AddClient(cks3.This))
}

func TestAddServerSecondClient(t *testing.T) {
	cks1, servers := newTest(1)
	defer closeServers(t, servers)
	srv1 := servers[0].This

	// Setup first client and server
	dbg.ErrFatal(cks1.AddClient(cks1.This))
	dbg.ErrFatal(cks1.AddServer(srv1))

	// Add a second client
	cks2 := newClient(1)
	err := cks2.AddServer(srv1)
	if err == nil {
		t.Fatal("Server should refuse to add unknown client")
	}
	dbg.ErrFatal(cks1.AddClient(cks2.This))
	if len(servers[0].Config.Clients) != 2 {
		t.Fatal("Should have 2 clients now")
	}

	// Update second client
	dbg.ErrFatal(cks2.Update(srv1))
	dbg.ErrFatal(cks2.Config.VerifySignature())
	if !bytes.Equal(cks1.Config.Hash(), cks2.Config.Hash()) {
		t.Fatal("Both configs should be the same")
	}

	// Check there is a commit sent and we can sign things
	cks3 := newClient(2)
	dbg.ErrFatal(cks1.AddClient(cks3.This))
	if len(cks1.Config.Clients) != 2 {
		t.Fatal("Should have 2 clients in cks1", *cks1.Config)
	}
	dbg.ErrFatal(cks2.Update(nil))
	dbg.ErrFatal(cks2.ConfirmNewConfig(nil))
	if len(cks2.Config.Clients) != 3 {
		t.Fatal("Should have 3 clients in cks2")
	}
	dbg.ErrFatal(cks1.Update(nil))
}

func TestServerPropose(t *testing.T) {
	cks1, servers := newTest(2)
	defer closeServers(t, servers)
	srv1, srv2 := servers[0].This, servers[1].This

	// Setup first client and server
	dbg.ErrFatal(cks1.AddServer(srv1))
	dbg.ErrFatal(cks1.AddServer(srv2))

	cks2 := newClient(1)
	err := cks2.AddServer(srv1)
	if err == nil {
		t.Fatal("Client 2 shouldn't be able to sign up")
	}
	dbg.ErrFatal(cks2.ServerPropose(srv1))
	dbg.ErrFatal(cks1.Update(srv2))
	if cks1.NewConfig == nil {
		t.Fatal("Didn't propagate new config")
	}
	if cks1.NewConfig.Version != 3 {
		t.Fatal("New proposed version should be 3:", cks1.NewConfig.Version)
	}
	if len(cks1.NewConfig.Clients) != 2 {
		t.Fatal("Should have 2 clients in proposed config")
	}

	dbg.ErrFatal(cks1.ConfirmNewConfig(nil))
	dbg.ErrFatal(cks2.AddServer(srv1))
	if len(cks2.Config.Clients) != 2 {
		t.Fatal("Client2 should have both clients")
	}
	if len(cks1.Config.Clients) != 2 {
		t.Fatal("Client1 should also have both clients")
	}

	cks3 := newClient(2)
	err = cks3.AddServer(srv1)
	if err == nil {
		t.Fatal("Client3 should not be able to connect to srv1")
	}
	if len(servers[0].Config.Clients) == 3 {
		t.Fatal("Server should not have stored client 3")
	}

	// Now add client three, but this time both clients need to sign
	dbg.ErrFatal(cks3.ServerPropose(srv1))
	dbg.ErrFatal(cks1.Update(nil))
	dbg.ErrFatal(cks2.Update(nil))
	dbg.ErrFatal(cks1.ConfirmNewConfig(nil))
	if len(cks1.Config.Clients) == 3 {
		t.Fatal("Client1 should not have client 3 yet")
	}
	dbg.ErrFatal(cks2.ConfirmNewConfig(nil))
	dbg.ErrFatal(cks3.AddServer(srv1))
	if len(servers[0].Config.Clients) != 3 {
		t.Fatal("There should be 3 clients stored now")
	}
}

func TestClientSign(t *testing.T) {
	// Test if a partial signature is correctly propagated
	cks1, servers := newTest(3)
	defer closeServers(t, servers)
	srv1, srv2, srv3 := servers[0].This, servers[1].This, servers[2].This
	cks2 := newClient(1)

	// Setup first client and server
	dbg.ErrFatal(cks1.AddServer(srv1))
	dbg.ErrFatal(cks1.AddServer(srv2))
	dbg.ErrFatal(cks1.AddClient(cks2.This))
	dbg.ErrFatal(cks2.AddServer(srv1))

	// Adding third server over one server and updating over the other
	var err error
	cks2.NewConfig, err = cks2.Config.Copy()
	dbg.ErrFatal(err)
	cks2.NewConfig.AddServer(srv3)
	dbg.ErrFatal(cks2.NetworkSendNewConfig(srv2))
	dbg.ErrFatal(cks2.ConfirmNewConfig(srv2))
	dbg.ErrFatal(cks1.Update(srv1))
	dbg.ErrFatal(cks1.ConfirmNewConfig(srv1))
	dbg.ErrFatal(cks2.Update(srv2))
	if len(cks1.Config.Servers) != 3 {
		t.Fatal("Client 1 should have 3 servers now")
	}
	if len(cks2.Config.Servers) != 3 {
		t.Fatal("Client 2 should have 3 servers now")
	}
}

func TestConfigMultiple(t *testing.T) {
	// Adding a server and a client, then signing the config and verifying
	// that there is only one version more and not multiple versions
}

func TestAddNewConfig(t *testing.T) {
}
