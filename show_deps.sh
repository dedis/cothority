#!/bin/bash

MODULES="github.com/dedis/onet github.com/dedis/cothority github.com/dedis/onchain-secrets github.com/dedis/kyber"
for mod in ${MODULES} ; do
    modgit=${GOPATH}/src/${mod}/.git
    echo cheking module ${mod} from ${modgit}

    if [ -d ${modgit} ] ; then
        git --git-dir=${modgit} log -1  --format=oneline || true
    fi
done
