Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../../README.md) ::
[Simulation](../../doc/Simulation.md) ::
Fault Tolreant Collective Signing

# Fault Tolerant Collective Signing

Simulation of blscosi includes code that builds a tree and then measures the time
it takes to create a collective signature. You can run it with:

```
cd $(go env GOPATH)/src/github.com/dedis/cothority/blscosi/simulation
go build
./simulation local.toml
```
