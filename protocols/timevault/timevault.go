package timevault

import (
	"fmt"
	"sync"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
	"github.com/dedis/crypto/random"
)

func init() {
	sda.ProtocolRegisterName("TimeVault", NewTimeVault)
}

// Type of shared secret identifiers
type SID string

// Identifiers for TimeVault shared secrets.
const (
	TVSS SID = "TVSS"
)

type TimeVault struct {
	*sda.Node
	keyPair     *config.KeyPair
	pubKeys     []abstract.Point
	info        poly.Threshold
	secrets     map[SID]*Secret // TODO: Could make sense to store pairs of secrets?!
	secretsDone chan bool
}

type Secret struct {
	secret   *poly.SharedSecret // Shared secret
	receiver *poly.Receiver     // Receiver to aggregate deals
	deals    map[int]*poly.Deal // Buffer for deals
	numConfs int                // Number of collected confirmations that shared secrets are ready
	mtx      sync.Mutex         // Mutex to sync access to numConfs
}

func NewTimeVault(node *sda.Node) (sda.ProtocolInstance, error) {

	kp := &config.KeyPair{Suite: node.Suite(), Public: node.Public(), Secret: node.Private()}
	n := len(node.List())
	pk := make([]abstract.Point, n)
	for i, tn := range node.List() {
		pk[i] = tn.Entity.Public
	}

	// NOTE: T <= R <= N (for simplicity we use T = R = N; might change later)
	info := poly.Threshold{T: n, R: n, N: n}

	tv := &TimeVault{
		Node:        node,
		keyPair:     kp,
		pubKeys:     pk,
		info:        info,
		secrets:     make(map[SID]*Secret),
		secretsDone: make(chan bool, 1),
	}

	// Setup message handlers
	h := []interface{}{
		tv.handleSecInit,
		tv.handleSecConf,
		tv.handleRevInit,
		tv.handleRevShare,
	}
	err := tv.RegisterHandlers(h...)

	return tv, err
}

func (tv *TimeVault) Start() error {
	return nil
}

// Seal encrypts a given message and releases the decryption key after a given time.
func (tv *TimeVault) Seal(msg []byte, time time.Duration) error {

	// Generate two shared secrets for ElGamal encryption
	var err error
	sid0 := SID(fmt.Sprintf("%s-0-%d", TVSS, tv.Node.Index()))
	err = tv.initSecret(sid0)
	<-tv.secretsDone
	if err != nil {
		return err
	}
	sid1 := SID(fmt.Sprintf("%s-1-%d", TVSS, tv.Node.Index()))
	err = tv.initSecret(sid1)
	<-tv.secretsDone
	if err != nil {
		return err
	}

	// Do ElGamal encryption
	X := tv.secrets[sid0].secret.Pub.SecretCommit()
	Y := tv.secrets[sid1].secret.Pub.SecretCommit()
	S := tv.keyPair.Suite.Point().Add(X, Y)
	M, m := tv.keyPair.Suite.Point().Pick(msg, random.Stream)
	C := tv.keyPair.Suite.Point().Add(M, S)

	dbg.Lvl1("Ciphertext:", C)

	// Reveal shares
	if err := tv.Broadcast(&RevInitMsg{Src: tv.Node.Index(), SID: sid0}); err != nil {
		return err
	}

	// TODO:
	// - Encrypt the msg and start a timer, put that into a go routine, return an ID
	// - Periodically query the TimeVault group with the given ID
	// - Once the timer in the go routine expired, broadcast a reveal secrets msg
	// - Once all shares have arrived, decrypt the message and pipe it out through a public channel

	return nil
}

func (tv *TimeVault) initSecret(sid SID) error {

	// Initialise shared secret of given type if necessary
	if _, ok := tv.secrets[sid]; !ok {
		dbg.Lvl2(fmt.Sprintf("Node %d: Initialising %s shared secret", tv.Node.Index(), sid))
		sec := &Secret{
			receiver: poly.NewReceiver(tv.keyPair.Suite, tv.info, tv.keyPair),
			deals:    make(map[int]*poly.Deal),
			numConfs: 0,
		}
		tv.secrets[sid] = sec
	}

	secret := tv.secrets[sid]

	// Initialise and broadcast our deal if necessary
	if len(secret.deals) == 0 {
		kp := config.NewKeyPair(tv.keyPair.Suite)
		deal := new(poly.Deal).ConstructDeal(kp, tv.keyPair, tv.info.T, tv.info.R, tv.pubKeys)
		dbg.Lvl2(fmt.Sprintf("Node %d: Initialising %v deal", tv.Node.Index(), sid))
		secret.deals[tv.Node.Index()] = deal
		db, _ := deal.MarshalBinary()
		msg := &SecInitMsg{
			Src:  tv.Node.Index(),
			SID:  sid,
			Deal: db,
		}
		if err := tv.Broadcast(msg); err != nil {
			return err
		}
	}
	return nil
}

func (tv *TimeVault) finaliseSecret(sid SID) error {
	secret, ok := tv.secrets[sid]
	if !ok {
		return fmt.Errorf("Error, shared secret does not exist")
	}

	dbg.Lvl2(fmt.Sprintf("Node %d: %s deals %d/%d", tv.Node.Index(), sid, len(secret.deals), len(tv.Node.List())))

	if len(secret.deals) == tv.info.T {

		for _, deal := range secret.deals {
			if _, err := secret.receiver.AddDeal(tv.Node.Index(), deal); err != nil {
				return err
			}
		}

		sec, err := secret.receiver.ProduceSharedSecret()
		if err != nil {
			return err
		}
		secret.secret = sec
		secret.mtx.Lock()
		secret.numConfs++
		secret.mtx.Unlock()
		dbg.Lvl2(fmt.Sprintf("Node %d: %v created", tv.Node.Index(), sid))

		// Broadcast that we have finished setting up our shared secret
		msg := &SecConfMsg{
			Src: tv.Node.Index(),
			SID: sid,
		}
		if err := tv.Broadcast(msg); err != nil {
			return err
		}
	}
	return nil

}
