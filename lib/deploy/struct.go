package deploy

type RunConfig string;

type Platform interface {
	Configure(*Deterlab)
	Build(string) error
	Deploy(RunConfig) error
	Start() error
	Stop() error
}

func NewPlatform() Platform {
	return &Deterlab{}
}
