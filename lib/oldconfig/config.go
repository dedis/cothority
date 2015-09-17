package oldconfig

import (
	"bytes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/proto/sign"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	//	"github.com/dedis/crypto/nist"
	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/app/stamp"
)

/*
Example configuration file.
file format: json

conn: indicates what protocol should be used
	by default it uses the "tcp" protocol
	"tcp": uses TcpConn for communications
	"goroutine": uses GoConn for communications [default]

ex.json
{
	conn: "tcp"
	hosts: ["host1", "host2", "host3"],
	tree: {name: host1,
		   children: [
		     {name: host2,
			  children: [{name: host3}, {name: host4}]}
			 {name: host5,
			  children: [{name: host6}]}}
}
*/

type ConfigFile struct {
	Conn  string   `json:"conn,omitempty"`
	Hosts []string `json:"hosts"`
	Tree  *Node    `json:"tree"`
}

type JSONPoint json.RawMessage

type Node struct {
	Name     string  `json:"name"`
	PriKey   string  `json:"prikey,omitempty"`
	PubKey   string  `json:"pubkey,omitempty"`
	Children []*Node `json:"children,omitempty"`
}

// HostConfig stores all of the relevant information of the configuration file.
type HostConfig struct {
	SNodes []*sign.Node          // an array of signing nodes
	Hosts  map[string]*sign.Node // maps hostname to host
	Dir    *coconet.GoDirectory  // the directory mapping hostnames to goPeers
}

func (hc *HostConfig) Verify() error {
	// root := hc.SNodes[0]
	// traverseTree(root, hc, publicKeyCheck)
	fmt.Println("tree verified")
	return nil
}

func traverseTree(p *sign.Node,
	hc *HostConfig,
	f func(*sign.Node, *HostConfig) error) error {
	if err := f(p, hc); err != nil {
		return err
	}
	for _, cn := range p.Children(0) {
		c := hc.Hosts[cn.Name()]
		err := traverseTree(c, hc, f)
		if err != nil {
			return err
		}
	}
	return nil
}

func (hc *HostConfig) String() string {
	b := bytes.NewBuffer([]byte{})

	// write the hosts
	b.WriteString("{\"hosts\": [")
	for i, sn := range hc.SNodes {
		if i != 0 {
			b.WriteString(", ")
		}
		b.WriteString("\"" + sn.Name() + "\"")
	}
	b.WriteString("],")

	// write the tree structure
	b.WriteString("\"tree\": ")
	if len(hc.SNodes) != 0 {
		root := hc.SNodes[0]
		writeHC(b, hc, root)
	} else {
		b.WriteString("{}")
	}
	b.WriteString("}\n")

	// format the resulting JSON for readability
	bformatted := bytes.NewBuffer([]byte{})
	err := json.Indent(bformatted, b.Bytes(), "", "\t")
	if err != nil {
		fmt.Println(string(b.Bytes()))
		fmt.Println("ERROR: ", err)
	}

	return string(bformatted.Bytes())
}

func writeHC(b *bytes.Buffer, hc *HostConfig, p *sign.Node) error {
	// Node{name, pubkey, x_hat, children}
	if p == nil {
		return errors.New("node does not exist")
	}
	prk, _ := p.PrivKey.MarshalBinary()
	pbk, _ := p.PubKey.MarshalBinary()
	fmt.Fprint(b, "{\"name\":", "\""+p.Name()+"\",")
	fmt.Fprint(b, "\"prikey\":", "\""+string(hex.EncodeToString(prk))+"\",")
	fmt.Fprint(b, "\"pubkey\":", "\""+string(hex.EncodeToString(pbk))+"\",")

	// recursively format children
	fmt.Fprint(b, "\"children\":[")
	i := 0
	for _, n := range p.Children(0) {
		if i != 0 {
			b.WriteString(", ")
		}
		c := hc.Hosts[n.Name()]
		err := writeHC(b, hc, c)
		if err != nil {
			b.WriteString("\"" + n.Name() + "\"")
		}
		i++
	}
	fmt.Fprint(b, "]}")
	return nil
}

// NewHostConfig creates a new host configuration that can be populated with
// hosts.
func NewHostConfig() *HostConfig {
	return &HostConfig{SNodes: make([]*sign.Node, 0), Hosts: make(map[string]*sign.Node), Dir: coconet.NewGoDirectory()}
}

type ConnType int

const (
	GoC ConnType = iota
	TcpC
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ConstructTree does a depth-first construction of the tree specified in the
// config file. ConstructTree must be called AFTER populating the HostConfig with
// ALL the possible hosts.
func ConstructTree(
	n *Node,
	hc *HostConfig,
	parent string,
	suite abstract.Suite,
	rand cipher.Stream,
	hosts map[string]coconet.Host,
	nameToAddr map[string]string,
	opts ConfigOptions) (int, error) {
	// passes up its X_hat, and/or an error

	// get the name associated with this address
	name, ok := nameToAddr[n.Name]
	if !ok {
		fmt.Println("unknown name in address book:", n.Name)
		return 0, errors.New("unknown name in address book")
	}

	// generate indicates whether we should generate the signing
	// node for this hostname
	generate := opts.Host == "" || opts.Host == name

	// check to make sure the this hostname is in the tree
	// it can be backed by a nil pointer
	h, ok := hosts[name]
	if !ok {
		fmt.Println("unknown host in tree:", name)
		return 0, errors.New("unknown host in tree")
	}

	var prikey abstract.Secret
	var pubkey abstract.Point
	var sn *sign.Node

	// if the JSON holds the fields field is set load from there
	if len(n.PubKey) != 0 {
		// dbg.Lvl3("decoding point")
		encoded, err := hex.DecodeString(string(n.PubKey))
		if err != nil {
			log.Error("failed to decode hex from encoded")
			return 0, err
		}
		pubkey = suite.Point()
		err = pubkey.UnmarshalBinary(encoded)
		if err != nil {
			log.Error("failed to decode point from hex")
			return 0, err
		}
		// dbg.Lvl3("decoding point")
		encoded, err = hex.DecodeString(string(n.PriKey))
		if err != nil {
			log.Error("failed to decode hex from encoded")
			return 0, err
		}
		prikey = suite.Secret()
		err = prikey.UnmarshalBinary(encoded)
		if err != nil {
			log.Error("failed to decode point from hex")
			return 0, err
		}
	}
	if generate {
		if prikey != nil {
			// if we have been given a private key load that
			aux := sign.NewKeyedNode(h, suite, prikey)
			aux.GenSetPool()
			hc.SNodes = append(hc.SNodes, aux)
			h.SetPubKey(pubkey)
		} else {
			// otherwise generate a random new one
			sn := sign.NewNode(h, suite, rand)
			sn.GenSetPool()
			hc.SNodes = append(hc.SNodes, sn)
			h.SetPubKey(sn.PubKey)
		}
		sn = hc.SNodes[len(hc.SNodes)-1]
		hc.Hosts[name] = sn
		if prikey == nil {
			prikey = sn.PrivKey
			pubkey = sn.PubKey
		}
		// dbg.Lvl3("pubkey:", sn.PubKey)
		// dbg.Lvl3("given: ", pubkey)
	}
	// if the parent of this call is empty then this must be the root node
	if parent != "" && generate {
		h.AddParent(0, parent)
	}
	// dbg.Lvl3("name: ", n.Name)
	// dbg.Lvl3("prikey: ", prikey)
	// dbg.Lvl3("pubkey: ", pubkey)
	height := 0
	for _, c := range n.Children {
		// connect this node to its children
		cname, ok := nameToAddr[c.Name]
		if !ok {
			fmt.Println("unknown name in address book:", n.Name)
			return 0, errors.New("unknown name in address book")
		}

		if generate {
			h.AddChildren(0, cname)
		}

		// recursively construct the children
		dbg.Lvl3("ConstructTree:", h, suite, rand, hosts, nameToAddr, opts)
		h, err := ConstructTree(c, hc, name, suite, rand, hosts, nameToAddr, opts)
		if err != nil {
			return 0, err
		}
		height = max(h+1, height)
		// if generating all csn will be availible
	}
	if generate {
		sn.Height = height
	}
	// dbg.Lvl3("name: ", n.Name)
	// dbg.Lvl3("final x_hat: ", x_hat)
	// dbg.Lvl3("final pubkey: ", pubkey)
	return height, nil
}

var ipv4Reg = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)
var ipv4host = "NONE"

// getAddress gets the localhosts IPv4 address.
func GetAddress() (string, error) {
	name, err := os.Hostname()
	if err != nil {
		log.Error("Error Resolving Hostname:", err)
		return "", err
	}

	if ipv4host == "NONE" {
		as, err := net.LookupHost(name)
		if err != nil {
			return "", err
		}

		addr := ""

		for _, a := range as {
			dbg.Lvl3("a = %+v", a)
			if ipv4Reg.MatchString(a) {
				dbg.Lvl3("matches")
				addr = a
			}
		}

		if addr == "" {
			err = errors.New("No IPv4 Address for Hostname")
		}
		return addr, err
	}
	return ipv4host, nil
}

var StartConfigPort = 9000

type ConfigOptions struct {
	ConnType  string         // "go", tcp"
	Hostnames []string       // if not nil replace hostnames with these
	GenHosts  bool           // if true generate random hostnames (all tcp)
	Host      string         // hostname to load into memory: "" for all
	Port      string         // if specified rewrites all ports to be this
	Faulty    bool           // if true, use FaultyHost wrapper around Hosts
	Suite     abstract.Suite // suite to use for Hosts
}

// TODO: if in tcp mode associate each hostname in the file with a different
// port. Get the remote address of this computer to combine with those for the
// complete hostnames to be used by the hosts.
func LoadJSON(file []byte, optsSlice ...ConfigOptions) (*HostConfig, error) {
	opts := ConfigOptions{}
	if len(optsSlice) > 0 {
		opts = optsSlice[0]
	}

	hc := NewHostConfig()
	var cf ConfigFile
	err := json.Unmarshal(file, &cf)
	if err != nil {
		return hc, err
	}
	connT := GoC
	if cf.Conn == "tcp" {
		connT = TcpC
	}

	// options override file
	if opts.ConnType == "tcp" {
		connT = TcpC
	}

	dir := hc.Dir
	hosts := make(map[string]coconet.Host)
	nameToAddr := make(map[string]string)

	if connT == GoC {
		for _, h := range cf.Hosts {
			if _, ok := hc.Hosts[h]; !ok {
				nameToAddr[h] = h
				// it doesn't make sense to only make 1 go host
				if opts.Faulty == true {
					gohost := coconet.NewGoHost(h, dir)
					hosts[h] = coconet.NewFaultyHost(gohost)
				} else {
					hosts[h] = coconet.NewGoHost(h, dir)
				}
			}
		}

	} else if connT == TcpC {
		localAddr := ""

		if opts.GenHosts {
			localAddr, err = GetAddress()
			if err != nil {
				return nil, err
			}
		}

		for i, h := range cf.Hosts {

			addr := h
			if opts.GenHosts {
				p := strconv.Itoa(StartConfigPort)
				addr = localAddr + ":" + p
				//dbg.Lvl3("created new host address: ", addr)
				StartConfigPort += 10
			} else if opts.Port != "" {
				dbg.Lvl3("attempting to rewrite port: ", opts.Port)
				// if the port has been specified change the port
				hostport := strings.Split(addr, ":")
				dbg.Lvl3(hostport)
				if len(hostport) == 2 {
					addr = hostport[0] + ":" + opts.Port
				}
				dbg.Lvl3(addr)
			} else if len(opts.Hostnames) != 0 {
				addr = opts.Hostnames[i]
			}

			nameToAddr[h] = addr
			// add to the hosts list if we havent added it before
			if _, ok := hc.Hosts[addr]; !ok {
				// only create the tcp hosts requested
				if opts.Host == "" || opts.Host == addr {
					if opts.Faulty == true {
						tcpHost := coconet.NewTCPHost(addr)
						hosts[addr] = coconet.NewFaultyHost(tcpHost)
					} else {
						hosts[addr] = coconet.NewTCPHost(addr)
					}
				} else {
					hosts[addr] = nil // it is there but not backed
				}
			}
		}
	}
	suite := edwards.NewAES128SHA256Ed25519(true)
	//suite := nist.NewAES128SHA256P256()
	if opts.Suite != nil {
		suite = opts.Suite
	}
	rand := suite.Cipher([]byte("example"))
	//fmt.Println("hosts", hosts)
	_, err = ConstructTree(cf.Tree, hc, "", suite, rand, hosts, nameToAddr, opts)
	if connT != GoC {
		hc.Dir = nil
	}

	dbg.Lvl3("IN LOAD JSON")
	// add a hostlist to each of the signing nodes
	var hostList []string
	for h := range hosts {
		hostList = append(hostList, h)
	}
	for _, sn := range hc.SNodes {
		sn.HostList = make([]string, len(hostList))
		sortable := sort.StringSlice(hostList)
		sortable.Sort()
		copy(sn.HostList, sortable)
		// set host list on view 0
		//dbg.Lvl3("in config hostlist", sn.HostList)
		sn.SetHostList(0, sn.HostList)
	}

	return hc, err
}

// run the given hostnames
func (hc *HostConfig) Run(stamper bool, signType sign.Type, hostnameSlice ...string) error {
	hostnames := make(map[string]*sign.Node)
	if hostnameSlice == nil {
		hostnames = hc.Hosts
	} else {
		for _, h := range hostnameSlice {
			sn, ok := hc.Hosts[h]
			if !ok {
				return errors.New("hostname given not in config file:" + h)
			}
			hostnames[h] = sn
		}
	}
	// set all hosts to be listening
	for _, sn := range hostnames {
		sn.Type = signType
		sn.Host.Listen()
	}

	for h, sn := range hostnames {
		var err error
		// exponential backoff for attempting to connect to parent
		startTime := time.Duration(200)
		maxTime := time.Duration(2000)
		for i := 0; i < 2000; i++ {
			dbg.Lvl3("Attempting to connect to parent", h)
			// the host should connect with the parent
			err = sn.Connect(0)
			if err == nil {
				// log.Infoln("hostconfig: connected to parent:")
				break
			}

			time.Sleep(startTime * time.Millisecond)
			startTime *= 2
			if startTime > maxTime {
				startTime = maxTime
			}
		}
		dbg.Lvl3(fmt.Sprintf("Successfully connected to parent %s", h))
		if err != nil {
			log.Fatal(fmt.Sprintf("%s failed to connect to parent"), h)
			//return errors.New("failed to connect")
		}
	}

	// need to make sure network connections are setup properly first
	// wait for a little bit for connections to establish fully
	// get rid of waits they hide true bugs
	// time.Sleep(1000 * time.Millisecond)
	if !stamper {
		for _, sn := range hostnames {
			go sn.Listen()
		}
	}
	return nil
}

// run each host in hostnameSlice with the number of clients given
func (hc *HostConfig) RunTimestamper(nclients int, hostnameSlice ...string) ([]*stamp.Server, []*stamp.Client, error) {
	dbg.Lvl3("RunTimestamper")
	hostnames := make(map[string]*sign.Node)
	// make a list of hostnames we want to run
	if hostnameSlice == nil {
		hostnames = hc.Hosts
	} else {
		for _, h := range hostnameSlice {
			sn, ok := hc.Hosts[h]
			if !ok {
				return nil, nil, errors.New("hostname given not in config file:" + h)
			}
			hostnames[h] = sn
		}
	}

	Clients := make([]*stamp.Client, 0, len(hostnames)*nclients)
	// for each client in
	stampers := make([]*stamp.Server, 0, len(hostnames))
	for _, sn := range hc.SNodes {
		if _, ok := hostnames[sn.Name()]; !ok {
			log.Errorln("signing node not in hostnmaes")
			continue
		}
		stampers = append(stampers, stamp.NewServer(sn))
		if hc.Dir == nil {
			//dbg.Lvl3("listening for clients")
			stampers[len(stampers)-1].Listen()
		}
	}
	//dbg.Lvl3("stampers:", stampers)
	clientsLists := make([][]*stamp.Client, len(hc.SNodes[1:]))
	for i, s := range stampers[1:] {
		// cant assume the type of connection
		clients := make([]*stamp.Client, nclients)

		h, p, err := net.SplitHostPort(s.Name())
		if hc.Dir != nil {
			h = s.Name()
		} else if err != nil {
			log.Fatal("RunTimestamper: bad Tcp host")
		}
		pn, err := strconv.Atoi(p)
		if hc.Dir != nil {
			pn = 0
		} else if err != nil {
			log.Fatal("port is not valid integer")
		}
		hp := net.JoinHostPort(h, strconv.Itoa(pn+1))
		//dbg.Lvl3("client connecting to:", hp)

		for j := range clients {
			clients[j] = stamp.NewClient("client" + strconv.Itoa((i-1)*len(stampers)+j))
			var c coconet.Conn

			// if we are using tcp connections
			if hc.Dir == nil {
				// the timestamp server serves at the old port + 1
				dbg.Lvl3("new tcp conn")
				c = coconet.NewTCPConn(hp)
			} else {
				dbg.Lvl3("new go conn")
				c, _ = coconet.NewGoConn(hc.Dir, clients[j].Name(), s.Name())
				stoc, _ := coconet.NewGoConn(hc.Dir, s.Name(), clients[j].Name())
				s.Clients[clients[j].Name()] = stoc
			}
			// connect to the server from the client
			clients[j].AddServer(s.Name(), c)
			//clients[j].Sns[s.Name()] = c
			//clients[j].Connect()
		}
		Clients = append(Clients, clients...)
		clientsLists[i] = clients
	}

	return stampers, Clients, nil
}

// LoadConfig loads a configuration file in the format specified above. It
// populates a HostConfig with HostNode Hosts and goPeer Peers.
func LoadConfig(fname string, opts ...ConfigOptions) (*HostConfig, error) {
	file, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	return LoadJSON(file, opts...)
}
