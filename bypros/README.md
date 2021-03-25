# Bypros - Byzcoin Proxy

Bypros can listen on a conode and save the chain in a relational database. It
offers a read-only service call that can execute a provided SQL query.

Apart from listening and executing a query, there is also a service call to
catch up from a given block to the end of the chain.

This service is meant to be deployed and run in a sidecar pattern to a node, as
it targets only a specific node and not a roster, as traditionally done with
cothority services. This implies that you should manage and fully trust the node
you are talking to. The proxy reflects the state of a specific node, and not
necessarily the "chain" represented by the collective authority. Note that this
limitation can be alleviated with a layer handling connection to multiple nodes
(roster) instead of only one, but this is out of scope of this service.

## Limitations

The proxy should be used to target only one skipchain. Upon a first call to
follow or catch up, the service saves the skipchain ID and prevents any call to
another skipchain ID.

A single "follow" is allowed at the same time as this operation, once requested,
continuously runs in the background until a request to "unfollow" is sent. An
error is thrown if one tries to follow while the system is already following.

## Some technical details

### Run postgres in a docker

Running the database with docker is only recommended in development
environments.

Use the dockerfile in `storage/sqlstore` and follow the instructions written in
it.

### Export the database urls

Proxy expects 2 databases urls: one with read/write rights, and another with
only read rights.

```sh
export PROXY_DB_URL="postgres://bypros:docker@localhost:5432/bypros"
export PROXY_DB_URL_RO="postgres://proxy:1234@localhost:5432/bypros"
```

### Example: select unexecuted deferred instance IDs

This is the kind of query that can be sent:

```sql
select encode(instruction.contract_iid::bytea, 'hex'), instruction.contract_name from cothority.instruction
where instruction.action = 'spawn:deferred'
and instruction.contract_iid not in (
	select instruction.contract_iid from cothority.instruction
	join cothority.transaction on
		transaction.transaction_id = instruction.transaction_id
	where transaction.accepted = true and
	instruction.action = 'invoke:deferred.execProposedTx'
	)
group by instruction.contract_iid, instruction.contract_name
```
