Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
Broadcast and Propagation

# Broadcast and Propagation

As the onet library works in a tree, only primitives to send to the parent or the
children are implemented. The broadcast protocol makes sure that all nodes will
receive a message and return how many actually confirmed the reception of the
message.

Another problem some services need to solve is to share data and propagate it
to other blocks. This is what is done in the propagation protocol where a
leader sends data to all other nodes which will confirm the correct reception
of the data. At the end the protocol will return how many nodes did receive
the data.
