// Status takes in a file containing a list of servers and returns the status
// reports of all of the servers.  A status is a list of connections and
// packets sent and received for each server in the file.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"

	cli "github.com/urfave/cli"
	status "go.dedis.ch/cothority/v3/status/service"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/app"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

const prometheusTemplate = `# HELP voting_conodes_status voting global conodes status: X is the number of conodes; X >= 0.66 OK, 0 < X < 0.66 KO
# TYPE voting_conodes_status gauge
voting_conodes_status{} {{ .Connectivity }}

# HELP voting_conodes_status_timestamp timestamp in seconds since epoch denoting last probe time
# TYPE voting_conodes_status_timestamp counter
voting_conodes_status_timestamp{} {{ .LastCheckedAt }}

# HELP probe error: {{ .ProbeError }}

# HELP voting_conode_status voting conode status: 1=OK, 0=KO
# TYPE voting_conode_status gauge
{{- range $conode, $status := .Matrix -}}
{{ if eq $conode $.Self }}
voting_conode_status{conode="{{ $conode}}", description="{{ $status.Description }}", critical="true"} {{ $status.Status -}}
{{ else }}
voting_conode_status{conode="{{ $conode}}", description="{{ $status.Description }}", critical="false"} {{ $status.Status -}}
{{ end }}
{{- end -}}
`

func main() {
	app := cli.NewApp()
	app.Name = "Status"
	app.Usage = "Get and print status of all servers of a file."

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "group, g",
			Value: "group.toml",
			Usage: "Cothority group definition in `FILE.toml`",
		},
		cli.StringFlag{
			Name:  "host",
			Usage: "Request information about this host",
		},
		cli.StringFlag{
			Name:  "format, f",
			Value: "txt",
			Usage: "Output format: \"txt\" (default) or \"json\".",
		},
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: `integer`: 1 for terse, 5 for maximal",
		},
		cli.BoolFlag{
			Name: "connectivity, c",
		},
	}
	app.Commands = cli.Commands{
		{
			Name:      "connectivity",
			Usage:     "if given, will verify connectivity of all nodes between themselves",
			Aliases:   []string{"c"},
			ArgsUsage: "group.toml private.toml",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "findFaulty, f",
					Usage: "it tries to find a list of nodes that can communicate with each other",
				},
				cli.StringFlag{
					Name:  "timeout, to",
					Usage: "timeout in ms to wait if a set of nodes is connected",
					Value: "1s",
				},
			},
			Subcommands: []cli.Command{
				{
					Name: "serve",
					Usage: "exposes the connectivity check results on HTTP. Assumes -findFaulty\n   " +
						"Note: The HTTP server does not rate limit connections. Administrators are advised\n   " +
						"to rate-limit connections or firewall the HTTP port themselves.",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "format, fo",
							Value: "prometheus",
							Usage: "Output format",
						},
						cli.StringFlag{
							Name:  "endpoint, e",
							Value: "/connectivity",
							Usage: "HTTP endpoint to serve the response at",
						},
						cli.IntFlag{
							Name:  "port, p",
							Value: 9000,
							Usage: "Port to listen on",
						},
						cli.StringFlag{
							Name:  "interval, i",
							Value: "1m",
							Usage: "Time interval to run connectivity checks in",
						},
					},
					Action: serve,
				},
			},
			Action: connectivity,
		},
	}
	app.Action = func(c *cli.Context) error {
		log.SetUseColors(false)
		log.SetDebugVisible(c.GlobalInt("debug"))
		return action(c)
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

type se struct {
	Server *network.ServerIdentity
	Status *status.Response
	Err    string
}

// will contact all cothorities in the group-file and print
// the status-report of each one.
func action(c *cli.Context) error {
	groupToml := c.GlobalString("g")
	format := c.String("format")
	var list []*network.ServerIdentity

	host := c.String("host")
	if host != "" {
		// Only contact one host
		log.Info("Only contacting one host", host)
		addr := network.Address(host)
		if !strings.HasPrefix(host, "tls://") {
			addr = network.NewAddress(network.TLS, host)
		}
		si := network.NewServerIdentity(nil, addr)
		if si.Address.Port() == "" {
			return errors.New("port not found, must provide host:port")
		}
		list = append(list, si)
	} else {

		ro, err := readGroup(groupToml)
		if err != nil {
			return errors.New("couldn't read file: " + err.Error())
		}
		log.Lvl3(ro)
		list = ro.List
		log.Info("List is", list)
	}
	cl := status.NewClient()

	var all []se
	for _, server := range list {
		sr, err := cl.Request(server)
		if err != nil {
			err = fmt.Errorf("could not get status from %v: %v", server, err)
		}

		if format == "txt" {
			if err != nil {
				log.Error(err)
			} else {
				printTxt(sr)
			}
		} else {
			// JSON
			errStr := "ok"
			if err != nil {
				errStr = err.Error()
			}
			all = append(all, se{Server: server, Status: sr, Err: errStr})
		}
	}
	if format == "json" {
		printJSON(all)
	}
	return nil
}

func connectivity(c *cli.Context) error {
	if c.NArg() != 2 {
		return errors.New("please give 2 arguments: group.toml private.toml")
	}
	ro, err := readGroup(c.Args().First())
	if err != nil {
		return errors.New("couldn't read file: " + err.Error())
	}
	log.Lvl3(ro)
	list := ro.List
	log.Info("List is", list)
	to, err := time.ParseDuration(c.String("timeout"))
	if err != nil {
		return errors.New("duration parse error: " + err.Error())
	}
	ff := c.Bool("findFaulty")
	coth, err := app.LoadCothority(c.Args().Get(1))
	if err != nil {
		return errors.New("error while loading private.toml: " + err.Error())
	}
	si, err := coth.GetServerIdentity()
	if err != nil {
		return errors.New("private.toml didn't have a serverIdentity: " + err.Error())
	}
	resp, err := status.NewClient().CheckConnectivity(si.GetPrivate(), list, time.Duration(to), ff)
	if err != nil {
		return errors.New("couldn't get private key from private.toml: " + err.Error())
	}
	switch len(resp) {
	case 1:
		return errors.New("couldn't contact any other node")
	case len(list):
		log.Info("All nodes can communicate with each other")
	default:
		log.Info("The following nodes can communicate with each other")
	}
	for _, si := range resp {
		log.Info("  ", si.String())
	}
	return nil
}

// readGroup takes a toml file name and reads the file, returning the entities
// within.
func readGroup(tomlFileName string) (*onet.Roster, error) {
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}
	g, err := app.ReadGroupDescToml(f)
	if err != nil {
		return nil, err
	}
	if len(g.Roster.List) <= 0 {
		return nil, errors.New("Empty or invalid group file:" +
			tomlFileName)
	}
	log.Lvl3(g.Roster)
	return g.Roster, err
}

// prints the status response that is returned from the server
func printTxt(e *status.Response) {
	log.Info("-----------------------------------------------")
	log.Infof("Address = \"%s\"", e.ServerIdentity.Address)
	log.Info("Suite = \"Ed25519\"")
	log.Infof("Public = \"%s\"", e.ServerIdentity.Public)
	log.Infof("Description = \"%s\"", e.ServerIdentity.Description)
	log.Info("-----------------------------------------------")
	var a []string
	if e.Status == nil {
		log.Error("no status from ", e.ServerIdentity)
		return
	}

	for sec, st := range e.Status {
		for key, value := range st.Field {
			a = append(a, (sec + "." + key + ": " + value))
		}
	}
	sort.Strings(a)
	log.Info(strings.Join(a, "\n"))
}

func printJSON(all []se) {
	b1 := new(bytes.Buffer)
	e := json.NewEncoder(b1)
	e.Encode(all)

	b2 := new(bytes.Buffer)
	json.Indent(b2, b1.Bytes(), "", "  ")

	out := bufio.NewWriter(os.Stdout)
	out.Write(b2.Bytes())
	out.Flush()
}

type serverStatus struct {
	Description string
	Status      int
}

type serveResponse struct {
	Connectivity  float64
	Matrix        map[string]*serverStatus
	Self          string
	LastCheckedAt int64
	ProbeError    string
}

type server struct {
	response serveResponse
	mu       sync.Mutex

	si       *network.ServerIdentity
	list     []*network.ServerIdentity
	timeout  time.Duration
	interval time.Duration
	quit     chan struct{}
}

func serve(c *cli.Context) error {
	if c.NArg() != 2 {
		return errors.New("please give 2 arguments: group.toml private.toml")
	}
	ro, err := readGroup(c.Args().First())
	if err != nil {
		return errors.New("couldn't read file: " + err.Error())
	}
	log.Lvl3(ro)
	list := ro.List
	log.Info("List is", list)
	to, err := time.ParseDuration(c.Parent().String("timeout"))
	if err != nil {
		return errors.New("duration parse error: " + err.Error())
	}
	coth, err := app.LoadCothority(c.Args().Get(1))
	if err != nil {
		return errors.New("error while loading private.toml: " + err.Error())
	}
	si, err := coth.GetServerIdentity()
	if err != nil {
		return errors.New("private.toml didn't have a serverIdentity: " + err.Error())
	}

	format := c.String("format")
	if format != "prometheus" {
		return errors.New("unsupported format: " + format)
	}

	endpoint := c.String("endpoint")
	if len(endpoint) == 0 || endpoint[0] != '/' {
		return errors.New("invalid endpoint: " + endpoint)
	}

	port := c.Int("port")
	interval, err := time.ParseDuration(c.String("interval"))
	if err != nil {
		return errors.New("invalid interval: " + err.Error())
	}

	s := &server{
		interval: interval,
		timeout:  to,
		list:     list,
		si:       si,
		quit:     make(chan struct{}),
	}

	http.HandleFunc(endpoint, s.serveHandler)
	httpServer := &http.Server{Addr: fmt.Sprintf(":%d", port)}

	done := s.probe()
	go func() {
		log.Infof("Starting HTTP server on %d\n", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("error starting server")
			return
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	<-signalChan
	log.Infof("Shutting down server..")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Errorf("Error shutting down server: %s", err)
	}

	s.quit <- struct{}{}
	<-done
	return nil
}

func (s *server) probe() chan struct{} {
	done := make(chan struct{})
	go func() {
		tick := time.Tick(s.interval)
		for {
			select {
			case <-s.quit:
				log.Infof("Shutting down prober...")
				done <- struct{}{}
				return
			case <-tick:
				log.Infof("Probing servers...")
				s.mu.Lock()
				s.response = serveResponse{
					Matrix:        make(map[string]*serverStatus),
					Connectivity:  0,
					Self:          s.si.Address.String(),
					LastCheckedAt: time.Now().Unix(),
				}
				for _, si := range s.list {
					s.response.Matrix[si.String()] = &serverStatus{
						Description: si.Description,
						Status:      0,
					}
				}

				resp, err := status.NewClient().CheckConnectivity(
					s.si.GetPrivate(),
					s.list,
					s.timeout,
					true,
				)

				if err != nil {
					log.Errorf("error checking connectivity: %s", err)
					s.response.ProbeError = err.Error()
				}

				for _, si := range resp {
					server, ok := s.response.Matrix[si.String()]
					if !ok {
						log.Lvlf3("unrecognised server in response: %s", si.String())
						continue
					}

					server.Status = 1
				}
				s.response.Connectivity = float64(len(resp)) / float64(len(s.list))
				s.mu.Unlock()
			}
		}
	}()
	return done
}

func (s *server) serveHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	t, err := template.New("prometheus").Parse(prometheusTemplate)
	if err != nil {
		log.Errorf("error parsing template: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	t.Execute(w, s.response)
}
