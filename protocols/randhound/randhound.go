package randhound

import (
	"bytes"
	"encoding/binary"
	"sync"
	"time"

	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

func init() {
	sda.ProtocolRegisterName("RandHound", NewRandHound)
}

type RandHound struct {
	*sda.TreeNodeInstance
	Transcript *Transcript       // Transcript of a protocol run
	Done       chan bool         // To signal that a protocol run is finished
	mutex      sync.Mutex        // ...
	counter    uint32            // XXX: dummy, remove later
	Server     [][]*sda.TreeNode // Grouped servers (with connection infos)
}

type Transcript struct {
	SID     []byte   // Session identifier
	Session *Session // Session parameters
}

type Session struct {
	Nodes     uint32             // Total number of nodes (client + servers)
	Faulty    uint32             // Maximum number of Byzantine servers
	Groups    uint32             // Total number of groups
	Purpose   string             // Purpose of the protocol run
	Time      time.Time          // Timestamp of initiation
	Rand      []byte             // Client-chosen randomness
	Key       [][]abstract.Point // Server key grouping derived from the client-chosen randomness
	Threshold []uint32           // Secret sharing thresholds of individual server groups
}

// I1 message
type I1 struct {
	SID []byte           // Session identifier
	T   uint32           // Secret sharing threshold
	Key []abstract.Point // Public keys of trustees
}

// R1 message
type R1 struct {
	HI1 []byte // Hash of I1
}

// I2 message
type I2 struct {
	SID []byte // Session identifier
}

// R2 message
type R2 struct {
	HI2 []byte // Hash of I2
}

// WI1 is a SDA-wrapper around I1
type WI1 struct {
	*sda.TreeNode
	I1
}

// WR1 is a SDA-wrapper around R1
type WR1 struct {
	*sda.TreeNode
	R1
}

// WI2 is a SDA-wrapper around I2
type WI2 struct {
	*sda.TreeNode
	I2
}

// WR2 is a SDA-wrapper around R2
type WR2 struct {
	*sda.TreeNode
	R2
}

func NewRandHound(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	// Setup RandHound protocol struct
	rh := &RandHound{
		TreeNodeInstance: node,
		counter:          0,
	}

	// Setup message handlers
	h := []interface{}{
		rh.handleI1, rh.handleI2,
		rh.handleR1, rh.handleR2,
	}
	err := rh.RegisterHandlers(h...)

	return rh, err
}

// Setup ...
func (rh *RandHound) Setup(nodes uint32, faulty uint32, groups uint32, purpose string) error {

	rh.Transcript = &Transcript{}
	rh.Transcript.Session = &Session{
		Nodes:     nodes,
		Faulty:    faulty,
		Groups:    groups,
		Purpose:   purpose,
		Threshold: make([]uint32, groups),
	}

	rh.Done = make(chan bool, 1)
	rh.counter = 0

	return nil
}

// SID ...
func (rh *RandHound) SID() ([]byte, error) {

	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.LittleEndian, rh.Transcript.Session.Nodes); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, rh.Transcript.Session.Faulty); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, rh.Transcript.Session.Groups); err != nil {
		return nil, err
	}

	if _, err := buf.WriteString(rh.Transcript.Session.Purpose); err != nil {
		return nil, err
	}

	t, err := rh.Transcript.Session.Time.MarshalBinary()
	if err != nil {
		return nil, err
	}

	if _, err := buf.Write(t); err != nil {
		return nil, err
	}

	if _, err := buf.Write(rh.Transcript.Session.Rand); err != nil {
		return nil, err
	}

	for _, keys := range rh.Transcript.Session.Key {
		for _, k := range keys {
			kb, err := k.MarshalBinary()
			if err != nil {
				return nil, err
			}
			if _, err := buf.Write(kb); err != nil {
				return nil, err
			}
		}
	}

	for _, t := range rh.Transcript.Session.Threshold {
		if err := binary.Write(buf, binary.LittleEndian, t); err != nil {
			return nil, err
		}
	}

	return crypto.HashBytes(rh.Suite().Hash(), buf.Bytes())
}

// Start ...
func (rh *RandHound) Start() error {

	// Set timestamp
	rh.Transcript.Session.Time = time.Now()

	// Choose randomness
	hs := rh.Suite().Hash().Size()
	rand := make([]byte, hs)
	random.Stream.XORKeyStream(rand, rand)
	rh.Transcript.Session.Rand = rand

	// Determine server grouping
	servers, keys, err := rh.Shard(rand, rh.Transcript.Session.Groups)
	if err != nil {
		return err
	}
	rh.Server = servers
	rh.Transcript.Session.Key = keys

	// Determine secret sharing thresholds (XXX: currently defaults to 2/3 of max value)
	for i := range keys {
		rh.Transcript.Session.Threshold[i] = uint32(2 * len(keys[i]) / 3)
	}

	// Determine session identifier
	sid, err := rh.SID()
	if err != nil {
		return err
	}
	rh.Transcript.SID = sid

	// Send messages to servers
	for i, group := range servers {
		i1 := &I1{SID: sid, T: rh.Transcript.Session.Threshold[i], Key: keys[i]}
		if err := rh.Multicast(i1, group...); err != nil {
			return err
		}
	}
	return nil
}

func (rh *RandHound) handleI1(i1 WI1) error {

	msg := &i1.I1

	// TODO: For init select random subset of msg.Key of size msg.T
	pvss := PVSS{}
	pvss.Setup(rh.Suite(), int(msg.T), msg.SID, msg.Key)

	hi1 := []byte{1}
	r1 := &R1{HI1: hi1}
	return rh.SendTo(rh.Root(), r1)
}

func (rh *RandHound) handleR1(r1 WR1) error {
	//log.Lvlf1("RandHound - R1: %v %v\n", rh.index(), &r1.R1)

	rh.mutex.Lock()
	defer rh.mutex.Unlock()
	rh.counter++
	//log.Lvlf1("RandHound - R1 - Counter: %v %v\n", rh.index(), rh.counter)
	if rh.counter == 4 {
		rh.counter = 0
		sid := []byte{2}
		i2 := &I2{SID: sid}
		return rh.Broadcast(i2)
	}
	return nil
}

func (rh *RandHound) handleI2(i2 WI2) error {
	//log.Lvlf1("RandHound - I2: %v %v\n", rh.index(), &i2.I2)
	hi2 := []byte{3}
	r2 := &R2{HI2: hi2}
	return rh.SendTo(rh.Root(), r2)
}

func (rh *RandHound) handleR2(r2 WR2) error {
	//log.Lvlf1("RandHound - R2: %v %v\n", rh.index(), &r2.R2)
	rh.mutex.Lock()
	defer rh.mutex.Unlock()
	rh.counter++
	//log.Lvlf1("RandHound - R2 - Counter: %v %v\n", rh.index(), rh.counter)
	if rh.counter == 4 {
		rh.Done <- true
	}
	return nil
}

func (rh *RandHound) index() uint32 {
	return uint32(rh.Index())
}
