package main

import (
	"fmt"
	"io"
	"log"
)

// should go in abstract Net package
type Host interface {
	Name() string          // this host's name
	Open(name string) Conn // open connection with named peer
	//Recv() (msg []byte, from Conn)	// receive from any Conn
	// XXX RecvFrom?
}

type Conn interface {
	PeerName() string      // name of peer host
	Send(msg []byte) error // send message to connected peer
	Recv() ([]byte, error) // receive message from peer
	Close() error          // close connection
}

// XXX channel-based virtual network; move to chanNet.go

type chanNet struct {
	hosts map[string]*chanHost
}

func newChanNet() *chanNet {
	cn := &chanNet{}
	cn.hosts = make(map[string]*chanHost)
	return cn
}

type chanHost struct {
	net  *chanNet // virtual network on which we're a host
	name string   // my hostname on this (virtual) network
	//rq chan chanMsg	// host's undirected message receive queue
	srv func(Conn) error // connection-processing function for servers
}

/*
type chanMsg struct {
	msg []byte		// content of message
	src *chanConn		// sender of this message
}
*/

func newChanHost(net *chanNet, name string, server func(Conn) error) *chanHost {
	if _, exists := net.hosts[name]; exists {
		panic(fmt.Sprintf("host %s already exists", name))
	}
	ch := &chanHost{net, name, server}
	net.hosts[name] = ch
	return ch
}

func (ch *chanHost) Name() string {
	return ch.name
}

func (ch *chanHost) Open(name string) Conn {
	dst := ch.net.hosts[name]

	var ci, cr chanConn
	ci = chanConn{ch, dst, make(chan []byte), &cr}
	cr = chanConn{dst, ch, make(chan []byte), &ci}

	// Launch a server goroutine to service this client
	go func() {
		if err := dst.srv(&cr); err != nil {
			log.Printf("server %s error: %s", dst.name, err)
		}
		cr.Close()
	}()

	return &ci
}

type chanConn struct {
	src *chanHost   // source this object is associated with
	dst *chanHost   // destination this object represents
	srq chan []byte // src's message receive queue
	rev *chanConn   // reverse-direction chanConn with src <-> dst
}

func (cp *chanConn) PeerName() string {
	return cp.dst.name
}

func (cp *chanConn) Send(msg []byte) error {
	cp.rev.srq <- msg
	return nil
}

func (cp *chanConn) Recv() ([]byte, error) {
	srq := cp.srq
	if srq == nil {
		return nil, io.EOF // message queue already closed
	}
	msg := <-srq
	if msg == nil {
		cp.srq = nil
		return nil, io.EOF // message queue just now closed
	}
	return msg, nil
}

func (cp *chanConn) Close() error {
	srq := cp.rev.srq
	if srq != nil {
		srq <- nil
	}
	return nil
}
