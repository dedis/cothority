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
    
# How to run

For the moment only the timestamping on Deterlab works:

In the top-level directory, type

```
go get ./...
go build
./cothority
```

then enter the name of the deterlab-installation, your username and your project-name, and you should
be ready to go. The arguments are:

	* -debug - number between 0 and 5 - 0 is silent, 5 is very verbose
	* -deploy [deterlab,gochannels] - by default is "deterlab" - gochannels are next
	* -app [server,client] - whether to run the application as server or client - not yet implemented

For the sake of easy development there are some switches that are to be used only for the
deterlab implementation:

	* -nobuild - don't build any of the helpers - useful if you're working on the main code
	* -build "helper1,helper2" - only build the helpers, separated by a "," - speeds up recompiling
	* -machines # - tells how many machines are to be used for the run


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

