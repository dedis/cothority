package swupdate

import (
	"errors"
	"io/ioutil"
	"strings"

	"path"

	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
)

/*
 * Implements the policy-simulation when reading the
 * interpreted debian-snapshot-data.
 */

type DebianRelease struct {
	Snapshot   string
	Time       time.Time
	Policy     *Policy
	Signatures []string
}

var policyKeys []*PGP

func NewDebianRelease(line, dir string) (*DebianRelease, error) {
	entries := strings.Split(line, ",")
	if len(entries) != 3 {
		return nil, errors.New("Should have three entries")
	}
	policy := &Policy{Name: entries[1], Version: entries[2]}
	dr := &DebianRelease{entries[0], time.Now(), policy, []string{}}
	if dir != "" {
		polBuf, err := ioutil.ReadFile(path.Join(dir, policy.Name, "policy-"+policy.Version))
		if err != nil {
			return nil, err
		}
		_, err = toml.Decode(string(polBuf), policy)
		if err != nil {
			return nil, err
		}
		for k := 0; k < policy.Threshold; k++ {
			if k >= len(policyKeys) {
				policyKeys = append(policyKeys, NewPGP())
			}
			pgp := policyKeys[k]
			pub := pgp.ArmorPublic()
			policy.Keys = append(policy.Keys, pub)
		}
		policyBin, err := network.MarshalRegisteredType(policy)
		if err != nil {
			return nil, err
		}
		for i := range policy.Keys {
			sig, err := policyKeys[i].Sign(policyBin)
			if err != nil {
				return nil, err
			}
			dr.Signatures = append(dr.Signatures, sig)
		}
	}
	return dr, nil
}

func GetReleases(file string) ([]*DebianRelease, error) {
	var ret []*DebianRelease
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	dir := path.Dir(file)
	for _, line := range strings.Split(string(buf), "\n") {
		dr, err := NewDebianRelease(line, dir)
		if err == nil {
			ret = append(ret, dr)
		} else {
			log.Warn(err)
		}
	}
	return ret, nil
}
