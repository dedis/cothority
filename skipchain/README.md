# SkipChains

The DEDIS-team is proud to present our very own BFT-blockchain called SkipChains.
It's a permissioned blockchain with backward- and forward-links, and additional
 higher-level links for faster traversal of the chain.
Our main goal is to have a structure that is lightweight and can still prove 
 that a given block is part of the chain.
Instead of doing Proof-of-Work, every skipchain can define its own verification-
 function that is run on all nodes. 
If 2/3 or more of the nodes accept the new block, it is collectively signed
 and appended to the blockchain.
The decision on the signature is done using a BFT-algorithm with our home-grown
 BFT-CoSi [http://arxiv.org/abs/1602.06997].
 
## Programming

If you are interested in programming using skipchains, you can go to the
[Programming.md]-document.

## Applications

We use skipchains for different applications, but we're looking forward for
 any submission of ideas and new usage.
These are the available applications:

- cothority/byzcoin - a system inspired from bitcoin-ng that allows to move
 bitcoin-transactions to 1000s/second
- cothority/identity - a skipchain that can store arbitrary key/value pairs
 that are evolved by a threshold of offline-signatures
- cothority/cisc - Cisc Identity SkipChain holds your ssh-public keys in an
 identity-skipchain and lets your server follow your updates
- chainiac - software-updates putting hashes of the packets on the skipchain
 for easy verification - introduces a multi-layer architecture to allow for
 secure key-updates [http://arxiv.org/chainiac]

## Terminology

There are some terms that are specific to skipchains that are not readily
available in other blockchains:

- BFT-CoSi: Byzantine Fault Tolerant Collective Signing - a two-phase protocol
 described in the ByzCoin-paper [http://arxiv.org/abs/1602.06997] which drives
 the skipchain-consensus-algorithm
- verification function: instead of PoW (proof of work) like Bitcoin and
 Ethereum, skipchains use a verification-function that is run on every node
 of the skipchain - most often the verification-function will check a threshold
 of offline signatures, but it can also verify that a PoW has been done correctly
- backward-links: as in bitcoin and ethereum, each block includes a backward-link
 to the previous block
- forward-links: in addition to backward-links, past blocks will sign off new
 blocks and add collectively signed forward-links. This allows for a delegation
 of trust from one block to the next
- baseHeight / maximumHeight: in skipchains we have more than one link between
 blocks. The baseHeight is the number of blocks between higher-level links.
 The maximumHeight limits how many levels of links will be added to a block.
- parents / children: skipchains can not only be linked horizontally, but also
 hierarchically with parents / children. This is used to have different levels
 of evolution in the skipchains. See Chainiac
- smart contracts: not yet ;)
