Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
BLS Collective Signing

# BLS Collective Signing

This package provides functionality to request and verify collective signatures
as well as run a standalone server for handling collective signing requests.
It is a fault tolerant version of CoSi, implemented in
[cosi](../cosi/README.md) package using the Boneh-Lynn-Shacham (BLS) signature
algorithm.

## Research Paper

It is the basis for the ByzCoinX protocol. For further background and technical
details, please refer to the
[research paper](https://eprint.iacr.org/2017/406.pdf).

You can read more about BLS signature by refering to
[the paper](https://crypto.stanford.edu/~dabo/pubs/papers/BLSmultisig.html).

## Description

The purpose of this work is to implement a robust and scalable consensus
algorithm using blscosi protocol and handling some exceptions. The tree
is a three level tree to make a compromise between the two-level tree, making
the root-node vulnerable to DoS, and a more than three level tree, slowing the
algorithm because of the RTT between the root node and the leaves.

The tree is composed of a leader (root-node), and some groups of equal size,
each having a sub-leader (second level nodes) and members (leaves). The group
composition are defined by the leader.

Ideally, we want to handle non-responding nodes, no matter where they are
in the tree. If a leaf is failing, then it is ignored in the blscosi commitment.
If a sub-leader is non-responding, then the leader (root node) recreates the
group by selecting another sub-leader from the group members. And finally, if
the leader is failing, the protocol restarts using another leader. At the
moment, however, we only handle leaf and sub-leader failure.

## Implementation
The protocol has three messages: 
- Announcement which is sent from the root down the tree and announce the
proposal. 
- Response which is sent back up to the root, containing the final aggregated
signature, then used by the root to sign the proposal.
- Refusal which is sent back to subleader to let them know the leaf has
failed the verification.

The protocol uses four files: 
- `struct.go` defines the messages sent around and the protocol constants.  
- `protocol.go` defines the root node behavior.
- `sub_protocol.go` defines non-root nodes behavior.
- `gen_tree.go` contains the function that generates trees.

Under-the-hood, there are two protocols. A main protocol which only runs on
the root node and a sub-protocol that runs on all nodes (including the
root). The tree structure of the sub-protocol is illustrated below.

```
       root
         |
         |
     sub-leader
      /       \
     /         \
  leaf_1 ... leaf_n
```

Namely, if there are m sub-leaders, the root will run m sub-protocols. The
sub- protocols do bulk of the work (collective signatures) and communicates
the result to the main protocol via channels.

- [BlsCosi CLI](blscosi/README.md) is a command line interface for interacting with blscosi
- [BlsCoSi protocol](protocol) the protocol used for collective signing
