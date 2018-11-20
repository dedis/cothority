# Cothority Helm Chart

Provides a [helm](https://helm.sh/) chart for running a conode.

How to use this:

## 1. Generating the key pair for the conode

```bash
docker run -it --rm dedis/conode:latest bash
./conode setup # Use defaults
cat /root/.config/conode/private.toml
```

Copy the file contents into a fresh values file (e.g. `./your-values.yaml`)
and edit the `Address` and `Description` fields.

## 2. Deploy the conode

First, make the helm chart:
```bash
helm package conode # From this folder
```

Then deploy it:
```bash
helm upgrade --install conode conode-0.1.0.tgz \
    --namespace conode \
    -f your-values.yaml
```
