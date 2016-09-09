package swupdate

import (
	"errors"
	"io/ioutil"
	"strings"

	"path"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
)

/*
 * Implements the policy-simulation when reading the
 * interpreted debian-snapshot-data.
 */

type DebianRelease struct {
	Snapshot   string
	Name       string
	Version    string
	Policy     *Policy
	Signatures []string
}

var policyKeys []*PGP

func NewDebianRelease(line, dir string) (*DebianRelease, error) {
	entries := strings.Split(line, ",")
	log.Print(entries)
	if len(entries) != 3 {
		return nil, errors.New("Should have three entries")
	}
	dr := &DebianRelease{entries[0], entries[1], entries[2],
		&Policy{}, []string{}}
	if dir != "" {
		polBuf, err := ioutil.ReadFile(path.Join(dir, dr.Name, "policy-"+dr.Version))
		if err != nil {
			return nil, err
		}
		_, err = toml.Decode(string(polBuf), dr.Policy)
		if err != nil {
			return nil, err
		}
		for k := 0; k < dr.Policy.Threshold; k++ {
			if k >= len(policyKeys) {
				policyKeys = append(policyKeys, NewPGP())
			}
			pgp := policyKeys[k]
			pub := pgp.ArmorPublic()
			dr.Policy.Keys = append(dr.Policy.Keys, pub)
			sig, err := pgp.Sign([]byte(dr.Policy.SourceHash))
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
