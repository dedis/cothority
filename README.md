# Cothority

The code permits the testing and running of a cothority-system together with the applications. It is split up in 
deployment, application and protocols. The basic cryptographic code comes from DeDiS/crypto. The following modules
are used:

Deploy
    * Deter - running
    * Go-routines - in preparation
    * Future:
        * Docker
        * LXC

Applications
    * timestamping
    * signing - needs to collect more data
    * vote - doesn't run yet.
    
Protocols
    * collective signing
    * joint threshold signing - work in progress

# Deployment
	Configure(*Config)
	Build() (error)
	Deploy() (error)
	Start() (error)
	Stop() (error)

The Life of a simulation:

1. Configure
    * read configuration
    * compile eventual files
2. Build
    * builds all files
    * eventually for different platforms
3. Deploy
    * make sure the environment is up and running
    * copy files
4. Start
    * start all logservers
    * start all nodes
    * start all clients
5. Stop
    * abort after timeout OR
    * wait for final message
6. Stats - work in progress
    * copy everything to local
    
# Applications

## Timestamping

For the moment the only running application - it sets up servers that listen for client-requests, collect all
requests and handle them to a root-node.

## Signing

A simple mechanism that only receives a message, signs it, and returns it.

## Voting

Not done yet

# Protocols

We want to compare different protocols for signing and timestamping uses.

## Collective signing

This one runs well and is described in a pre-print from Dylan Visher.

## Join threshold signing

A baseline-comparison being developed by the DeDiS-lab at EPFL.
