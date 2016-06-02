/*
The storage point of all protocols that Cothority can run.

If you want to add a new protocol, chose one of example/channels or
example/handlers and copy it to a new directory under protocols.
Adjust all names and implement your protocol. You can always test it
using the _test.go test.

For simulating your protocol, insert the include-path below, so that the
Cothority-framework knows about it. Now you can copy one of
simul/runfiles/test_channels.toml, adjust the Simulation-name and
change the parameters to your liking. You can run it like any other
simulation now:

 	cd simul
 	go build
 	./simul runfiles/test_yourprotocol.toml
 	matplotlib/plot.py test_data/test_yourprotocol.csv

Don't forget to tell us on the cothority-mailing list about your
new protocol!
*/
package protocols

/*
Only used to include the different protocols
*/

import (
	// Don't forget to "register" your protocols here too
	_ "github.com/dedis/cothority/protocols/byzcoin"
	_ "github.com/dedis/cothority/protocols/byzcoin/ntree"
	_ "github.com/dedis/cothority/protocols/byzcoin/pbft"
	_ "github.com/dedis/cothority/protocols/cosi"
	_ "github.com/dedis/cothority/protocols/example/channels"
	_ "github.com/dedis/cothority/protocols/example/handlers"
	_ "github.com/dedis/cothority/protocols/jvss"
	_ "github.com/dedis/cothority/protocols/manage"
	_ "github.com/dedis/cothority/protocols/ntree"
	_ "github.com/dedis/cothority/protocols/prifi"
	_ "github.com/dedis/cothority/protocols/randhound"
)
