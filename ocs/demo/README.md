Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Applications](../doc/Applications.md) ::
[Onchain-Secrets](../README.md) ::
Demo

# OCS Demo

This demo does a simple run to show how to use the OCS with the X509
certificates. To run it, you first need to run the docker image
to start 3 nodes locally:

```bash
docker run -it -p 7770-7775:7770-7775 --rm -v$(pwd)/data:/conode_data -e COTHORITY_ALLOW_INSECURE_ADMIN=true c4dt/ocs:dev ./run_nodes.sh -n 3 -v 2 -c -d /conode_data
```

This creates 3 nodes that are listening on the localhost using the ports 7770-7775.
All data is stored in the `$(pwd)/data` directory. Once the nodes are up and running,
the demo can be started:

```bash
cd cothority/ocs/demo
go run main.go
```

The demo will do the following:

1. set up a root CA that is stored in the service as being allowed to create new OCS-instances
2. Create a new OCS-instance with a reencryption policy being set by a node-certificate
3. Encrypt a symmetric key to the OCS-instance public key
4. Ask the OCS-instance to re-encrypt the key to an ephemeral key
5. Decrypt the symmetric key

All communication is done over the network, the same way as it has to be done in
a real system.
