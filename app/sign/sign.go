package sign

import "github.com/dedis/cothority/deploy"

// Dispatch-function for running either client or server (mode-parameter)
func Run(mode string, conf *deploy.Config) {
	// Do some common setup
	switch mode{
	case "client":
		RunClient(conf)
	case "server":
		RunServer(conf)
	}
}
