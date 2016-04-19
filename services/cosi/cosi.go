package cosi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/cosi"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"golang.org/x/net/context"
)

// This file contains all the code to run a CoSi service. It is used to reply to
// client request for signing something using CoSi.
// As a prototype, it just signs and returns. It would be very easy to write an
// updated version that chains all signatures for example.

func init() {
	sda.RegisterNewService("Cosi", newCosiService)
}

// Cosi is the service that handles collective signing operations
type Cosi struct {
	c    sda.Context
	path string
}

// ServiceRequest is what the Cosi service is expected to receive from clients.
type ServiceRequest struct {
	Message    []byte
	EntityList *sda.EntityList
}

// CosiRequestType is the type that is embedded in the Request object for a
// CosiRequest
var CosiRequestType = network.RegisterMessageType(ServiceRequest{})

// ServiceResponse is what the Cosi service will reply to clients.
type ServiceResponse struct {
	Challenge abstract.Secret
	Response  abstract.Secret
}

// CosiRequestType is the type that is embedded in the Request object for a
// CosiResponse
var CosiResponseType = network.RegisterMessageType(ServiceResponse{})

// ProcessRequest treats external request to this service.
func (cs *Cosi) ProcessClientRequest(e *network.Entity, r *sda.ClientRequest) {
	if r.Type != CosiRequestType {
		return
	}
	var req ServiceRequest
	// XXX should provide a UnmarshalRegisteredType(buff) func instead of having to give
	// the constructors with the suite.
	id, pm, err := network.UnmarshalRegisteredType(r.Data, network.DefaultConstructors(network.Suite))
	if err != nil {
		dbg.Error(err)
		return
	}
	if id != CosiRequestType {
		dbg.Error("Wrong message coming in")
		return
	}
	req = pm.(ServiceRequest)
	tree := req.EntityList.GenerateBinaryTree()
	tni := cs.c.NewTreeNodeInstance(tree, tree.Root)
	pi, err := cosi.NewProtocolCosi(tni)
	if err != nil {
		return
	}
	cs.c.RegisterProtocolInstance(pi)
	pcosi := pi.(*cosi.ProtocolCosi)
	pcosi.SigningMessage(req.Message)
	pcosi.RegisterDoneCallback(func(chall abstract.Secret, resp abstract.Secret) {
		respMessage := &ServiceResponse{
			Challenge: chall,
			Response:  resp,
		}
		if err := cs.c.SendRaw(e, respMessage); err != nil {
			dbg.Error(err)
		}
	})
	dbg.Lvl1("Cosi Service starting up root protocol")
	go pi.Dispatch()
	go pi.Start()
}

// ProcessServiceMessage is to implement the Service interface.
func (cs *Cosi) ProcessServiceMessage(e *network.Entity, s *sda.ServiceMessage) {
	return
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (c *Cosi) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dbg.Lvl1("Cosi Service received New Protocol event")
	pi, err := cosi.NewProtocolCosi(tn)
	go pi.Dispatch()
	return pi, err
}

func newCosiService(c sda.Context, path string) sda.Service {
	return &Cosi{
		c:    c,
		path: path,
	}
}

// PrintServer prints out the configuration that can be copied to the servers.toml
// file
func (ca *App) PrintServer() {
	serverToml := NewServerToml(network.Suite, ca.host.Entity.Public,
		ca.host.Entity.Addresses...)
	groupToml := NewGroupToml(serverToml)
	fmt.Println(groupToml.String())
}

// Start listens on the incoming port and DOES NOT return
func (ca *App) Start() {
	ca.host.RegisterExternalMessage(SignRequest{}, ca.HandleCosiSignRequest)
	ca.host.Listen()
	ca.host.StartProcessMessages()
	ca.host.WaitForClose()
}

// Write saves the config to a file
func (ca *App) Write(cnf string) error {
	if err := ca.host.SaveToFile(cnf); err != nil {
		return err
	}
	return nil
}

// handleCosiSignRequest will
// * register the entitylist
// * create a flat tree out of it with him being at the root
// * launch a CoSi protocol
// * wait for it to finish and send back the response to the client
func (ca *App) HandleCosiSignRequest(msg *network.Message) network.ProtocolMessage {
	dbg.Lvl2("Received client CosiSignRequest from", msg.From)
	empty := &SignResponse{}
	cr, ok := msg.Msg.(SignRequest)
	if !ok {
		return empty
	}
	idx, e := cr.EntityList.Search(ca.host.Entity.ID)
	if e == nil {
		dbg.Error("Received CosiSignRequest without being included in the Entitylist")
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
		dbg.Error("Error creating tree upon client SignRequest:", err)
		return empty
	}
	pcosi := node.ProtocolInstance().(*cosi.ProtocolCosi)
	pcosi.SigningMessage(cr.Message)
	hash := network.Suite.Hash()
	if _, err := hash.Write(cr.Message); err != nil {
		dbg.Error("Couldn't hash message:", err)
	}
	sum := hash.Sum(nil)
	// Register the handler when the signature is finished
	rchan := make(chan *SignResponse)
	fn := func(chal, resp abstract.Secret) {
		response := &SignResponse{
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
func Sign(r io.Reader, tomlFileName string) (*SignResponse, error) {
	dbg.Lvl3("Starting signature")
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}
	el, err := ReadGroupToml(f)
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
func SignStatement(r io.Reader, el *sda.EntityList) (*SignResponse, error) {

	// create a throw-away key pair:
	kp := config.NewKeyPair(network.Suite)

	// create a throw-away entity with an empty  address:
	e := network.NewEntity(kp.Public, "")
	client := network.NewSecureTCPHost(kp.Secret, e)
	msg, _ := crypto.HashStream(network.Suite.Hash(), r)
	req := &SignRequest{
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

	dbg.Lvl3("Sending sign SignRequest")
	pchan := make(chan SignResponse)
	go func() {
		// send the SignRequest
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
		pchan <- packet.Msg.(SignResponse)
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
	sig := &SignResponse{}
	dbg.Lvl4("Unmarshalling signature ")
	if err := json.Unmarshal(sb, sig); err != nil {
		return err
	}
	fGroup, err := os.Open(groupToml)
	if err != nil {
		return err
	}
	dbg.Lvl4("Reading group definition")
	el, err := ReadGroupToml(fGroup)
	if err != nil {
		return err
	}
	dbg.Lvl4("Verfifying signature")
	err = VerifySignatureHash(b, sig, el)
	return err
}

// verifySignature checks whether the signature is valid
func VerifySignatureHash(b []byte, sig *SignResponse, el *sda.EntityList) error {
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

// GroupToml represents the structure of the group.toml file given to the cli.
type GroupToml struct {
	Description string
	Servers     []*ServerToml `toml:"servers"`
}

// ServerToml is one entry in the group.toml file describing one server to use for
// the cothority system.
type ServerToml struct {
	Addresses   []string
	Public      string
	Description string
}

// ReadGroupToml reads a group.toml file and returns the list of Entity
// described in the file.
func ReadGroupToml(f io.Reader) (*sda.EntityList, error) {
	group := &GroupToml{}
	_, err := toml.DecodeReader(f, group)
	if err != nil {
		return nil, err
	}
	// convert from ServerTomls to entities
	var entities = make([]*network.Entity, 0, len(group.Servers))
	for _, s := range group.Servers {
		en, err := s.toEntity(network.Suite)
		if err != nil {
			return nil, err
		}
		entities = append(entities, en)
	}
	el := sda.NewEntityList(entities)
	return el, nil
}

// NewGroupToml creates a new GroupToml struct from the given ServerTomls.
// Currently used together with calling String() on the GroupToml to output
// a snippet which is needed to define the CoSi group
func NewGroupToml(servers ...*ServerToml) *GroupToml {
	return &GroupToml{
		Servers: servers,
	}
}

// String returns the TOML representation of this GroupToml
func (gt *GroupToml) String() string {
	var buff bytes.Buffer
	if gt.Description == "" {
		gt.Description = "Description of the system"
	}
	for _, s := range gt.Servers {
		if s.Description == "" {
			s.Description = "Description of the server"
		}
	}
	enc := toml.NewEncoder(&buff)
	if err := enc.Encode(gt); err != nil {
		return "Error encoding grouptoml" + err.Error()
	}
	return buff.String()
}

// toEntity will convert this ServerToml struct to a network entity.
func (s *ServerToml) toEntity(suite abstract.Suite) (*network.Entity, error) {
	pubR := strings.NewReader(s.Public)
	public, err := crypto.ReadPub64(suite, pubR)
	if err != nil {
		return nil, err
	}
	return network.NewEntity(public, s.Addresses...), nil
}

// Returns a ServerToml out of a public key and some addresses => to be printed
// or written to a file
func NewServerToml(suite abstract.Suite, public abstract.Point, addresses ...string) *ServerToml {
	var buff bytes.Buffer
	if err := crypto.WritePub64(suite, &buff, public); err != nil {
		dbg.Error("Error writing public key")
		return nil
	}
	return &ServerToml{
		Addresses: addresses,
		Public:    buff.String(),
	}
}

// Returns its TOML representation
func (s *ServerToml) String() string {
	var buff bytes.Buffer
	if s.Description == "" {
		s.Description = "## Put your description here for convenience ##"
	}
	enc := toml.NewEncoder(&buff)
	if err := enc.Encode(s); err != nil {
		return "## Error encoding server informations ##" + err.Error()
	}
	return buff.String()
}
