/*
Package protocol contains an implementation of the CoSi protocol as described in the paper
"Keeping Authorities "Honest or Bust" with Decentralized Witness Cosigning"

The protocol has four messages:
	- Announcement which is sent from the root down the tree and announce the proposal
	- Commitment which is sent back up to the root, containing an aggregated commitment from all nodes
	- Challenge which is sent from the root down the tree and contains the aggregated challenge
	- Response which is sent back up to the root, containing the final aggregated signature, then used by the root to sign the proposal

The protocol uses five files:
- struct.go defines the messages sent around and the protocol constants
- protocol.go defines the root node behavior
- subprotocol.go defines non-root nodes behavior
- gen_tree.go contains the function that generates trees
- helper_functions.go defines some functions that are used by both the root and the other nodes

The package protocol_tests contains unit tests testing the package's code.

Under-the-hood, there are two protocols. A main protocol which only runs on the
root node and a sub-protocol that runs on all nodes (including the root). The
tree structure of the sub-protocol is illustrated below.
     root
       |
       |
   sub-leader
    /       \
   /         \
leaf_1 ... leaf_n
Namely, if there are m sub-leaders, the root will run m sub-protocols. The sub-
protocols do bulk of the work (collective signatures) and communicates the
result to the main protocol via channels.
*/
package protocol
