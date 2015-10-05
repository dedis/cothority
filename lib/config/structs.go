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

// Simplest config representig the type of connection we want to do (tcp / goroutines ?)
// and the list of hostnames like "10.0.4.10:2000
type HostsConfig struct {
	Conn  string   `json:"conn,omitempty"`
	Hosts []string `json:"hosts"`
}
