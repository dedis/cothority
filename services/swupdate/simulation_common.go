package swupdate

import (
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/sda"
)

// InitializePackages sets up all skipchains for the packages in releaseFile and
// returns a slice of string with all packages encountered.
func InitializePackages(releaseFile string, service *Service, roster *sda.Roster, base, height int) ([]string, error) {
	// Read all packages from the releaseFile
	packets := make(map[string]*SwupChain)
	drs, err := GetReleases("snapshot/updates.csv")
	if err != nil {
		return nil, err
	}
	for _, dr := range drs {
		pol := dr.Policy
		log.Lvl1("Building", pol.Name, pol.Version)
		// Verify if it's the first version of that packet
		sc, knownPacket := packets[pol.Name]
		release := &Release{pol, dr.Signatures, false}
		round := monitor.NewTimeMeasure("full_" + pol.Name)
		if knownPacket {
			// Append to skipchain, will build
			service.UpdatePackage(nil,
				&UpdatePackage{sc, release})
		} else {
			// Create the skipchain, will build
			cp, err := service.CreatePackage(nil,
				&CreatePackage{
					Roster:  roster,
					Base:    base,
					Height:  height,
					Release: release})
			if err != nil {
				return nil, err
			}
			packets[pol.Name] = cp.(*CreatePackageRet).SwupChain
		}
		round.Record()
	}
	var packages []string
	for k := range packets {
		packages = append(packages, k)
	}
	return packages, nil
}
