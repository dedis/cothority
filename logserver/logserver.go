package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/ineiti/cothorities/helpers/cliutils"
	"github.com/ineiti/cothorities/helpers/graphs"

	"golang.org/x/net/websocket"
)

var addr, hosts, depth, bf, hpn, nmsgs, master, rate string
var homePage *template.Template

type Home struct {
	LogServer        string
	Hosts            string
	Depth            string
	BranchingFactor  string
	HostsPerNode     string
	NumberOfMessages string
	Rate             string
}

var Log Logger

func init() {
	flag.StringVar(&addr, "addr", "", "the address of the logging server")
	flag.StringVar(&hosts, "hosts", "", "number of hosts in config file")
	flag.StringVar(&depth, "depth", "", "the depth of the tree")
	flag.StringVar(&bf, "bf", "", "the branching factor of the tree")
	flag.StringVar(&hpn, "hpn", "", "number of hosts per node")
	flag.StringVar(&rate, "rate", "", "the rate of messages")
	flag.StringVar(&nmsgs, "nmsgs", "", "number of messages per round")
	flag.StringVar(&master, "master", "", "address of the master of this node")

	Log = Logger{
		Slock: sync.RWMutex{},
		Sox:   make(map[*websocket.Conn]bool),
		Mlock: sync.RWMutex{},
		Msgs:  make([][]byte, 0, 100000),
	}
	rand.Seed(42)
}

type Logger struct {
	Slock sync.RWMutex
	Sox   map[*websocket.Conn]bool

	Mlock sync.RWMutex
	Msgs  [][]byte
	End   int
}

// keep a list of websockets that people are listening on

// keep a log of messages received

func logEntryHandler(ws *websocket.Conn) {
	var data []byte
	err := websocket.Message.Receive(ws, &data)
	for err == nil {
		if !isMaster {
			websocket.Message.Send(wsmaster, append([]byte(fmt.Sprintf("IP : %s", ws.RemoteAddr().String())), data...))
		} else {
			Log.Mlock.Lock()
			Log.Msgs = append(Log.Msgs, data)
			Log.End += 1
			Log.Mlock.Unlock()
		}
		err = websocket.Message.Receive(ws, &data)
	}
	log.Println("log server client error:", err)
}

f unc logHandler(ws *websocket.Conn) {
	log.Println("LOG HANDLER")
	i := 0
	for {
		Log.Mlock.RLock()
		end := Log.End
		Log.Mlock.RUnlock()
		if i >= end {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		Log.Mlock.RLock()
		msg := Log.Msgs[i]
		Log.Mlock.RUnlock()
		_, err := ws.Write(msg)
		if err != nil {
			log.Println("unable to write to log websocket")
			return
		}

		i++
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		log.Println("home handler is handling non-home request")
		http.NotFound(w, r)
		return
	}
	log.Println("HOME HANDLER: ", r.URL)
	host := r.Host
	// fmt.Println(host)
	ws := "ws://" + host + "/log"

	err := homePage.Execute(w, Home{ws, hosts, depth, bf, hpn, nmsgs, rate})
	if err != nil {
		panic(err)
		log.Fatal(err)
	}
}

func NewReverseProxy(target *url.URL) *httputil.ReverseProxy {
	director := func(r *http.Request) {
		r.URL.Scheme = target.Scheme
		r.URL.Host = target.Host

		// get rid of the (/d/short_name)/debug of the url path requested
		//  --> long_name/debug
		pathComp := strings.Split(r.URL.Path, "/")
		// remove the first two components /d/short_name
		pathComp = pathComp[3:]
		r.URL.Path = target.Path + "/" + strings.Join(pathComp, "/")
		log.Println("redirected to: ", r.URL.String())
	}
	log.Println("setup reverse proxy for destination url:", target.Host, target.Path)
	return &httputil.ReverseProxy{Director: director}
}

func proxyDebugHandler(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("proxy serving request for: ", r.URL)
		p.ServeHTTP(w, r)
	}
}

var timesSeen = make(map[string]int)

func reverseProxy(server string) {
	remote, err := url.Parse("http://" + server)
	if err != nil {
		panic(err)
	}
	// get the short name of this remote
	s := strings.Split(server, ".")[0]
	short := s + "-" + strconv.Itoa(timesSeen[s])
	timesSeen[s] = timesSeen[s] + 1

	// setup a reverse proxy s.t.
	//
	// "/d/short_name/debug" -> http://server/debug
	//
	proxy := NewReverseProxy(remote)

	//log.Println("setup proxy for: /d/"+short+"/", " it points to : "+server)
	// register the reverse proxy forwarding for this server
	http.HandleFunc("/d/"+short+"/", proxyDebugHandler(proxy))
}

func getDebugServers() []string {
	// read in physical nodes and virtual nodes into global variables
	phys, err := cliutils.ReadLines("phys.txt")
	if err != nil {
		log.Errorln(err)
	}
	virt, err := cliutils.ReadLines("virt.txt")
	if err != nil {
		log.Errorln(err)
	}

	// create mapping from virtual nodes to physical nodes
	vpmap := make(map[string]string)
	for i := range phys {
		vpmap[virt[i]] = phys[i]
	}

	// now read in the hosttree to get a list of servers
	cfg, e := ioutil.ReadFile("cfg.json")
	if e != nil {
		log.Fatal("Error Reading Configuration File:", e)
	}

	var tree graphs.Tree
	json.Unmarshal(cfg, &tree)

	debugServers := make([]string, 0, len(virt))
	tree.TraverseTree(func(t *graphs.Tree) {
		h, p, err := net.SplitHostPort(t.Name)
		if err != nil {
			log.Fatal("improperly formatted hostport:", err)
		}
		pn, _ := strconv.Atoi(p)
		s := net.JoinHostPort(vpmap[h], strconv.Itoa(pn+2))
		debugServers = append(debugServers, s)
	})
	return debugServers
}

var isMaster bool
var wsmaster *websocket.Conn

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// read in from flags the port I should be listening on
	flag.Parse()
	if master == "" {
		isMaster = true
	}
	var role string
	if isMaster {
		role = "Master"
	} else {
		role = "Servent"
	}
	log.Println(fmt.Sprintf("running logserver %s  with nmsgs %d branching factor: %d", role, nmsgs, bf))
	if isMaster {
		var err error
		homePage, err = template.ParseFiles("home.html")
		if err != nil {
			log.Fatal("unable to parse home.html", err)
		}

		debugServers := getDebugServers()

		for _, s := range debugServers {
			reverseProxy(s)
		}

		log.Println(fmt.Sprintf("Log server %s running at : %s", role, addr))
		// /bower_components/Chart.js/Chart.min.js
		http.HandleFunc("/", homeHandler)
		fs := http.FileServer(http.Dir("bower_components/"))
		http.Handle("/bower_components/", http.StripPrefix("/bower_components/", fs))
	} else {
	retry:
		var err error
		origin := "http://localhost/"
		url := "ws://" + master + "/_log"
		wsmaster, err = websocket.Dial(url, "", origin)
		if err != nil {
			time.Sleep(time.Second)
			goto retry
		}
		log.Println(fmt.Sprintf("Log server %s running at : %s ", role, addr))
	}
	http.Handle("/_log", websocket.Handler(logEntryHandler))
	http.Handle("/log", websocket.Handler(logHandler))
	log.Fatalln("ERROR: ", http.ListenAndServe(addr, nil))
	// now combine that port
}
