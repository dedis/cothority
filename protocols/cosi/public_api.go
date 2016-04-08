package cosi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dedis/cothority/app"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

func init() {
	dbg.Print("Registering")
	dbg.Print(network.RegisterMessageType(CosiRequest{}))
	dbg.Print(network.RegisterMessageType(CosiResponse{}))
}

type CosiApp struct {
	host    *sda.Host
	overlay *sda.Overlay
	Entity  *network.Entity
}

// CLI Part of SDA

// CosiRequest is used by the client to send something to sda that
// will in turn give that to the CoSi system.
// It contains the message the client wants to sign.
type CosiRequest struct {
	// The entity list to use for creating the cosi tree
	EntityList *sda.EntityList
	// the actual message to sign by CoSi.
	Message []byte
}

// CosiResponse contains the signature out of the CoSi system.
// It can be verified using the lib/cosi package.
// NOTE: the `suite` field is absent here because this struct is a temporary
// hack and we only supports one suite for the moment,i.e. ed25519.
type CosiResponse struct {
	// The hash of the signed statement
	Sum []byte
	// The Challenge out a of the Multi Schnorr signature
	Challenge abstract.Secret
	// the Response out of the Multi Schnorr Signature
	Response abstract.Secret
}

// MarshalJSON implements golang's JSON marshal interface
func (s *CosiResponse) MarshalJSON() ([]byte, error) {
	cw := new(bytes.Buffer)
	rw := new(bytes.Buffer)

	err := crypto.WriteSecret64(network.Suite, cw, s.Challenge)
	if err != nil {
		return nil, err
	}
	err = crypto.WriteSecret64(network.Suite, rw, s.Response)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Sum       string
		Challenge string
		Response  string
	}{
		Sum:       base64.StdEncoding.EncodeToString(s.Sum),
		Challenge: cw.String(),
		Response:  rw.String(),
	})
}

// UnmarshalJSON implements golang's JSON unmarshal interface
func (s *CosiResponse) UnmarshalJSON(data []byte) error {
	type Aux struct {
		Sum       string
		Challenge string
		Response  string
	}
	aux := &Aux{}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	var err error
	if s.Sum, err = base64.StdEncoding.DecodeString(aux.Sum); err != nil {
		return err
	}
	suite := network.Suite
	cr := strings.NewReader(aux.Challenge)
	if s.Challenge, err = crypto.ReadSecret64(suite, cr); err != nil {
		return err
	}
	rr := strings.NewReader(aux.Response)
	if s.Response, err = crypto.ReadSecret64(suite, rr); err != nil {
		return err
	}
	return nil
}

func CreateCosiApp(ip string) *CosiApp {
	// create the public / private keys
	kp := config.NewKeyPair(network.Suite)
	entity := network.NewEntity(kp.Public, ip)
	host := sda.NewHost(entity, kp.Secret)

	return &CosiApp{
		host:    host,
		overlay: host.GetOverlay(),
	}
}

func ReadCosiApp(cnf string) (*CosiApp, error) {
	h, err := sda.NewHostFromFile(cnf)
	if err != nil {
		return nil, err
	}
	return &CosiApp{
		host:    h,
		overlay: h.GetOverlay(),
	}, nil
}

func AddCosiApp(h *sda.Host) {
	ca := &CosiApp{
		host:    h,
		overlay: h.GetOverlay(),
	}
	ca.host.RegisterMessage(CosiRequest{}, ca.HandleCosiRequest)
}

// PrintServer prints out the configuration that can be copied to the servers.toml
// file
func (ca *CosiApp) PrintServer() {
	serverToml := app.NewServerToml(network.Suite, ca.host.Entity.Public,
		ca.host.Entity.Addresses...)
	groupToml := app.NewGroupToml(serverToml)
	fmt.Println(groupToml.String())
}

// Start listens on the incoming port and DOES NOT return
func (ca *CosiApp) Start() {
	ca.host.RegisterMessage(CosiRequest{}, ca.HandleCosiRequest)
	ca.host.Listen()
	ca.host.StartProcessMessages()
	ca.host.WaitForClose()
}

// Write saves the config to a file
func (ca *CosiApp) Write(cnf string) error {
	if err := ca.host.SaveToFile(cnf); err != nil {
		return err
	}
	return nil
}

// handleCosiRequest will
// * register the entitylist
// * create a flat tree out of it with him being at the root
// * launch a CoSi protocol
// * wait for it to finish and send back the response to the client
func (ca *CosiApp) HandleCosiRequest(msg *network.Message) network.ProtocolMessage {
	dbg.Lvl2("Received client CosiRequest from", msg.From)
	empty := &CosiResponse{}
	cr, ok := msg.Msg.(CosiRequest)
	if !ok {
		return empty
	}
	idx, e := cr.EntityList.Search(ca.host.Entity.ID)
	if e == nil {
		dbg.Error("Received CosiRequest without being included in the Entitylist")
		return empty
	}
	if idx != 0 {
		// replace the first entity by this host's entity
		tmp := cr.EntityList.List[0]
		cr.EntityList.List[0] = e
		cr.EntityList.List[idx] = tmp
	}
	// register & create the tree
	ca.overlay.RegisterEntityList(cr.EntityList)
	// for the moment let's just stick to a very simple binary tree
	tree := cr.EntityList.GenerateBinaryTree()
	ca.overlay.RegisterTree(tree)

	// run the CoSi protocol
	node, err := ca.overlay.CreateNewNodeName("CoSi", tree)
	if err != nil {
		dbg.Error("Error creating tree upon client request:", err)
		return empty
	}
	pcosi := node.ProtocolInstance().(*ProtocolCosi)
	pcosi.SigningMessage(cr.Message)
	hash := network.Suite.Hash()
	if _, err := hash.Write(cr.Message); err != nil {
		dbg.Error("Couldn't hash message:", err)
	}
	sum := hash.Sum(nil)
	// Register the handler when the signature is finished
	rchan := make(chan *CosiResponse)
	fn := func(chal, resp abstract.Secret) {
		response := &CosiResponse{
			Sum:       sum,
			Challenge: chal,
			Response:  resp,
		}
		dbg.Lvl2("Getting CoSi signature back => sending to client")
		// send back to client
		rchan <- response
	}
	pcosi.RegisterDoneCallback(fn)
	dbg.Lvl2("Starting CoSi protocol...")
	go node.StartProtocol()
	return <-rchan
}

// sign takes a stream and a toml file defining the servers
func Sign(r io.Reader, tomlFileName string) (*CosiResponse, error) {
	dbg.Lvl3("Starting signature")
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}
	el, err := app.ReadGroupToml(f)
	if err != nil {
		return nil, err
	}
	dbg.Lvl2("Sending signature to", el)
	res, err := SignStatement(r, el)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// signStatement can be used to sign the contents passed in the io.Reader
// (pass an io.File or use an strings.NewReader for strings)
func SignStatement(r io.Reader, el *sda.EntityList) (*CosiResponse, error) {

	// create a throw-away key pair:
	kp := config.NewKeyPair(network.Suite)

	// create a throw-away entity with an empty  address:
	e := network.NewEntity(kp.Public, "")
	client := network.NewSecureTCPHost(kp.Secret, e)
	msg, _ := crypto.HashStream(network.Suite.Hash(), r)
	req := &CosiRequest{
		EntityList: el,
		Message:    msg,
	}

	// Connect to the root
	host := el.List[0]
	dbg.Lvl3("Opening connection to", host.Addresses[0], host.Public)
	con, err := client.Open(host)
	defer client.Close()
	if err != nil {
		return nil, err
	}

	dbg.Lvl3("Sending sign request")
	pchan := make(chan CosiResponse)
	go func() {
		// send the request
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
		pchan <- packet.Msg.(CosiResponse)
	}()
	select {
	case response, ok := <-pchan:
		dbg.Lvl5("Response:", ok, response)
		if !ok {
			return nil, errors.New("Invalid repsonse: Could not cast the " +
				"received response to the right type")
		}
		err = cosi.VerifySignature(network.Suite, msg, el.Aggregate,
			response.Challenge, response.Response)
		if err != nil {
			return nil, err
		}
		return &response, nil
	case <-time.After(time.Second * 10):
		return nil, errors.New("Timeout on signing")
	}
}

// verify takes a file and a group-definition, calls the signature
// verification and prints the result
func Verify(fileName, groupToml string) error {
	// if the file hash matches the one in the signature
	dbg.Lvl4("Reading file " + fileName)
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}
	// Read the JSON signature file
	dbg.Lvl4("Reading signature")
	sb, err := ioutil.ReadFile(fileName + ".sig")
	if err != nil {
		return err
	}
	sig := &CosiResponse{}
	dbg.Lvl4("Unmarshalling signature ")
	if err := json.Unmarshal(sb, sig); err != nil {
		return err
	}
	fGroup, err := os.Open(groupToml)
	if err != nil {
		return err
	}
	dbg.Lvl4("Reading group definition")
	el, err := app.ReadGroupToml(fGroup)
	if err != nil {
		return err
	}
	dbg.Lvl4("Verfifying signature")
	err = VerifySignatureHash(b, sig, el)
	return err
}

// verifySignature checks whether the signature is valid
func VerifySignatureHash(b []byte, sig *CosiResponse, el *sda.EntityList) error {
	// We have to hash twice, as the hash in the signature is the hash of the
	// message sent to be signed
	fHash, _ := crypto.HashBytes(network.Suite.Hash(), b)
	hashHash, _ := crypto.HashBytes(network.Suite.Hash(), fHash)
	if !bytes.Equal(hashHash, sig.Sum) {
		return errors.New("You are trying to verify a signature " +
			"belongig to another file. (The hash provided by the signature " +
			"doesn't match with the hash of the file.)")
	}
	if err := cosi.VerifySignature(network.Suite, fHash, el.Aggregate, sig.Challenge, sig.Response); err != nil {
		return errors.New("Invalid sig:" + err.Error())
	}
	return nil
}
