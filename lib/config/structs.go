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

type AppConfig struct {
	Hostname    string // Hostname like server-0.cs-dissent ?
	Logger      string // ip addr of the logger to connect to
	PhysAddr    string // physical IP addr of the host
	AmRoot      bool   // is the host root (i.e. special operations)
	TestConnect bool   // ?
	App         string // which app are we running on this host
	Mode        string // ?
	Name        string // ?
	Server      string // ?
}
