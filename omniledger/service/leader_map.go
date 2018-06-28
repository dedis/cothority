package service

import (
	"errors"
	"sync"

	"github.com/dedis/onet/network"
)

type leaderMap struct {
	sync.Mutex
	// mapping from skipchain ID -> server identity
	mapping map[string]*network.ServerIdentity
	me      *network.ServerIdentity
}

func newLeaderMap(myIdentity *network.ServerIdentity) leaderMap {
	return leaderMap{
		mapping: make(map[string]*network.ServerIdentity),
		me:      myIdentity,
	}
}

func (m *leaderMap) add(k string, v *network.ServerIdentity) error {
	m.Lock()
	defer m.Unlock()
	_, ok := m.mapping[k]
	if ok {
		return errors.New(k + " is already in the map")
	}
	m.mapping[k] = v
	return nil
}

func (m *leaderMap) update(k string, v *network.ServerIdentity) error {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.mapping[k]; ok {
		m.mapping[k] = v
	}
	return errors.New(k + " is not in the map")
}

func (m *leaderMap) get(k string) *network.ServerIdentity {
	m.Lock()
	defer m.Unlock()
	if v, ok := m.mapping[k]; ok {
		return v
	}
	return nil
}

func (m *leaderMap) isMe(k string) bool {
	m.Lock()
	defer m.Unlock()
	if v, ok := m.mapping[k]; ok {
		return m.me.Equal(v)
	}
	return false
}
