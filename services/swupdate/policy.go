package swupdate

import (
	"errors"
	"io/ioutil"
	"strings"

	"path"

	"time"

	"github.com/BurntSushi/toml"
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

func NewDebianRelease(line, dir string, keys int) (*DebianRelease, error) {
	entries := strings.Split(line, ",")
	if len(entries) != 4 {
		return nil, errors.New("Should have five entries" + line)
	}
	policy := &Policy{Name: entries[1], Version: entries[2]}
	// //	Mon Jan 2 15:04:05 -0700 MST 2006
	t, err := time.Parse("20060102150405", entries[0])
	if err != nil {
		return nil, err
	}
	dr := &DebianRelease{entries[0], t, policy, []string{}}
	if false {
		if dir != "" {
			polBuf, err := ioutil.ReadFile(path.Join(dir, policy.Name, "policy-"+policy.Version))
			if err != nil {
				return nil, err
			}
			_, err = toml.Decode(string(polBuf), policy)
			if err != nil {
				return nil, err
			}
		}
	} else {
		policy.Threshold = keys
		policy.BinaryHash = entries[3]
		//policy.SourceHash = entries[4]
	}

	key := NewPGP()
	for k := 0; k < policy.Threshold; k++ {
		if k >= len(policyKeys) {
			policyKeys = append(policyKeys, key)
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

	return dr, nil
}

func GetReleases(file string) ([]*DebianRelease, error) {
	return GetReleasesKey(file, 5)
}

func GetReleasesKey(file string, keys int) ([]*DebianRelease, error) {
	var ret []*DebianRelease
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	dir := path.Dir(file)
	for _, line := range strings.Split(string(buf), "\n")[1:] {
		dr, err := NewDebianRelease(line, dir, keys)
		if err == nil {
			ret = append(ret, dr)
			//} else {
			//	log.Error(err, line)
		}
	}
	return ret, nil
}
