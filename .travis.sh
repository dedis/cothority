#!/usr/bin/env bash

# If we're in a branch for gopkg.in, switch to that directory and checkout the
# correct branch
#if [ ${TRAVIS_BRANCH} == 'v0' ]; then
#        GOPKG=$GOPATH/src/gopkg.in/dedis/cothority.v0
#        rm -rf $GOPKG
#        cp -a ../cothority $GOPKG
#        cd $GOPKG
#fi

# do not run any test binary in parallel (see go help build for more info on the -p flag)
go test -race -p=1 ./... || exit 1

# Check for correct formatting
./gofmt.sh || exit 1

go get -u github.com/golang/lint/golint

# Check the linter
./lint.sh || exit 1

# check for missing error handling:
#go get -u github.com/kisielk/errcheck
#- ./errcheck.sh