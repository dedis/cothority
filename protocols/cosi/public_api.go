package cosi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
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
	dbg.Print(network.RegisterMessageType(Request{}))
	dbg.Print(network.RegisterMessageType(Response{}))
}

// App represents the application that will listen for signatures
type App struct {
	host    *sda.Host
	overlay *sda.Overlay
	Entity  *network.Entity
}

// Request is used by the client to send something to sda that
// will in turn give that to the CoSi system.
// It contains the message the client wants to sign.
type Request struct {
	// The entity list to use for creating the cosi tree
	EntityList *sda.EntityList
	// the actual message to sign by CoSi.
	Message []byte
}

// Response contains the signature out of the CoSi system.
// It can be verified using the lib/cosi package.
// NOTE: the `suite` field is absent here because this struct is a temporary
// hack and we only supports one suite for the moment,i.e. ed25519.
type Response struct {
	// The hash of the signed statement
	Sum []byte
	// The Challenge out a of the Multi Schnorr signature
	Challenge abstract.Secret
	// the Response out of the Multi Schnorr Signature
	Response abstract.Secret
}

// MarshalJSON implements golang's JSON marshal interface
func (s *Response) MarshalJSON() ([]byte, error) {
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
func (s *Response) UnmarshalJSON(data []byte) error {
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

// CreateCosiApp takes the ip:Port and returns an app for signing
func CreateCosiApp(ip string) *App {
	// create the public / private keys
	kp := config.NewKeyPair(network.Suite)
	entity := network.NewEntity(kp.Public, ip)
	host := sda.NewHost(entity, kp.Secret)

	return &App{
		host:    host,
		overlay: host.GetOverlay(),
	}
}

// ReadCosiApp reads the configuration from a file
func ReadCosiApp(cnf string) (*App, error) {
	h, err := sda.NewHostFromFile(cnf)
	if err != nil {
		return nil, err
	}
	return &App{
		host:    h,
		overlay: h.GetOverlay(),
	}, nil
}

// AddCosiApp registers the handler to the given host. This is used
// if another app wants to add the signing feature to the same host
func AddCosiApp(h *sda.Host) {
	ca := &App{
		host:    h,
		overlay: h.GetOverlay(),
	}
	ca.host.RegisterMessage(Request{}, ca.HandleCosiRequest)
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
	ca.host.RegisterMessage(Request{}, ca.HandleCosiRequest)
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

// handleCosiRequest will
// * register the entitylist
// * create a flat tree out of it with him being at the root
// * launch a CoSi protocol
// * wait for it to finish and send back the response to the client
func (ca *App) HandleCosiRequest(msg *network.Message) network.ProtocolMessage {
	dbg.Lvl2("Received client CosiRequest from", msg.From)
	empty := &Response{}
	cr, ok := msg.Msg.(Request)
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
	rchan := make(chan *Response)
	fn := func(chal, resp abstract.Secret) {
		response := &Response{
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
func Sign(r io.Reader, tomlFileName string) (*Response, error) {
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
func SignStatement(r io.Reader, el *sda.EntityList) (*Response, error) {

	// create a throw-away key pair:
	kp := config.NewKeyPair(network.Suite)

	// create a throw-away entity with an empty  address:
	e := network.NewEntity(kp.Public, "")
	client := network.NewSecureTCPHost(kp.Secret, e)
	msg, _ := crypto.HashStream(network.Suite.Hash(), r)
	req := &Request{
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
	pchan := make(chan Response)
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
		pchan <- packet.Msg.(Response)
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
	sig := &Response{}
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
func VerifySignatureHash(b []byte, sig *Response, el *sda.EntityList) error {
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
