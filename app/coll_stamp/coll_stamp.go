package coll_stamp
import (
	"github.com/dedis/cothority/lib/coconet"
	"strconv"
	"net"
	"errors"
	"github.com/dedis/cothority/proto/sign"
	"github.com/dedis/cothority/lib/config"
	"time"
	"fmt"
	"github.com/dedis/crypto/nist"
	"github.com/dedis/crypto/edwards/ed25519"
	"github.com/dedis/crypto/abstract"
	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/deploy"
)

func Run(app *config.AppConfig, proto *deploy.Config) {
	switch app.Mode{
	case "server":
		RunServer(app.Hostname, app.App, proto.Rounds, proto.RootWait, proto.Debug, app.TestConnect,
			proto.Failures, proto.RFail, proto.FFail, app.Logger, proto.Suite)
	case "client":
		RunClient(app.Server, proto.Nmsgs, app.Name, proto.Rate)
	}
}


func GetSuite(suite string) abstract.Suite {
	var s abstract.Suite
	switch {
	case suite == "nist256":
		s = nist.NewAES128SHA256P256()
	case suite == "nist512":
		s = nist.NewAES128SHA256QR512()
	case suite == "ed25519":
		s = ed25519.NewAES128SHA256Ed25519(true)
	default:
		s = nist.NewAES128SHA256P256()
	}
	return s
}


func RunServer(hostname, app string, rounds int, rootwait int, debug int, testConnect bool,
failureRate, rFail, fFail int, logger, suite string) {
	dbg.Lvl1(hostname, "Starting to run")
	if debug > 1 {
		sign.DEBUG = true
	}

	// fmt.Println("EXEC TIMESTAMPER: " + hostname)
	if hostname == "" {
		fmt.Println("hostname is empty")
		log.Fatal("no hostname given")
	}

	// load the configuration
	//dbg.Lvl2("loading configuration")
	var hc *config.HostConfig
	var err error
	s := GetSuite(suite)
	opts := config.ConfigOptions{ConnType: "tcp", Host: hostname, Suite: s}
	if failureRate > 0 || fFail > 0 {
		opts.Faulty = true
	}
	hc, err = config.LoadConfig("tree.json", opts)
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}

	for i := range hc.SNodes {
		// set FailureRates
		if failureRate > 0 {
			hc.SNodes[i].FailureRate = failureRate
		}
		// set root failures
		if rFail > 0 {
			hc.SNodes[i].FailAsRootEvery = rFail
		}
		// set follower failures
		// a follower fails on %ffail round with failureRate probability
		hc.SNodes[i].FailAsFollowerEvery = fFail
	}

	// run this specific host
	// dbg.Lvl3("RUNNING HOST CONFIG")
	err = hc.Run(app != "coll_sign", sign.MerkleTree, hostname)
	if err != nil {
		log.Fatal(err)
	}

	defer func(sn *sign.Node) {
		//log.Panicln("program has terminated:", hostname)
		dbg.Lvl1("Program timestamper has terminated:", hostname)
		sn.Close()
	}(hc.SNodes[0])

	stampers, _, err := RunTimestamper(hc, 0, hostname)
	// get rid of the hc information so it can be GC'ed
	hc = nil
	if err != nil {
		log.Fatal(err)
	}
	for _, s := range stampers {
		// only listen if this is the hostname specified
		if s.Name() == hostname {
			s.Logger = logger
			s.Hostname = hostname
			s.App = app
			if s.IsRoot(0) {
				dbg.Lvl1("Root timestamper at:", hostname, rounds, "Waiting: ", rootwait)
				// wait for the other nodes to get set up
				time.Sleep(time.Duration(rootwait) * time.Second)

				dbg.Lvl1("Starting root-round")
				s.Run("root", rounds)
				// dbg.Lvl2("\n\nROOT DONE\n\n")

			} else if !testConnect {
				dbg.Lvl1("Running regular timestamper on:", hostname)
				s.Run("regular", rounds)
				// dbg.Lvl1("\n\nREGULAR DONE\n\n")
			} else {
				// testing connection
				dbg.Lvl1("Running connection-test on:", hostname)
				s.Run("test_connect", rounds)
			}
		}
	}
}

// run each host in hostnameSlice with the number of clients given
func RunTimestamper(hc *config.HostConfig, nclients int, hostnameSlice ...string) ([]*Server, []*Client, error) {
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

	Clients := make([]*Client, 0, len(hostnames) * nclients)
	// for each client in
	stampers := make([]*Server, 0, len(hostnames))
	for _, sn := range hc.SNodes {
		if _, ok := hostnames[sn.Name()]; !ok {
			log.Errorln("signing node not in hostnmaes")
			continue
		}
		stampers = append(stampers, NewServer(sn))
		if hc.Dir == nil {
			//dbg.Lvl3("listening for clients")
			stampers[len(stampers) - 1].Listen()
		}
	}
	//dbg.Lvl3("stampers:", stampers)
	clientsLists := make([][]*Client, len(hc.SNodes[1:]))
	for i, s := range stampers[1:] {
		// cant assume the type of connection
		clients := make([]*Client, nclients)

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
		hp := net.JoinHostPort(h, strconv.Itoa(pn + 1))
		//dbg.Lvl3("client connecting to:", hp)

		for j := range clients {
			clients[j] = NewClient("client" + strconv.Itoa((i - 1) * len(stampers) + j))
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
