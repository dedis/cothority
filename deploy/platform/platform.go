package platform

type RunConfig string

type Platform interface {
	Configure()
	Build(string) error
	Deploy(RunConfig) error
	Start() error
	Stop() error
}

var deterlab string = "deterlab"
var localhost string = "localhost"

// Return the appropriate platform
// [deterlab,localhost]
func NewPlatform(t string) Platform {
	var p Platform
	switch t {
	case deterlab:
		p = &Deterlab{}
	case localhost:
		p = &Localhost{}
	}
	return p
}
