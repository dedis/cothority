package main

import (
	"fmt"
)

// should go in abstract Net package
type Host interface {
	Name() string			// this host's name
	Find(name string) Peer		// find a Peer by name
	Recv() (msg []byte, from Peer)	// receive from any Peer
	// XXX RecvFrom?
}

type Peer interface {
	Name() string
	Send(msg []byte) error
	//Recv() ([]byte, error)	// receive only from this Peer
}



// XXX channel-based virtual network; move to chanNet.go


type chanNet struct {
	hosts	map[string]*chanHost
}

func newChanNet() *chanNet {
	cn := &chanNet{}
	cn.hosts = make(map[string]*chanHost)
	return cn
}


type chanHost struct {
	net *chanNet		// virtual network on which we're a host
	name string		// my hostname on this (virtual) network
	rq chan chanMsg		// host's undirected message receive queue
}

type chanMsg struct {
	msg []byte		// content of message
	src *chanPeer		// sender of this message
}

func newChanHost(net *chanNet, name string) *chanHost {
	if _, exists := net.hosts[name]; exists {
		panic(fmt.Sprintf("host %s already exists", name))
	}
	ch := &chanHost{net, name, make(chan chanMsg)}
	net.hosts[name] = ch
	return ch
}

func (ch *chanHost) Name() string {
	return ch.name
}

func (ch *chanHost) Find(name string) Peer {
	return &chanPeer{ch, ch.net.hosts[name], nil}
}

func (ch *chanHost) Recv() ([]byte, Peer) {
	cm := <-ch.rq
	return cm.msg, cm.src
}



type chanPeer struct {
	src *chanHost		// source this object is associated with
	dst *chanHost		// destination this object represents
	rev *chanPeer		// reverse-direction chanPeer with src <-> dst
}



func (cp *chanPeer) Name() string {
	return cp.dst.name
}

func (cp *chanPeer) Send(msg []byte) error {
	if cp.rev == nil {	// create reverse-direction chanPeer
		cp.rev = &chanPeer{cp.dst, cp.src, cp}
	}
	cp.dst.rq <- chanMsg{msg, cp.rev}
	return nil
}


