package schnorr_sign

import (
"github.com/dedis/cothority/deploy"
"github.com/dedis/cothority/lib/config"
)

// Dispatch-function for running either client or server (mode-parameter)
func Run(app *config.AppConfig, conf *deploy.Config) {
	// Do some common setup
	switch app.Mode{
	case "client":
		RunClient(conf)
	case "server":
		RunServer(conf)
	}
}
