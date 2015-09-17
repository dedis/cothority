package config

import (
	"net"

	log "github.com/Sirupsen/logrus"

	"github.com/dedis/cothority/lib/graphs"
)

type ConfigFile struct {
	Hosts []string     `json:"hosts"`
	Tree  *graphs.Tree `json:"tree"`
}

func ConfigFromTree(t *graphs.Tree, hosts []string) *ConfigFile {
	cf := &ConfigFile{}
	cf.Hosts = make([]string, len(hosts))
	copy(cf.Hosts, hosts)
	cf.Tree = t
	return cf
}

func (cf *ConfigFile) AddPorts(ports ...string) {
	if len(ports) == 0 {
		// default port 9001
		log.Println("ports is empty")
		ports = append(ports, "9001")
	}
	if len(ports) == 1 {
		port := ports[0]
		for i := 1; i < len(cf.Hosts); i++ {
			ports = append(ports, port)
		}
	}
	if len(ports) != len(cf.Hosts) {
		log.Fatal("len ports != len cf.Hosts")
	}
	// mcreate a mapping of oldhostnames to new hostnames
	hostmap := make(map[string]string)
	for i, hp := range cf.Hosts {
		h, _, err := net.SplitHostPort(hp)
		if err != nil {
			h = hp
		}
		hostmap[hp] = net.JoinHostPort(h, ports[i])
		cf.Hosts[i] = net.JoinHostPort(h, ports[i])
	}
	log.Println("add ports hostmap:", hostmap)
	cf.Tree.TraverseTree(func(t *graphs.Tree) {
		t.Name = hostmap[t.Name]
	})
}
