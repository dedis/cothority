Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/master/doc/BuildingBlocks.md) ::
[OmniLedger](README.md) ::
Child Transactions

# Child Transactions

This is an overview of how the transactions can be also accepted by child nodes
and how the leader can fetch all new transactions from the children.

## Transaction Queue and Block Generation

This part of the document describes the technical details of the design and
implementation of transaction queue and block generation for OmniLedger. The
assumption is that the leader will not fail. We will implement view-change in
the future, starting by eliminating stop-failure and then byzantine-failure.
Further, we assume there exists a maximum block size of B bytes. A transaction
is similar to what is defined above, namely a key/kind/value triplet and a
signature of the requester (client). However, for bookkeeping purposes, we add a
private field: "status". A status can be one of three states: "New" | "Verified" |
"Submitted". The transaction request message is the same as the Transaction
struct above, e.g.

```
message TransactionRequest {
  Transaction Transaction = 1;
}
```

TransactionRequest can be sent to any conode. The client should send it to
multiple conodes if it suspects that some of the conode might fail or are
malicious. On the conode, it will store all transactions that it received, in a
queue in memory, with the initial state being "New". We use a slice with a mutex
as the implementation for the queue. If the total size of the queue exceeds B
bytes (we may need to adjust this to support a large backlog), then the conode
responds to the client with a failure message, otherwise a success message. The
client should not see the success message as an indication that the transaction
is included in the block, but that the transaction is received and may be
accepted into a block. We do not attempt to check whether the transaction is
valid at this point because the conode’s darc database might not be up-to-date,
for example if it just came back online.  

### Block Generation

The poll method is inspired by the
beta synchroniser, where the leader sends a message, e.g.

```
message PollTxRequest{
  bytes LatestBlockID = 1;
}
```

down the communication tree, and then every
node will respond with a type

```
message PollTxResponse {
  repeated Transaction Txs = 1;
}
```

The transactions are combined on the subleader nodes.

However, before sending the `PollTxResponse` message, the conodes must check that
the state of omniledger does include the transactions in the latest block
given by the id in `PollTxRequest`. If the state is not
up-to-date, then the nodes must do an update to ensure it is. Then, the nodes
verify `Transaction.Signature` to make sure that all transaction in their queue are
valid. The valid transactions are marked as "Verified" and the bad transactions
are dropped and a message is printed to the audit log. Finally, the transactions
with the "Verified" flag are sent to the leader in the `PollTxResponse` message.
These transactions are marked as "Submitted". The `PollTxResponse` message should
not be larger than B bytes.

Upon receiving all the `PollTxResponse` message, the leader does the following:
- remove duplicates
- verify signature
- shuffle the transactions in a deterministic way
- go through the list of transactions, and for each transaction mark if it
applies correctly to the state updated with previous valid transactions

Then the leader creates the block and then sends it to the conodes to cosign, e.g.

```
message BlockProposal {
  Data Data = 1;
}
```

The conodes run the same checks and need to make sure that the transactions are
in the same state as marked by the leader. If this is the case, they sign the
hash of the proposed block.

The new block, with a collective signature, is propagated back to all nodes.
Then every node updates their queue and removes the transactions that are in the
new block. For the transactions that were not added to the new block, they need
to be moved to the front of the queue and marked as "New" because the state
of omniledger may have changed and the old transactions may become invalid. All the
"Verified" transactions must also be changed back to "New".

### Additional blocks

What we described above is how to generate a single block, how do we run it
multiple times? A simple solution would be for the leader to send a
`PollTxRequest` after every new block is generated. However, it results in a lot
of wasted messages if there are very little or no transactions. We can attempt
to implement the simplest technique first and then try to optimise it later. For
example, a slightly better version would be to add some delay when the blocks
are getting smaller. But this is only a heuristic because the leader does not
know how many transactions are in the queues of the non-root nodes.

Another option would be that each node sends _one_ message to the leader if it
has not-included transactions. This could happen when:
- the queue has been empty and a first transaction comes in
- a new block has been accepted, but not all transaction of the queue are in
that block
This would be halfway between only depending on the leader and sending _all_
transactions to the leader.
