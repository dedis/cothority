package services

// Importing the services so they register their services to SDA
// automatically when importing github.com/dedis/cothority/services

import (
	_ "github.com/dedis/cosi/service"
	_ "github.com/dedis/cothority/services/cosimul"
)
