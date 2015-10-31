/*
 * This is a simple interface to get a string stamped by
 * a cothority. It can be used as standalone or in an
 * application that needs collective signing from an existing
 * cothority.
 */
package conode
import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/coconet"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"strings"
	"strconv"
	"math/rand"
	"github.com/dedis/crypto/abstract"
	"bytes"
	"encoding/base64"
	"fmt"
)

type Stamp struct {
	Config app.ConfigConode
	X0     abstract.Point
	Suite  abstract.Suite
	conn   *coconet.TCPConn
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
func (s *Stamp)GetStamp(msg, server string) (*TimeStampMessage, error) {
	err := s.connect(server)
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
	if !VerifySignature(s.Suite, tsm.Srep, s.X0, []byte(msg)) {
		return nil, fmt.Errorf("Verification of signature failed")
	}

	return tsm, nil
}

// Used to connect to server
func (s *Stamp)connect(server string) error {
	// First get a connection. Get a random one if no server provided
	if server == "" {
		serverPort := strings.Split(s.Config.Hosts[rand.Intn(len(s.Config.Hosts))], ":")
		server = serverPort[0]
		port, _ := strconv.Atoi(serverPort[1])
		server += ":" + strconv.Itoa(port + 1)
	}
	if !strings.Contains(server, ":") {
		server += ":2000"
	}
	dbg.Lvl2("Connecting to", server)
	s.conn = coconet.NewTCPConn(server)
	err := s.conn.Connect()
	if err != nil {
		return fmt.Errorf("Couldn't get connection to host: %s", err)
	}

	dbg.Lvl3("Connected to ", server)
	return nil
}

// This stamps the message, but the connection already needs
// to be set up
func (s *Stamp)stamp(msg string)(*TimeStampMessage, error) {
	tsmsg := &TimeStampMessage{
		Type:  StampRequestType,
		ReqNo: 0,
		Sreq:  &StampRequest{Val: []byte(msg)}}

	err := s.conn.PutData(tsmsg)
	if err != nil {
		return nil, fmt.Errorf("Couldn't send hash-message to server: %s", err)
	}
	dbg.Lvl3("Sent signature request")

	// Wait for the signed message
	tsm := &TimeStampMessage{}
	tsm.Srep = &StampReply{}
	tsm.Srep.SuiteStr = s.Suite.String()
	err = s.conn.GetData(tsm)
	if err != nil {
		return nil, fmt.Errorf("Error while receiving signature: %s", err)
	}
	dbg.Lvl3("Got signature response")
	return tsm, nil
}

// Asking to close the connection
func (s *Stamp)disconnect() error {
	err := s.conn.PutData(&TimeStampMessage{
		ReqNo: 1,
		Type:  StampClose,
	})
	if err != nil{
		return err
	}

	s.conn.Close()
	dbg.Lvl3("Connection closed with server")
	return nil
}
