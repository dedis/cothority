#!/bin/bash

## install.sh will fetch all dependencies of the project.
## If the branch from where the PR comes from starts with
## `refactor_`, it will try to checkout the same branch name 
## in dedis/cosi.
export PR=https://api.github.com/repos/$TRAVIS_REPO_SLUG/pulls/$TRAVIS_PULL_REQUEST
export BRANCH=$(echo `curl -s $PR | jq -r .head.ref`)

echo "TRAVIS_BRANCH=$TRAVIS_BRANCH, PR=$PR, BRANCH=$BRANCH"

pattern="refactor_*"; 
if [[ $BRANCH =~ $pattern ]]; then 
    echo "Using refactor branch ..."
    repo=github.com/dedis/cosi; 
    go get $repo; 
    cd $GOPATH/src/$repo; 
    git checkout -f $BRANCH; 
    git pull;
    echo $(git rev-parse --abbrev-ref HEAD);
fi;
cd $GOPATH/src/github.com/dedis/cothority; 
go get -t ./...

