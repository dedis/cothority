Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Simulation](../doc/Simulation.md) ::
Collective Signing

# Collective Signing

Simulation of CoSi includes code that builds a tree and then measures the time
it takes to create a collective signature. You can run it with:

```
cd $(go env GOPATH)/src/github.com/dedis/cothority/cosi/simulation
go build
./simulation cosi_verification.toml
```
