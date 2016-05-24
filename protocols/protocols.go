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
	_ "gopkg.in/dedis/cothority.v0/protocols/byzcoin"
	_ "gopkg.in/dedis/cothority.v0/protocols/byzcoin/ntree"
	_ "gopkg.in/dedis/cothority.v0/protocols/byzcoin/pbft"
	_ "gopkg.in/dedis/cothority.v0/protocols/cosi"
	_ "gopkg.in/dedis/cothority.v0/protocols/example/channels"
	_ "gopkg.in/dedis/cothority.v0/protocols/example/handlers"
	_ "gopkg.in/dedis/cothority.v0/protocols/jvss"
	_ "gopkg.in/dedis/cothority.v0/protocols/manage"
	_ "gopkg.in/dedis/cothority.v0/protocols/ntree"
	_ "gopkg.in/dedis/cothority.v0/protocols/randhound"
)
