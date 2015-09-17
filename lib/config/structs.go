package config
import "github.com/dedis/cothority/lib/graphs"

// This file has only the structures in it, for easy references


type ConfigFile struct {
	Hosts []string     `json:"hosts"`
	Tree  *graphs.Tree `json:"tree"`
}

type ConfigFileOld struct {
	Conn  string   `json:"conn,omitempty"`
	Hosts []string `json:"hosts"`
	Tree  *Node    `json:"tree"`
}

type AppConfig struct{
	Hostname string
	Logger string
	PhysAddr string
	AmRoot bool
	TestConnect bool
	App string
	Mode string
	Name string
	Server string
}

