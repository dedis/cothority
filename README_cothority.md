# Collective Authority (Cothority)

The Cothority project offers a framework for research, simulation and deployment 
of crypto-related protocols with an emphasis of decentralized, distributed protocols.
This repository holds protocols, services and apps for you to use.

Cothority is developed and maintained by [DEDIS/EFPL](http://dedis.epfl.ch) and
put under a dual-license GNU/AGPL 3.0 and a commercial license. The code is
available in the `cothority`-organisation under github. It builds on the following
two libraries in the `dedis`-organisation:

- [Crypto](https://github.com/dedis/crypto) - Crypto-library
- [Protobuf](https://github.com/dedis/protobuf) - Protobuf-library

## Documentation

The documentation for Cothority is split into three parts:

- To run and use a conode, have a look at 
	[Cothority Node](https://github.com/cothority/conode/wiki)
	with examples of protocols, services and apps
- To start a new project and develop and integrate a new protocol have a look at
	the [Cothority Template](https://github.com/cothority/template/wiki)
- To participate as a core-developer, go to 
	[Cothority Network Library](https://github.com/cothority/conet/wiki)

# Old projects

If you have a project depending on the old `dedis/cothority`, you can simply
replace all occurrences of `dedis/cothority` with `cothority/conode` and use
the corresponding branch.

We strongly encourage you to move your project to use the new `cothority/conet`-
library as available through gopkg: `gopkg.in/cothority/conet.v1`. This is
a stable release and only bugfixes will be added. The development continues in
the master-branch.

# Contact

You can contact us at https://groups.google.com/forum/#!forum/cothority
