Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
BftCoSi

# Byzantine Fault Tolerant CoSi

*WARNGING* - BFTCoSi is deprecated, please use [ByzCoinX](../byzcoinx).

If you use a single CoSi round to reach consensus, there are a number of things
that might go wrong:
- If any of the node stalls the final phase of the signature, you cannot start
another round without risking that node to finalize the signature later.
- If a node drops out, you cannot blame him without risking that the blamer
itself blames a honest node.

Using PBFT we can solve this problem, but then we cannot use more than 10-15
nodes, because PBFT needs to broadcast all messages and thus incurs a O(n^2)
communication cost.

To solve these problems, we introduce BFTCoSi, a protocol based on PBFT and that
uses two rounds of CoSi. The first round is to assure that all the nodes are
willing to sign the data. Only if a threshold of nodes give their agreement
will the protocol proceed to the second round and get the actual signature on
the data.
If a node tries to freeze-attack in the first round, nothing is lost, and the
protocol can be restarted excluding that node. Even if he tries later to
validate the signature, it will not be accepted, as it's only the first round.
If a node agrees in the first round to participate, but drops out in the second
round, he can be blamed and everybody can verify that indeed he did agree to
sign, but then he refused to do so in the second round.
Instead of broadcasting all messages, the signature requests are sent through
a tree-structure which reduces the communication cost to O(n).

## Research Papers

- [PBFT](http://pmg.csail.mit.edu/papers/osdi99.pdf) describes the original
PBFT protocol that is limited to 10-15 nodes
- [ByzCoin](https://www.usenix.org/system/files/conference/usenixsecurity16/sec16_paper_kokoris-kogias.pdf)
  describes the BFTCoSi protocol and uses it to enhance bitcoin consensus
