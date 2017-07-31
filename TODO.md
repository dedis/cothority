# Items that are still waiting for completion

## Actual re-encryption of symmetric encryption key

The core of the onchain-secrets algorithm is not done yet: the symmetric
key is stored as-is on the skipchain. We have an implementation of Lefteris'
paper-draft, but still need to port it as a 'protocol'.

Estimated time: 2 days

## Using of new skipchains

The current implementation of skipchains lacks in multiple ways:

- parallel writing/reading to the skipchain is not handled correctly
	- done in development version
	- 1 day of porting
- saving the data is done as a big blob - needs a database
	- needs to rewrite part of the underlying framework
	- 1 week
- all blocks are held in memory - out of memory error if too many blocks exist
	- `bunch` needs to allow for dropping unused blocks
	- once the database is in place, 1 day
