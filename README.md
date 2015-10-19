# Cothority

The code permits the testing and running of a cothority-system together with the applications. It is split up in 
deployment, application and protocols. The basic cryptographic code comes from DeDiS/crypto. 

## Deploy

    * Deter
    * Localhost
    * Future:
        * Docker
        * LXC

## Applications

    * timestamping
    * signing
    * shamir-secret-service signing
    * shamir-secret-service with tree signing
    * To come:
    	* Randhound - decentrailzed randomness cothority
	* vote - doesn't run yet.
    
## Protocols

    * collective signing

# How to run

The apps are stand-alone (with the correct configuration) in each directory app/*. They can be used with either
the ```localhost```- or the ```deterlab```-deployment. For a simple check on localhost, you can use the following:

```
go get ./...
cd deploy
go build
./deploy -deploy localhost simulation/sign_single.toml
```

## How to run on deterlab

If you use ```-deploy deterlab```, then you will have to enter the name of the deterlab-installation, your username, project- and experiment-name. Furthermore the your public ssh-key has to be installed on the deterlab-site.

For the sake of easy development there are some switches that are to be used only for the
deterlab implementation:

	* -nobuild - don't build any of the helpers - useful if you're working on the main code
	* -build "helper1,helper2" - only build the helpers, separated by a "," - speeds up recompiling

# Applications

## Conode

You will find more information about the conode in it's README:

https://github.com/dedis/cothority/app/conode/README.md

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
