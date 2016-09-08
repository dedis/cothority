package swupdate

import (
	"github.com/dedis/cothority/sda"
	"github.com/satori/go.uuid"
)

type Package struct {
	Policy     *Policy
	Signatures []string
}

func NewPackage(p *Policy, s []string) *Package {
	return &Package{
		Policy:     p,
		Signatures: s,
	}
}

type Policy struct {
	Name    string
	Version string
	// Represents how to fetch the source of that version -
	// only implementation so far will be deb-src://, but github://
	// and others are possible.
	Source     string
	Keys       []string
	Threshold  int
	BinaryHash string
}

func NewPolicy(line string) *Policy {
	return &Policy{}
}

type ProjectID uuid.UUID

type CreateProject struct {
	*sda.Roster
	*Policy
}

type CreateProjectRet struct {
	ProjectID
}

type SignBuild struct {
	Data string
}

type SignBuildRet struct {
	OK bool
}
