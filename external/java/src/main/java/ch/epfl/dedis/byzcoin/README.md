# ByzCoin notes

## Contracts, instances, and data

In ByzCoin land, all data is stored in instances, where every instance has the following fields:

- data - representing the current state of that instance
- contractId - pointing to the contract responsible for updating this instance
- darcId - pointing to the darc that defines the rules for the contract

Whenever a ClientTransaction is sent to ByzCoin, it contains one or more instructions. Every instruction is sent
to an instance and can be one of the following:

- spawn - creating a new instance
- invoke - calling a method of the contract responsible of the instance
- delete - removing the instance

The first instance in a system is the GenesisDarc that is created and stored in the first block of ByzCoin. This
genesis darc can be evolved (invoke:evolve) to have more rules to create other instance-types. To do this,
two methods exist in a `DarcInstance` object:

- `spawnDarc` to create another darc that will be able to receive instructions
- `spawnInstance` to create a general instance of type `ContractId`, where the original darc needs a rule "spawn:ContractId"

