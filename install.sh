#!/bin/bash

## install.sh will fetch all dependencies of the project.
## If the branch from where the PR comes from starts with
## `refactor_`, it will try to checkout the same branch name 
## in dedis/cosi.

pwd
cd $TRAVIS_BUILD_DIR
pwd

# Return if we are not in a Pull Request
[[ "$TRAVIS_PULL_REQUEST" = "false" ]] && go get -t ./... && return

# Method 1 from
# http://graysonkoonce.com/getting-the-current-branch-name-during-a-pull-request-in-travis-ci/
PR=https://api.github.com/repos/$TRAVIS_REPO_SLUG/pulls/$TRAVIS_PULL_REQUEST
BRANCH1=$(echo `curl -s $PR | jq -r .head.ref`)

# method 2 from https://github.com/travis-ci/travis-ci/issues/1633
git fetch --tags
git fetch --unshallow
BRANCH2=`git rev-parse --abbrev-ref HEAD`

# method 3 from
# https://gist.github.com/derekstavis/0526ac13cfecb5d6ffe5#file-travis-github-pull-request-integration-sh
GITHUB_PR_URL=https://api.github.com/repos/$TRAVIS_REPO_SLUG/pulls/$TRAVIS_PULL_REQUEST
echo PR_URL: $GITHUB_PR_URL
GITHUB_PR_BODY=$(curl -s $GITHUB_PR_URL 2>/dev/null)
echo PR_BODY: $GITHUB_PR_BODY
if [[ $GITHUB_PR_BODY =~ \"ref\":\ *\"([a-zA-Z0-9_-]*)\" ]]; then
  BRANCH3=${BASH_REMATCH[1]}
fi

echo "TRAVIS_BRANCH=$TRAVIS_BRANCH, BRANCHES=$BRANCH1--$BRANCH2--$BRANCH3"
export BRANCH=$BRANCH1
export BRANCH=refactor_cothority_506

pattern="refactor_*";
if [[ $BRANCH =~ $pattern ]]; then 
    echo "Using refactor branch ..."
    repo=github.com/dedis/cosi; 
    go get -d $repo; 
    cd $GOPATH/src/$repo; 
    git checkout -f $BRANCH; 
    echo $(git rev-parse --abbrev-ref HEAD);
fi;

cd $GOPATH/src/github.com/dedis/cothority; 
go get -t ./...
