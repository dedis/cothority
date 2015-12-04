package tree

import (
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"strconv"
)

// PeerList regroups a number of peers in a list. One peer can be
// member of more than one PeerList.
type PeerList struct {
	// A list of all peers that are part of this list
	Peers []*Peer
	// The hash-id of this list
	Hash hashid.HashId
	// The suite used in this list
	Suite abstract.Suite
}

// Peer represents a Cothority-member
type Peer struct {
	// The hostname of the peer, including the port-number
	Name string
	// The public-key
	PubKey abstract.Point
	// The private-key - mostly for ourselves
	PrivKey abstract.Secret
	// A network-connection - if already set up
	Conn network.Conn
}

// NewPeerListLocalhost creates a list of n peers, starting
// with port p and random keys.
// Each host from hosts is taken in turn to make a new peer.
func NewPeerList(s abstract.Suite, hosts []string, n, port int) *PeerList {
	pl := &PeerList{
		Peers: make([]*Peer, n),
		Suite: s,
	}

	nh := len(hosts)
	// Pick a random generator with a seed of n and port
	rand := s.Cipher([]byte(strconv.Itoa(n * port)))
	for i := 0; i < n; i++ {
		privKey := s.Secret().Pick(rand)
		pl.Peers[i] = &Peer{
			Name:    hosts[i%nh] + ":" + strconv.Itoa(port+i/nh),
			PrivKey: privKey,
			PubKey:  s.Point().Mul(nil, privKey),
		}
	}
	//pl.Hash = s.Secret().Pick(rand)
	return pl
}

// NewNaryTree creates a tree of peers recursively with branching
// factor bf. If bf = 2, it will create a binary tree.
func (pl *PeerList) NewNaryTree(bf int) *Tree {
	// Create a hash of (peerList.Hash || #peers || bf )
	hash := pl.Suite.Hash()
	hash.Write(pl.Hash)
	hash.Write([]byte(strconv.Itoa(len(pl.Peers))))
	hash.Write([]byte(strconv.Itoa(bf)))
	t := &Tree{
		Hash:   hash.Sum(nil),
		HashPL: pl.Hash,
		BF:     bf,
	}
	root := t.AddRoot(pl.Peers[0])
	root.NewNaryTree(pl.Peers[1:])
	return t
}

/*

// MarshalJSON puts the Peer into a JSON-structure
func (p *Peer) MarshalJSON() ([]byte, error) {
	return nil, nil
}

// MarshalJSON puts the PeerList into a JSON-compatible container
// putting all peers as binary blobs.
func (pl *PeerList) MarshalJSON() ([]byte, error) {
	var ps [][]byte = make([][]byte, 0)
	for _, p := range pl.Peers {
		b, err := p.MarshalJSON()
		if err != nil {
			return nil, err
		}
		ps = append(ps, b)
	}
	return json.Marshal(&struct {
		SuiteStr string
		Hash     []byte
		Peers    [][]byte
	}{
		SuiteStr: pl.Suite.String(),
		Hash:     pl.Hash,
		Peers:    ps,
	})
}

func (pl *PeerList) UnmarshalJSON(dataJSON []byte) error {
	type Alias PeerList
	suite, err := suites.StringToSuite(pl.Suite)
	if err != nil {
		return fmt.Errorf("Couldn't get suite: %s", err)
	}
	aux := &struct {
		BinaryBlob []byte
		Response   abstract.Secret
		Challenge  abstract.Secret
		AggCommit  abstract.Point
		AggPublic  abstract.Point
		*Alias
	}{
		Response:  suite.Secret(),
		Challenge: suite.Secret(),
		AggCommit: suite.Point(),
		AggPublic: suite.Point(),
		Alias:     (*Alias)(sr),
	}
	if err := json.Unmarshal(dataJSON, &aux); err != nil {
		return err
	}
	if err := suite.Read(bytes.NewReader(aux.BinaryBlob), &sr.Response,
		&sr.Challenge, &sr.AggCommit, &sr.AggPublic); err != nil {
		dbg.Fatal("decoding signature Response / Challenge / AggCommit:", err)
		return err
	}
	return nil
}
*/
