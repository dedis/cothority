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
	keyPair          *config.KeyPair
	pubKeys          []abstract.Point
	info             poly.Threshold
	secrets          map[SID]*Secret // TODO: Could make sense to store pairs of secrets?!
	recoveredSecrets map[SID]*RecoveredSecret
	secretsDone      chan bool
	secretsChan      chan abstract.Secret
}

type Secret struct {
	secret   *poly.SharedSecret // Shared secret
	receiver *poly.Receiver     // Receiver to aggregate deals
	deals    map[int]*poly.Deal // Buffer for deals
	numConfs int                // Number of collected confirmations that shared secrets are ready
	mtx      sync.Mutex         // Mutex to sync access to numConfs
}

// TODO: Rename to vault
type RecoveredSecret struct {
	PriShares *poly.PriShares
	NumShares int
	mtx       sync.Mutex
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
		secretsChan: make(chan abstract.Secret, 1),
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
func (tv *TimeVault) Seal(msg []byte, dur time.Duration) error {

	// Generate shared secret for ElGamal encryption
	var err error
	sid := SID(fmt.Sprintf("%s%d", TVSS, tv.Node.Index()))
	err = tv.initSecret(sid)
	<-tv.secretsDone
	if err != nil {
		return err
	}

	// Do ElGamal encryption

	// Generate an emphereal key pair for masking
	eKP := config.NewKeyPair(tv.keyPair.Suite)
	M, _ := tv.keyPair.Suite.Point().Pick(msg, random.Stream)
	X := tv.secrets[sid].secret.Pub.SecretCommit()
	C := tv.keyPair.Suite.Point().Add(M, tv.keyPair.Suite.Point().Mul(X, eKP.Secret))

	// Run Timer
	<-time.After(dur)

	// Setup list of recovered secrets if necessary
	if tv.recoveredSecrets == nil {
		tv.recoveredSecrets = make(map[SID]*RecoveredSecret)
	}

	rs := &RecoveredSecret{PriShares: &poly.PriShares{}, NumShares: 0}
	rs.PriShares.Empty(tv.keyPair.Suite, tv.info.T, tv.info.N)
	rs.PriShares.SetShare(tv.secrets[sid].secret.Index, *tv.secrets[sid].secret.Share)
	rs.NumShares++
	tv.recoveredSecrets[sid] = rs

	// Start process to reveal shares
	if err := tv.Broadcast(&RevInitMsg{Src: tv.Node.Index(), SID: sid}); err != nil {
		return err
	}
	x := <-tv.secretsChan

	// Do ElGamal decryption
	XX := tv.keyPair.Suite.Point().Mul(nil, x)

	dbg.Lvl1("Recovered secret valid?", X.Equal(XX))
	ix := tv.keyPair.Suite.Secret().Neg(x)
	Z := tv.keyPair.Suite.Point().Mul(eKP.Public, ix)
	MM := tv.keyPair.Suite.Point().Add(C, Z)
	dbg.Lvl1("Recovered message equal?", M.Equal(MM))

	data, _ := MM.Data()

	dbg.Lvl1("Data:", string(data))

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
