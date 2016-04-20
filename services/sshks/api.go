package sshks

import (
	"errors"
	"os/user"
	"strings"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"golang.org/x/net/context"
)

// NetworkSendAnonymous makes a connection to a remote host without checking
// it's public key. This is useful if you want the user to ask only for the
// address of the server, without providing the public key. Of course this is
// not secure!
func NetworkSendAnonymous(addr string, req network.ProtocolMessage) (*network.Message, error) {
	// create a throw-away key pair:
	kp := config.NewKeyPair(network.Suite)
	dst := network.NewEntity(kp.Public, addr)
	return NetworkSend(kp.Secret, dst, req)
}

// NetworkSend opens the connection to 'dst' and sends the message 'req'. The
// reply is returned, or an error if the timeout of 10 seconds is reached.
func NetworkSend(sec abstract.Secret, dst *network.Entity, req network.ProtocolMessage) (*network.Message, error) {
	pub := network.Suite.Point().Mul(nil, sec)
	client := network.NewSecureTCPHost(sec, network.NewEntity(pub, ""))

	// Connect to the root
	dbg.Lvl4("Opening connection to", dst)
	con, err := client.Open(dst)
	defer client.Close()
	if err != nil {
		return &network.Message{}, err
	}

	pchan := make(chan network.Message)
	go func() {
		// send the request
		dbg.Lvlf3("Sending request %+v", req)
		if err := con.Send(context.TODO(), req); err != nil {
			close(pchan)
			return
		}
		dbg.Lvl4("Waiting for the response")
		// wait for the response
		packet, err := con.Receive(context.TODO())
		if err != nil {
			close(pchan)
			return
		}
		pchan <- packet
	}()
	select {
	case response := <-pchan:
		dbg.Lvlf5("Response: %+v", response)
		return &response, nil
	case <-time.After(time.Second * 10):
		return &network.Message{}, errors.New("Timeout on signing")
	}
}

// ExpandHDir takes a string and expands any leading '~' with the home-dir
// of the user.
func ExpandHDir(dir string) string {
	usr, _ := user.Current()
	hdir := usr.HomeDir

	// Check in case of paths like "/something/~/something/"
	if dir[:2] == "~/" {
		return strings.Replace(dir, "~", hdir, 1)
	}
	return dir
}
