# Bypros - Byzcoin Proxy

Bypros can listen on a conode and save the chain in a relational database. It
offers a read-only service call that can execute a provided SQL query.

Apart from listening and executing a query, there is also a service call to
catch up from a given block to the end of the chain.

## Some technical details

### Run postgres in a docker

If you want to locally run a postgresql database with docker:

```sh
docker run -e POSTGRES_PASSWORD=docker -e POSTGRES_USER=bypros -d -p 5432:5432 -v ${PWD}/postgres:/var/lib/postgresql/data postgres
```

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

### Create a readonly user

The service uses a read-only user, which can be created as follow:

```sql
CREATE USER proxy WITH PASSWORD '1234';
GRANT CONNECT ON DATABASE bypros TO proxy;
GRANT USAGE ON SCHEMA cothority TO proxy;
GRANT SELECT ON ALL TABLES IN SCHEMA cothority TO proxy;
```
