Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
ByzCoinX

# ByzCoinX - improved BFTCoSi

ByzCoinX is an improved version of [BFTCoSi](../bftcosi/README.md). Like
BFTCoSi it is based on a two-round (fault-tolerant) collective signature scheme
but optimized for common use cases with 100-200 nodes.

Unlike BFTCoSi that may operate using a deep tree, making it difficult to
tolerate fault, ByzCoinX operates on a simpler, three level tree. This is
described in [ftCoSi](../ftcosi/README.md). The rest of the protocol is very
similar to BFTCoSi with two signing-rounds: a _prepare_ and a _commit_ round.
During the first round, all nodes are asked whether they are willing to sign,
and during the _commit_ round the actual signature is produced.

## Research Papers

- [PBFT](http://pmg.csail.mit.edu/papers/osdi99.pdf) describes the original
PBFT protocol that is limited to 10-15 nodes
- [ByzCoin](https://arxiv.org/abs/1602.06997) describes the BFTCoSi protocol
and uses it to enhance bitcoin consensus
- [Omniledger](https://eprint.iacr.org/2017/406.pdf) describes the improved
BFTCoSi protocol, called OmniCoin in older versions, and ByzCoinX in newer versions
of the paper
