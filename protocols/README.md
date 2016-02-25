#Protocols

## Byzcoin 

XXX Need references.
ByzCoin implements the ByzCoin protocols which consists of two CoSi Protocols:
* One Prepare phase where the block is dispatched and signed amongst the tree.
* One Commit phase where the previous signature is seen by all the participants
  and they make a final one out of it.
This implicitly implements a Byzantine Fault Tolerant protocol.

## PBFT

PBFT is the Practical Fault Tolerant algorithm described
in http://pmg.csail.mit.edu/papers/osdi99.pdf .
It's a very simple implementation just to compare the performance relative to
ByzCoin.

## Ntree

Ntree is like ByzCoin but with independant signatures. There is no Collective
Signature (CoSi) done. It has been made to compare the perform relative to
ByzCoin.


