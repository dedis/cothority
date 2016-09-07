package services

import (
	// Importing the services so they register their services to SDA
	// automatically when importing github.com/dedis/cothority/services
	// XXX We should not import that doing weird cross dependancy between
	// packages
	//	_ "github.com/dedis/cosi/service"
	_ "github.com/dedis/cothority/services/guard"
	_ "github.com/dedis/cothority/services/identity"
	_ "github.com/dedis/cothority/services/skipchain"
	_ "github.com/dedis/cothority/services/status"
)
