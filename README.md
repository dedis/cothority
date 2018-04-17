# Omniledger

This repo is the development of the omniledger project, that aims to bring
skipchains to a new level of performance and functionality. Broadly speaking,
Omniledger will implement:

1. multiple transactions per block
2. allow for verification functions that apply to different kinds of data
3. queuing of transactions at each node and periodical creation of a new
block by the leader
4. view-change in case the leader fails
5. sharding of the nodes
6. inter-shard transactions

1-4 are issues that should've been included in skipchains for a long time, but
never got in. Only 5-6 are 'real' omniledger improvements as described in the
[Omniledger Paper](https://eprint.iacr.org/2017/406.pdf).

# Implementation

## Schedule

We will follow the above list to implement the different parts of Omniledger.
Once we reach point 3 or 4, we'll start porting services over to the new
omniledger blockchain. As we still want to keep downwards-compatibility, we
probably will need to create new services.

Currently work on 1. is ongoing

## Sub-tasks

For 1. to work, there are two libraries that need to be done correctly:
- Darc - to define the access control
- Collections - to handle the Merkle tree holding all the data

In addition to this, the ByzCoinX protocol needs to be improved.

### Darc

Kelong is looking into Darc and is working on rewriting the policy mechanism
that allows for AND, OR, NOT and THRESHOLD keywords, to combine signatures from:
- DarcIdentity - a link to another darc that is allowed to sign
- Ed25519 - our cryptographic work-horse
- X509 EC - a more general place holder for cryptographic signatures

### Collections

Raphael did a big cleanup of the collections library to be understandable (putting
the documentation in the functions) and to follow the go-standard.

Sooner or later we'll need to think of how to hold the tree in a database instead
of keeping it in memory.

### ByzCoinX

This protocol handles the consensus algorithm of Omniledger and is described
in the paper. One thing that is missing in the paper is possible improvements
to make the protocol more usable in a real-world environment:
- sub-leaders propagate the commit once a threshold of leafs replied
- the leader can accept to go on if there are enough commits from the subleaders
to reach 2/3 consensus with a high probability

## People

This effort is lead by the following people:
- Semester students:
  - Pablo Lorenceau - working on multpile transactions per block
  - Raphael Dunant - improving ByzCoinX
- Engineers:
  - Linus Gasser - documentation and communication
  - Kelong Cong - working on Darc
  - Jeff Allen - testing and code review
- PhD students:
  - Eleftherios Kokoris - making sure we don't do stupid things
