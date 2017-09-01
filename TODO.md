# Items that are still waiting for completion

## Handling of public keys

Now the public keys, the documents and the symmetric keys are
all stored in the same skipchain. If the public keys need to be
separated, we'd have to re-enable this feature.

## Improving skipchains

The current implementation of skipchains lacks in multiple ways:

- saving the data is done as a big blob - needs a database
	- needs to rewrite part of the underlying framework
	- 1 week
- all blocks are held in memory - out of memory error if too many blocks exist
	- `bunch` needs to allow for dropping unused blocks
	- once the database is in place, 1 day
