# OmniLedger

This repo is the development of the omniledger project, that aims to bring
skipchains to a new level of performance and functionality. Broadly speaking,
OmniLedger will implement:

1. multiple transactions per block
2. queuing of transactions at each node and periodical creation of a new
block by the leader
3. allow for verification functions that apply to different kinds of data
4. view-change in case the leader fails
5. sharding of the nodes
6. inter-shard transactions

1-4 are issues that should've been included in skipchains for a long time, but
never got in. Only 5-6 are 'real' omniledger improvements as described in the
[OmniLedger Paper](https://eprint.iacr.org/2017/406.pdf).

# Implementation

## Schedule

We will follow the above list to implement the different parts of OmniLedger.
Once we reach point 3 or 4, we'll start porting services over to the new
omniledger blockchain. As we still want to keep downwards-compatibility, we
probably will need to create new services.

Work on 1. is finished, work on 2. has been started.

To find the current state of omniledger, use the [README](omniledger/README.md).

## Sub-tasks

For 2. to work, we go in steps:
- implement queueing at the leader
- implement queues at the followers
- leader regularly asks followers for new transactions

In addition to this, the ByzCoinX protocol needs to be improved.

### Queueing at the leader

Whenever a leader gets a new transaction request, he puts it in a queue and
waits for further transactions to come in. After a timeout, the leader collects
all transactions and proposes them in a new block to the followers who will sign
the new block by creating a forward-link.

### Queueing at the followers

The followers hold a queue

### ByzCoinX

This protocol handles the consensus algorithm of OmniLedger and is described
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
