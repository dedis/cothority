package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"reflect"
	"sort"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share/pvss"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

func (rh *RandHound) newSession(nodes int, groups int, purpose string, timestamp int64, seed []byte, clientKey kyber.Point) (*Session, error) {

	var err error

	if timestamp == 0 {
		timestamp = time.Now().UTC().Unix()
	}

	if seed == nil {
		seed = make([]byte, rh.Suite().Hash().Size())
		random.Bytes(seed, random.New())
		// seed = random.Bytes(, random.Stream)
	}

	// Shard servers
	indices, err := Shard(rh.Suite(), seed, nodes, groups)
	if err != nil {
		return nil, err
	}

	// Setup group information
	treeNodes := rh.List()
	servers := make([][]*onet.TreeNode, groups)
	serverKeys := make([][]kyber.Point, groups)
	thresholds := make([]uint32, groups)
	groupNum := make(map[int]int)
	groupPos := make(map[int]int)
	for i, group := range indices {
		s := make([]*onet.TreeNode, len(group))
		k := make([]kyber.Point, len(group))
		for j, g := range group {
			s[j] = treeNodes[g]
			k[j] = treeNodes[g].ServerIdentity.Public
			groupNum[g] = i
			groupPos[g] = j
		}
		servers[i] = s
		serverKeys[i] = k
		thresholds[i] = uint32(len(s)/3 + 1)
	}

	// Compute session identifier
	sid, err := sessionID(rh.Suite(), clientKey, serverKeys, indices, purpose, timestamp)
	if err != nil {
		return nil, err
	}

	// Setup session
	session := &Session{
		nodes:      nodes,
		groups:     groups,
		purpose:    purpose,
		time:       timestamp,
		seed:       seed,
		clientKey:  clientKey,
		sid:        sid,
		servers:    servers,
		serverKeys: serverKeys,
		indices:    indices,
		thresholds: thresholds,
		groupNum:   groupNum,
		groupPos:   groupPos,
	}

	return session, nil
}

func sessionID(suite Suite, clientKey kyber.Point, serverKeys [][]kyber.Point, indices [][]int, purpose string, timestamp int64) ([]byte, error) {
	// Setup some buffers
	keyBuf := new(bytes.Buffer)
	idxBuf := new(bytes.Buffer)
	miscBuf := new(bytes.Buffer)

	// Process client key
	cb, err := clientKey.MarshalBinary()
	if err != nil {
		return nil, err
	}
	if _, err := keyBuf.Write(cb); err != nil {
		return nil, err
	}

	// Process server keys and group indices
	for i := range serverKeys {
		for j := range serverKeys[i] {
			kb, err := serverKeys[i][j].MarshalBinary()
			if err != nil {
				return nil, err
			}
			if _, err := keyBuf.Write(kb); err != nil {
				return nil, err
			}
			if err := binary.Write(idxBuf, binary.LittleEndian, uint32(indices[i][j])); err != nil {
				return nil, err
			}
		}
	}

	// Process purpose string
	if _, err := miscBuf.WriteString(purpose); err != nil {
		return nil, err
	}

	// Process time stamp
	if err := binary.Write(miscBuf, binary.LittleEndian, timestamp); err != nil {
		return nil, err
	}

	hash := sha256.New()
	hash.Write([]byte(suite.String()))
	hash.Write(keyBuf.Bytes())
	hash.Write(idxBuf.Bytes())
	hash.Write(miscBuf.Bytes())
	return hash.Sum(nil), nil
	// return hash.Bytes(suite.Hash(), keyBuf.Bytes(), idxBuf.Bytes(), miscBuf.Bytes())
}

func (rh *RandHound) newMessages() *Messages {
	return &Messages{
		i1:  nil,
		i2s: make(map[int]*I2),
		i3:  nil,
		r1s: make(map[int]*R1),
		r2s: make(map[int]*R2),
		r3s: make(map[int]*R3),
	}
}

func recoverRandomness(suite Suite, sid []byte, keys []kyber.Point, thresholds []uint32, indices [][]int, records map[int]map[int]*Record) ([]byte, error) {
	rnd := suite.Point().Null()
	G := suite.Point().Base()
	H := suite.Point().Pick(suite.XOF(sid))
	for src := range records {
		var groupKeys []kyber.Point
		var encShares []*pvss.PubVerShare
		var decShares []*pvss.PubVerShare
		for tgt, record := range records[src] {
			if record.Eval != nil && record.EncShare != nil && record.DecShare != nil {
				if pvss.VerifyEncShare(suite, H, keys[tgt], record.Eval, record.EncShare) == nil {
					groupKeys = append(groupKeys, keys[tgt])
					encShares = append(encShares, record.EncShare)
					decShares = append(decShares, record.DecShare) // NOTE: decrypted shares will be verified during recovery
				}
			}
		}
		grp := 0 // find group number
		for i := range indices {
			for j := range indices[i] {
				if src == indices[i][j] {
					grp = i
					break
				}
			}
		}
		ps, err := pvss.RecoverSecret(suite, G, groupKeys, encShares, decShares, int(thresholds[grp]), len(indices[grp]))
		if err != nil {
			return nil, err
		}
		rnd = suite.Point().Add(rnd, ps)
	}
	rb, err := rnd.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return rb, nil
}

func chosenSecrets(records map[int]map[int]*Record) []uint32 {
	var chosenSecrets []uint32
	for src := range records {
		chosenSecrets = append(chosenSecrets, uint32(src))
	}
	sort.Slice(chosenSecrets, func(i, j int) bool {
		return chosenSecrets[i] < chosenSecrets[j]
	})
	return chosenSecrets
}

func signSchnorr(suite schnorr.Suite, key kyber.Scalar, m interface{}) error {
	// Reset signature field
	reflect.ValueOf(m).Elem().FieldByName("Sig").SetBytes([]byte{0}) // XXX: hack

	// Marshal message
	mb, err := network.Marshal(m) // TODO: change m to interface with hash to make it compatible to other languages (network.Marshal() adds struct-identifiers)
	if err != nil {
		return err
	}

	// Sign message
	sig, err := schnorr.Sign(suite, key, mb)
	if err != nil {
		return err
	}

	// Store signature
	reflect.ValueOf(m).Elem().FieldByName("Sig").SetBytes(sig) // XXX: hack

	return nil
}

func verifySchnorr(suite schnorr.Suite, key kyber.Point, m interface{}) error {
	// Make a copy of the signature
	sig := reflect.ValueOf(m).Elem().FieldByName("Sig").Bytes()

	// Reset signature field
	reflect.ValueOf(m).Elem().FieldByName("Sig").SetBytes([]byte{0}) // XXX: hack

	// Marshal message
	mb, err := network.Marshal(m) // TODO: change m to interface with hash to make it compatible to other languages (network.Marshal() adds struct-identifiers)
	if err != nil {
		return err
	}

	// Copy back original signature
	reflect.ValueOf(m).Elem().FieldByName("Sig").SetBytes(sig) // XXX: hack

	return schnorr.Verify(suite, key, mb, sig)
}

func hashMessage(suite Suite, m interface{}) ([]byte, error) {

	// Reset signature field
	reflect.ValueOf(m).Elem().FieldByName("Sig").SetBytes([]byte{0}) // XXX: hack

	// Marshal ...
	mb, err := network.Marshal(m) // TODO: change m to interface with hash to make it compatible to other languages (network.Marshal() adds struct-identifiers)
	if err != nil {
		return nil, err
	}

	// ... and hash message
	hash := sha256.New()
	hash.Write([]byte(suite.String()))
	hash.Write(mb)
	return hash.Sum(nil), nil
}
