# Chainiac

Chainiac is a multi-level blockchain architecture that allows for secure
evolution of keys and nodes in a way that there is no single point of failure.
The system uses three levels of blockchains that are tied together:

- root-skipchain: a basic, slowly-evolving skipchain (a couple of times a year), 
 that only adds new blocks if they are signed by offline keys
- control-skipchain: faster-evolving skipchain (once or twice a week) that
 represents the active list of nodes available for the blockchain
- data-skipchain: the active blockchain holding data, either stateful or
 transaction data
 
In a future version, we'd like to add access-control to those chains, so that
 you can give rights to create new data-skipchains.

