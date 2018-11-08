package byzcoin

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/dedis/cothority/darc"
)

// getSignerCounter returns 0 if the key is not set, otherwise it loads the
// counter from the Trie.
func getSignerCounter(st ReadOnlyStateTrie, id string) (uint64, error) {
	val, _, _, err := st.GetValues(publicVersionKey(id))
	if err == errKeyNotSet {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	ver := binary.LittleEndian.Uint64(val)
	return ver, nil
}

// incrementSignerCounters loads the existing counters from sigs and then
// increments all of them by 1.
func incrementSignerCounters(st ReadOnlyStateTrie, sigs []darc.Signature) (StateChanges, error) {
	var scs StateChanges
	for _, sig := range sigs {
		id := sig.Signer.String()
		ver, err := getSignerCounter(st, id)
		if err != nil {
			return scs, err
		}
		verBuf := make([]byte, 8)
		// If ver is the highest uint64, then it'll overflow and go
		// back to 0, this is the intended behaviour, otherwise the
		// client will not be able to make more transactions.
		binary.LittleEndian.PutUint64(verBuf, ver+1)
		scs = append(scs, StateChange{
			StateAction: Update,
			InstanceID:  publicVersionKey(id),
			ContractID:  []byte{},
			Value:       verBuf,
			DarcID:      darc.ID([]byte{}),
		})
	}
	return scs, nil
}

// verifySignerCounters verifies whether the given counters are valid with
// respect to the current counters.
func verifySignerCounters(st ReadOnlyStateTrie, counters []uint64, sigs []darc.Signature) error {
	if len(counters) != len(sigs) {
		return errors.New("lengths of the counters and signatures are not the same")
	}
	for i, counter := range counters {
		if !sigs[i].Signer.PrimaryIdentity() {
			return errors.New("not a primary identity")
		}
		id := sigs[i].Signer.String()
		c, err := getSignerCounter(st, id)
		if err != nil {
			return err
		}
		// If c is the highest uint64, then it'll overflow and go back
		// to 0, this is the intended behaviour, otherwise the client
		// will not be able to make more transactions.
		if counter != c+1 {
			return fmt.Errorf("for pk %s, got version %v, but need %v", id, counter, c+1)
		}
	}
	return nil
}

func publicVersionKey(id string) []byte {
	h := sha256.New()
	h.Write([]byte("signercounter_"))
	h.Write([]byte(id))
	return h.Sum(nil)
}
