package services

import (
	// Importing the services so they register their services to SDA
	// automatically when importing github.com/dedis/cothority/services
	_ "github.com/dedis/cosi/service"
	_ "github.com/dedis/cothority/services/status"
)
