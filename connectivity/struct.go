package connectivity

import (
	"go.dedis.ch/onet/v3"
	"time"
)

const Name = "connectivity"

// Ping is used to check connectivity with children
type Ping struct {
}

type pingWrapper struct {
	*onet.TreeNode
	Ping
}

// Pong is used to acknowledge a ping
type Pong struct {
}

type pongWrapper struct {
	*onet.TreeNode
	Pong
}

type state struct {
	Name        string
	Down        bool
	LastErrorAt int64
}

type CheckRequest struct {
	Roster *onet.Roster
}

type CheckReply struct {
	ConnectivityMatrix
}

type ConnectivityMatrix struct {
	Status        map[string]*state
	LastCheckedAt time.Time
}
