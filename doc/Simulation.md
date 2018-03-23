Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
Simulation

# Simulation

When testing protocols and services, it is useful to have a way to test for more
than 1-10 nodes (which is covered by go-tests). Simulation can do this on
different levers:

- localhost - up to 100 nodes in a more real environment than go-tests
- mininet - using a small number of machines, simulations can be done on up
to 1000 nodes
- deterlab - with good machines a protocol like CoSi can be tested and measured
on up to 50'000 nodes

The goal is to discover different implementation-problems when going from a
small number of nodes to a big number and then fix those problems as they arise.
Simulations are also useful when writing papers and you need to run different
configurations of your protocol or service and measure the time and bandwidth
spent.

## Running a simulation

To run a simulation for CoSi, you can do the following:

```
cd /cosi/simulation
go build
./simulation cosi_verification.toml
```

The result of the simulation will be stored in `test_data` as a csv-file
that can be used for further analysis.

## Simulations available

Here is a list of available simulations in the cothority-code:
- [Collective Signing](../cosi/simulation/README.md)
- [Fault Tolerant Collective Signing](../ftcosi/simulation/README.md)
- [Randhound](../randhound/simulation/README.md)
