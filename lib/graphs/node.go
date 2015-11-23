package graphs

import (
	"bytes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sign"

	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/crypto/abstract"
	"sort"
	"strconv"
	"strings"
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

type JSONPoint json.RawMessage

// HostConfig stores all of the relevant information of the configuration file.
type HostConfig struct {
	SNodes []*sign.Node          // an array of signing nodes
	Hosts  map[string]*sign.Node // maps hostname to host
	Dir    *coconet.GoDirectory  // the directory mapping hostnames to goPeers
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
		dbg.Lvl3(string(b.Bytes()))
		dbg.Lvl3("ERROR: ", err)
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
	fmt.Fprint(b, "{\"name\":", "\"" + p.Name() + "\",")
	fmt.Fprint(b, "\"prikey\":", "\"" + string(hex.EncodeToString(prk)) + "\",")
	fmt.Fprint(b, "\"pubkey\":", "\"" + string(hex.EncodeToString(pbk)) + "\",")

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
node *Tree,
hc *HostConfig,
parent string,
suite abstract.Suite,
rand cipher.Stream,
hosts map[string]coconet.Host,
nameToAddr map[string]string,
opts ConfigOptions) (int, error) {
	// passes up its X_hat, and/or an error

	// get the name associated with this address
	name, ok := nameToAddr[node.Name]
	if !ok {
		dbg.Lvl3("unknown name in address book:", node.Name)
		return 0, errors.New("unknown name in address book")
	}

	// generate indicates whether we should generate the signing
	// node for this hostname
	//dbg.Lvl4("opts.Host - name", opts.Host, name)
	generate := opts.Host == "" || opts.Host == name

	// check to make sure the this hostname is in the tree
	// it can be backed by a nil pointer
	h, ok := hosts[name]
	if !ok {
		dbg.Lvl3("unknown host in tree:", name)
		return 0, errors.New("unknown host in tree")
	}

	var prikey abstract.Secret
	var pubkey abstract.Point
	var sn *sign.Node

	// if the JSON holds the fields field is set load from there
	if len(node.PubKey) != 0 {
		// dbg.Lvl4("decoding point")
		encoded, err := hex.DecodeString(string(node.PubKey))
		if err != nil {
			dbg.Error("failed to decode hex from encoded")
			return 0, err
		}
		pubkey = suite.Point()
		err = pubkey.UnmarshalBinary(encoded)
		if err != nil {
			dbg.Error("failed to decode point from hex")
			return 0, err
		}
	}
	if len(node.PriKey) != 0 {
		// dbg.Lvl4("decoding point")
		encoded, err := hex.DecodeString(string(node.PriKey))
		if err != nil {
			dbg.Error("failed to decode hex from encoded")
			return 0, err
		}
		prikey = suite.Secret()
		err = prikey.UnmarshalBinary(encoded)
		if err != nil {
			dbg.Error("failed to decode point from hex")
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
		sn = hc.SNodes[len(hc.SNodes) - 1]
		hc.Hosts[name] = sn
		if prikey == nil {
			prikey = sn.PrivKey
			pubkey = sn.PubKey
		}
		// dbg.Lvl4("pubkey:", sn.PubKey)
		// dbg.Lvl4("given: ", pubkey)
	}
	// if the parent of this call is empty then this must be the root node
	if parent != "" && generate {
		//dbg.Lvl5("Adding parent for", h.Name(), "to", parent)
		h.AddParent(0, parent)
	}

	// dbg.Lvl4("name: ", n.Name)
	// dbg.Lvl4("prikey: ", prikey)
	// dbg.Lvl4("pubkey: ", pubkey)
	height := 0
	for _, c := range node.Children {
		// connect this node to its children
		cname, ok := nameToAddr[c.Name]
		if !ok {
			dbg.Lvl3("unknown name in address book:", node.Name)
			return 0, errors.New("unknown name in address book")
		}

		if generate {
			//dbg.Lvl4("Adding children for", h.Name())
			h.AddChildren(0, cname)
		}

		// recursively construct the children
		// Don't enable this debugging-line - it will make the constructtree VERY slow
		//dbg.Lvl5("ConstructTree:", h, suite, rand, hosts, nameToAddr, opts)
		h, err := ConstructTree(c, hc, name, suite, rand, hosts, nameToAddr, opts)
		if err != nil {
			return 0, err
		}
		height = max(h + 1, height)
		// if generating all csn will be availible
	}
	if generate {
		sn.Height = height
	}

	// dbg.Lvl4("name: ", n.Name)
	// dbg.Lvl4("final x_hat: ", x_hat)
	// dbg.Lvl4("final pubkey: ", pubkey)
	return height, nil
}

var ipv4Reg = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)
var ipv4host = "NONE"

// getAddress gets the localhosts IPv4 address.
func GetAddress() (string, error) {
	name, err := os.Hostname()
	if err != nil {
		dbg.Error("Error Resolving Hostname:", err)
		return "", err
	}

	if ipv4host == "NONE" {
		as, err := net.LookupHost(name)
		if err != nil {
			return "", err
		}

		addr := ""

		for _, a := range as {
			dbg.Lvl4("a = %+v", a)
			if ipv4Reg.MatchString(a) {
				dbg.Lvl4("matches")
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
	NoTree    bool           // bool flag to tell wether we want to construct
							 // the tree or not. Setting this to false will
							 // construct the tree. True will not.
}

// run the given hostnames
func (hc *HostConfig) Run(stamper bool, signType sign.Type, hostname string) error {
	dbg.Lvl3(hc.Hosts, "going to connect everything for", hostname)
	node := hc.Hosts[hostname]

	node.Type = signType
	dbg.Lvl3("Listening on", node.Host)
	node.Host.Listen()

	var err error
	// exponential backoff for attempting to connect to parent
	startTime := time.Duration(200)
	maxTime := time.Duration(2000)
	for i := 0; i < 2000; i++ {
		dbg.Lvl3(hostname, "attempting to connect to parent")
		// the host should connect with the parent
		err = node.Connect(0)
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
	if err != nil {
		dbg.Fatal(fmt.Sprintf("%s failed to connect to parent"), hostname)
		//return errors.New("failed to connect")
	} else {
		dbg.Lvl3(fmt.Sprintf("Successfully connected to parent %s", hostname))
	}

	if !stamper {
		// This will call the dispatcher in collectiveSigning for every request
		dbg.Lvl4("Starting to listen for incoming stamp-requests on", hostname)
		node.Listen()
	}

	return nil
}

// TODO: if in tcp mode associate each hostname in the file with a different
// port. Get the remote address of this computer to combine with those for the
// complete hostnames to be used by the hosts.
// LoadConfig loads a configuration file in the format specified above. It
// populates a HostConfig with HostNode Hosts and goPeer Peers.
func LoadConfig(appHosts []string, appTree *Tree, suite abstract.Suite, optsSlice ...ConfigOptions) (*HostConfig, error) {
	opts := ConfigOptions{}
	if len(optsSlice) > 0 {
		opts = optsSlice[0]
	}

	hc := NewHostConfig()

	connT := GoC
	// options override file
	if opts.ConnType == "tcp" {
		connT = TcpC
	}

	dir := hc.Dir
	hosts := make(map[string]coconet.Host)
	nameToAddr := make(map[string]string)

	if connT == GoC {
		for _, h := range appHosts {
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
			var err error
			localAddr, err = GetAddress()
			if err != nil {
				return nil, err
			}
		}

		for i, h := range appHosts {

			addr := h
			if opts.GenHosts {
				p := strconv.Itoa(StartConfigPort)
				addr = localAddr + ":" + p
				//dbg.Lvl4("created new host address: ", addr)
				StartConfigPort += 10
			} else if opts.Port != "" {
				dbg.Lvl4("attempting to rewrite port: ", opts.Port)
				// if the port has been specified change the port
				hostport := strings.Split(addr, ":")
				dbg.Lvl4(hostport)
				if len(hostport) == 2 {
					addr = hostport[0] + ":" + opts.Port
				}
				dbg.Lvl4(addr)
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

	//suite := edwards.NewAES128SHA256Ed25519(true)
	//suite := nist.NewAES128SHA256P256()
	rand := suite.Cipher([]byte("example"))
	//dbg.Lvl3("hosts", hosts)
	// default value = false
	start := time.Now()
	if opts.NoTree == false {
		_, err := ConstructTree(appTree, hc, "", suite, rand, hosts, nameToAddr, opts)
		if err != nil {
			dbg.Fatal("Couldn't construct tree:", err)
		}
	}
	dbg.Lvl3("Timing for ConstructTree", time.Since(start))
	if connT != GoC {
		hc.Dir = nil
	}

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
		//dbg.Lvl4("in config hostlist", sn.HostList)
		sn.SetHostList(0, sn.HostList)
	}

	return hc, nil
}
