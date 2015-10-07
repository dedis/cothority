# Cothority

The code permits the testing and running of a cothority-system together with the applications. It is split up in 
deployment, application and protocols. The basic cryptographic code comes from DeDiS/crypto. The following modules
are used:

Deploy

    * Deter - running
    * Localhost
    * Future:
	    * Go-routines - in preparation
        * Docker
        * LXC

Applications

    * timestamping
    * signing
    * shamir-signing
    * To come:
    	* Randhound - decentrailzed randomness cothority
	    * vote - doesn't run yet.
    
Protocols

    * collective signing
    * shamir signing
    
# How to run

The apps will be stand-alone (not yet tested) in each directory app/*. For the moment they
can be run only through the deterlab-testbed in the ```deploy```-directory:

```
go get ./...
cd deploy
go build
./deploy runconfig/sign_single.toml
```

then enter the name of the deterlab-installation, your username, project- and experiment-name, and you should
be ready to go. The arguments are:

	* runconfig - any .toml-file in the runconfig/-directory
	* -debug - number between 0 and 5 - 0 is silent, 5 is very verbose

For the sake of easy development there are some switches that are to be used only for the
deterlab implementation:

	* -nobuild - don't build any of the helpers - useful if you're working on the main code
	* -build "helper1,helper2" - only build the helpers, separated by a "," - speeds up recompiling
	* -machines # - tells how many machines are to be used for the run

# Deployment
	Configure(*Config)
	Build() (error)
	Deploy() (error)
	Start() (error)
	Stop() (error)

The Life of a simulation:

1. Configure
	* Prepare global specific configuration (deterlab or localhost)
2. Build
    * builds all files for the target platforms
3. Deploy
    * make sure the environment is up and running
    * prepare configuration for the app to run
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

It sets up servers that listen for client-requests, collect all
requests and handles them to a root-node.

## Signing

A simple mechanism that only receives a message, signs it, and returns it.

## Randhound

Test-implementation of a randomization-protocol based on the cothority

# Protocols

We want to compare different protocols for signing and timestamping uses.

## Collective signing

This one runs well and is described in a pre-print from Dylan Visher.

## Shamir signing

A textbook shamir signing for baseline-comparison against the collective signing protocol.
