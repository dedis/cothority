#!/bin/bash

## install.sh will fetch all dependencies of the project.
## If the branch from where the PR comes from starts with
## `refactor_`, it will checkout the same branch name
## in dedis/cosi.


if [ "$TRAVIS_PULL_REQUEST" = "false" ]; then
  BRANCH=$TRAVIS_BRANCH
else
  BRANCH=refactor_cothority_506
fi

# If you don't believe in travis-magic:
echo "Thinking we're on branch $BRANCH"

pattern="refactor_*";
if [[ $BRANCH =~ $pattern ]]; then 
    echo "Using refactored branch $BRANCH - fetching cosi"
    repo=github.com/dedis/cosi; 
    go get -d $repo; 
    cd $GOPATH/src/$repo; 
    git checkout -f $BRANCH; 
    echo $(git rev-parse --abbrev-ref HEAD);
fi;

cd $GOPATH/src/github.com/dedis/cothority; 
go get -t ./...
