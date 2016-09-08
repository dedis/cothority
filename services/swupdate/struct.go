package swupdate

import (
	"github.com/dedis/cothority/sda"
	"github.com/satori/go.uuid"
)

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
