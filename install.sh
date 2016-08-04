#!/bin/bash

## install.sh will fetch all dependencies of the project.
## If the branch from where the PR comes from starts with
## `refactor_`, it will try to checkout the same branch name 
## in dedis/cosi.
# Method 1 from
# http://graysonkoonce.com/getting-the-current-branch-name-during-a-pull-request-in-travis-ci/
export PR=https://api.github.com/repos/$TRAVIS_REPO_SLUG/pulls/$TRAVIS_PULL_REQUEST

i=0
while [ (-z $BRANCH) || "$BRANCH" = "null" ]
do
    a=`expr $a + 1`
    export BRANCH=$(echo `curl -s $PR | jq -r .head.ref`)
    echo "Got BRANCH=$BRANCH (round $a)"
done

echo "TRAVIS_BRANCH=$TRAVIS_BRANCH, PR=$PR, BRANCH=$BRANCH"

# method 2 from https://github.com/travis-ci/travis-ci/issues/1633
#git fetch --tags
#git fetch --unshallow
#BRANCH=`git rev-parse --abbrev-ref HEAD`

# method 3 from
# https://gist.github.com/derekstavis/0526ac13cfecb5d6ffe5#file-travis-github-pull-request-integration-sh
# Return if we are not in a Pull Request
#[[ "$TRAVIS_PULL_REQUEST" = "false" ]] && go get -t ./... && return
#
#GITHUB_PR_URL=https://api.github.com/repos/$TRAVIS_REPO_SLUG/pulls/$TRAVIS_PULL_REQUEST
#GITHUB_PR_BODY=$(curl -s $GITHUB_PR_URL 2>/dev/null)
#
#if [[ $GITHUB_PR_BODY =~ \"ref\":\ *\"([a-zA-Z0-9_-]*)\" ]]; then
#      export TRAVIS_BRANCH=${BASH_REMATCH[1]}
#fi;

#BRANCH=$TRAVIS_BRANCH

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

##echo "Using refactor branch ..."
##repo=github.com/dedis/cosi; 
##go get -d $repo; 
##cd $GOPATH/src/$repo; 
##git checkout -f refactor_mocking;
##git pull;
##echo $(git rev-parse --abbrev-ref HEAD);
##
cd $GOPATH/src/github.com/dedis/cothority; 
go get -t ./...

