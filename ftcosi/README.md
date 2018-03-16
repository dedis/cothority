Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
Fault Tolerant Collective Signing

# Fault Tolerant Collective Signing

This package provides functionality to request and verify collective signatures
as well as run a standalone server for handling collective signing requests.
It is a fault tolerant version of CoSi, implemented in
[cosi](../cosi/README.md) package.

## Research Paper

It is the basis for the ByzCoinX protocol. For further background and technical
details, please refer to the
[research paper](https://eprint.iacr.org/2017/406.pdf).

## Description

The purpose of this work is to implement a robust and scalable consensus
algorithm using ftCoSi protocol and handling some exceptions. The ftCoSi tree
is a three level tree to make a compromise between the two-level tree, making
the root-node vulnerable to DoS, and a more than three level tree, slowing the
algorithm because of the RTT between the root node and the leaves.

The tree is composed of a leader (root-node), and some groups of equal size,
each having a sub-leader (second level nodes) and members (leaves). The group
composition are defined by the leader.

Ideally, we want to handle non-responding nodes, no matter where they are
in the tree. If a leaf is failing, then it is ignored in the ftCoSi commitment.
If a sub-leader is non-responding, then the leader (root node) recreates the
group by selecting another sub-leader from the group members. And finally, if
the leader is failing, the protocol restarts using another leader. At the
moment, however, we only handle leaf and sub-leader failure.

More complex adversaries (modifying messages, non-responding at challenge time,
etc.) are not yet handled.
The purpose of the project is to test scalability and robustness of this
service on a testbed and to have a well-documented reusable code for it.

- [ftCoSi CLI](CLI.md) is a command line interface for interacting with ftCoSi
- [ftCoSi protocol](protocol) the protocol used for collective signing
- [ftCoSi service](service) the service with the outward looking API
