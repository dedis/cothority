package byzcoin

import (
	"crypto/sha256"
	"encoding/binary"

	"go.dedis.ch/cothority/v3/darc"
	"golang.org/x/xerrors"
)

// getSignerCounter returns 0 if the key is not set, otherwise it loads the
// counter from the Trie.
func getSignerCounter(st ReadOnlyStateTrie, id string) (uint64, error) {
	val, _, _, _, err := st.GetValues(publicVersionKey(id))
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
func incrementSignerCounters(st ReadOnlyStateTrie, ids []darc.Identity) (StateChanges, error) {
	var scs StateChanges
	for _, id := range ids {
		id := id.String()
		ver, err := getSignerCounter(st, id)
		if err != nil {
			return scs, err
		}
		verBuf := make([]byte, 8)
		// If ver is the highest uint64, then it'll overflow and go
		// back to 0, this is the intended behaviour, otherwise the
		// client will not be able to make more transactions.
		binary.LittleEndian.PutUint64(verBuf, ver+1)
		// If we're at version 0, then it means the counter is not set,
		// so we use the Create action
		action := Update
		if ver == 0 {
			action = Create
		}
		scs = append(scs, StateChange{
			StateAction: action,
			InstanceID:  publicVersionKey(id),
			ContractID:  "",
			Value:       verBuf,
			Version:     ver + 1,
			DarcID:      darc.ID([]byte{}),
		})
	}
	return scs, nil
}

// verifySignerCounters verifies whether the given counters are valid with
// respect to the current counters.
func verifySignerCounters(st ReadOnlyStateTrie, counters []uint64, ids []darc.Identity) error {
	if len(counters) != len(ids) {
		return xerrors.New("lengths of the counters and signatures are not the same")
	}
	for i, counter := range counters {
		if !ids[i].PrimaryIdentity() {
			return xerrors.New("not a primary identity")
		}
		id := ids[i].String()
		c, err := getSignerCounter(st, id)
		if err != nil {
			return err
		}
		// If c is the highest uint64, then it'll overflow and go back
		// to 0, this is the intended behaviour, otherwise the client
		// will not be able to make more transactions.
		if counter != c+1 {
			return xerrors.Errorf("for pk %s, got counter=%v, but need %v", id, counter, c+1)
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
