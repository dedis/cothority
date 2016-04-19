package protocols

/*
Only used to include the different protocols
*/

import (
	// Don't forget to "register" your protocols here too
	_ "github.com/dedis/cothority/protocols/byzcoin"
	_ "github.com/dedis/cothority/protocols/byzcoin/pbft"
	_ "github.com/dedis/cothority/protocols/cosi"
	_ "github.com/dedis/cothority/protocols/example/channels"
	_ "github.com/dedis/cothority/protocols/example/handlers"
	_ "github.com/dedis/cothority/protocols/jvss"
	_ "github.com/dedis/cothority/protocols/manage"
	_ "github.com/dedis/cothority/protocols/medco"
)
