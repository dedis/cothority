package ssh_ks

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"golang.org/x/net/context"
	"os/user"
	"strings"
	"time"
)

func NetworkSendAnonymous(addr string, req network.ProtocolMessage) (*network.Message, error) {
	// create a throw-away key pair:
	kp := config.NewKeyPair(network.Suite)
	dst := network.NewEntity(kp.Public, addr)
	return NetworkSend(kp.Secret, dst, req)
}

func NetworkSend(sec abstract.Secret, dst *network.Entity, req network.ProtocolMessage) (*network.Message, error) {
	client := network.NewSecureTCPHost(sec, nil)

	// Connect to the root
	dbg.Lvl3("Opening connection")
	con, err := client.Open(dst)
	defer client.Close()
	if err != nil {
		return &network.Message{}, err
	}

	pchan := make(chan network.Message)
	go func() {
		// send the request
		dbg.Lvl3("Sending request", req)
		if err := con.Send(context.TODO(), req); err != nil {
			close(pchan)
			return
		}
		dbg.Lvl3("Waiting for the response")
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
		dbg.Lvl5("Response:", response)
		return &response, nil
	case <-time.After(time.Second * 10):
		return &network.Message{}, errors.New("Timeout on signing")
	}
}

func ExpandHDir(dir string) string {
	usr, _ := user.Current()
	hdir := usr.HomeDir

	// Check in case of paths like "/something/~/something/"
	if dir[:2] == "~/" {
		return strings.Replace(dir, "~", hdir, 1)
	}
	return dir
}
