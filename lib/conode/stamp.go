package conode

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
	"math/rand"
	"strconv"
	"strings"
)

/*
 * This is a simple interface to get a string stamped by
 * a cothority. It can be used as standalone or in an
 * application that needs collective signing from an existing
 * cothority.
 */

type Stamp struct {
	Config app.ConfigConode
	X0     abstract.Point
	Suite  abstract.Suite
	host   network.Host
	conn   network.Conn
}

// NewStamp initializes a new stamp-client by reading all
// configuration from a "config.toml"-file.
// If an error occurs, it is returned by the second argument.
// It also initializes X0 and Suite for later use.
func NewStamp(file string) (*Stamp, error) {
	s := &Stamp{}
	err := app.ReadTomlConfig(&s.Config, file)
	if err != nil {
		return nil, err
	}
	s.Suite = app.GetSuite(s.Config.Suite)
	pub, _ := base64.StdEncoding.DecodeString(s.Config.AggPubKey)
	s.Suite.Read(bytes.NewReader(pub), &s.X0)
	return s, nil
}

// GetStamp contacts the "server" and waits for the "msg" to
// be signed
// If server is empty, it will contact one randomly
func (s *Stamp) GetStamp(msg []byte, server string) (*StampSignature, error) {
	if server == "" {
		server = s.Config.Hosts[rand.Intn(len(s.Config.Hosts))]
	}
	dbg.Lvl2("StampClient will stamp on server", server)
	portstr := strconv.Itoa(cliutils.GetPort(server, DefaultPort) + 1)
	err := s.connect(cliutils.GetAddress(server) + ":" + portstr)
	if err != nil {
		return nil, err
	}
	tsm, err := s.stamp(msg)
	if err != nil {
		return nil, err
	}

	err = s.disconnect()
	if err != nil {
		return nil, err
	}

	// Verify if what we received is correct
	if !VerifySignature(s.Suite, tsm, s.X0, msg) {
		return nil, fmt.Errorf("Verification of signature failed")
	}

	return tsm, nil
}

// Used to connect to server
func (s *Stamp) connect(server string) error {
	// First get a connection. Get a random one if no server provided
	if server == "" {
		serverPort := strings.Split(s.Config.Hosts[rand.Intn(len(s.Config.Hosts))], ":")
		server = serverPort[0]
		port, _ := strconv.Atoi(serverPort[1])
		server += ":" + strconv.Itoa(port+1)
	}
	if !strings.Contains(server, ":") {
		server += ":2000"
	}
	dbg.Lvl2("Connecting to", server)
	// giving localhost based host since we don't care about our IP address
	s.host = network.NewTcpHost()
	c, err := s.host.Open(server)
	if err != nil {
		return err
	}
	s.conn = c
	dbg.Lvl3("Connected to", server)
	return nil
}

// This stamps the message, but the connection already needs
// to be set up
func (s *Stamp) stamp(msg []byte) (*StampSignature, error) {
	tsmsg := &StampRequest{
		ReqNo: 0,
		Val:   msg}
	ctx := context.TODO()
	err := s.conn.Send(ctx, tsmsg)
	if err != nil {
		return nil, fmt.Errorf("Couldn't send hash-message to server: %s", err)
	}
	dbg.Lvl3("Sent signature request")

	// Wait for the signed message
	am, err := s.conn.Receive(ctx)
	if err != nil || am.MsgType != StampSignatureType {
		return nil, fmt.Errorf("Error while receiving signature: %s", err)
	}
	dbg.Lvl3("Got signature response")
	ss := am.Msg.(StampSignature)
	return &ss, nil
}

// Asking to close the connection
func (s *Stamp) disconnect() error {
	ctx := context.TODO()
	err := s.conn.Send(ctx, &StampClose{
		ReqNo: 1,
	})
	if err != nil {
		return err
	}
	s.host.Close()
	dbg.Lvl3("Connection closed with server")
	return nil
}
