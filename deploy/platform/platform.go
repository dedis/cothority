package platform

type RunConfig string;

type Platform interface {
	Configure()
	Build(string) error
	Deploy(RunConfig) error
	Start() error
	Stop() error
}

func NewPlatform() Platform {
	return &Deterlab{}
}
