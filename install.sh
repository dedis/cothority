#!/bin/bash

## install.sh will fetch all dependencies of the project.
## If the branch from where the PR comes from starts with
## `refactor_`, it will try to checkout the same branch name 
## in dedis/cosi.
# Method 1 from
# http://graysonkoonce.com/getting-the-current-branch-name-during-a-pull-request-in-travis-ci/
#export PR=https://api.github.com/repos/$TRAVIS_REPO_SLUG/pulls/$TRAVIS_PULL_REQUEST
#export BRANCH=$(echo `curl -s $PR | jq -r .head.ref`)
#echo "TRAVIS_BRANCH=$TRAVIS_BRANCH, PR=$PR, BRANCH=$BRANCH"
# method 2 from https://github.com/travis-ci/travis-ci/issues/1633
cd $GOPATH/src/github.com/dedis/cothority; 
git fetch --tags
git fetch --unshallow
BRANCH=`git rev-parse --abbrev-ref HEAD`
echo "Branch is $BRANCH"

pattern="refactor_*"; 
if [[ $BRANCH =~ $pattern ]]; then 
    echo "Using refactor branch ..."
    repo=github.com/dedis/cosi; 
    go get -d $repo; 
    cd $GOPATH/src/$repo; 
    git checkout -f $BRANCH; 
    git pull;
    echo $(git rev-parse --abbrev-ref HEAD);
fi;
go get -t ./...

