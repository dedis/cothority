#!/bin/bash

## install.sh will fetch all dependencies of the project.
## If the branch from where the PR comes from starts with
<<<<<<< HEAD
## `refactor_`, it will try to checkout the same branch name 
## in dedis/cosi.
# Method 1 from
# http://graysonkoonce.com/getting-the-current-branch-name-during-a-pull-request-in-travis-ci/
export PR=https://api.github.com/repos/$TRAVIS_REPO_SLUG/pulls/$TRAVIS_PULL_REQUEST

unset BRANCH
export BRANCH=$(echo `curl -s $PR | jq -r .head.ref`)

echo "TRAVIS_BRANCH=$TRAVIS_BRANCH, PR=$PR, BRANCH=$BRANCH"

cd $GOPATH/src/github.com/dedis/cothority; 
export BRANCH=$(`git status | grep "On branch" | cut -d" " -f3`)
echo "Git status: $BRANCH"

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

echo "Using refactor branch ..."
repo=github.com/dedis/cosi; 
go get -d $repo; 
cd $GOPATH/src/$repo; 
git checkout -f refactor_mocking;
git pull;
echo $(git rev-parse --abbrev-ref HEAD);

cd $GOPATH/src/github.com/dedis/cothority; 
go get -t ./...

=======
## `refactor_`, it will checkout the same branch name
## in dedis/cosi.

# Temporarily overwrite the branch
BRANCH=master

if [ "$TRAVIS_PULL_REQUEST" = "false" ]; then
  BRANCH=$TRAVIS_BRANCH
elif [ "$BRANCH" ]; then
  echo "Manual override of branch to: $BRANCH"
else
  # http://graysonkoonce.com/getting-the-current-branch-name-during-a-pull-request-in-travis-ci/
  PR=https://api.github.com/repos/$TRAVIS_REPO_SLUG/pulls/$TRAVIS_PULL_REQUEST
  BRANCH=$(curl -s $PR | jq -r .head.ref )
  if [ "$BRANCH" = "null" ]; then
    echo "Couldn't fetch branch - probably too many requests."
    echo "Please set your own branch manually in install.sh"
    exit 1
  fi
fi

echo "Using branch $BRANCH"

pattern="refactor_*"
if [[ $BRANCH =~ $pattern ]]; then 
    echo "Using refactored branch $BRANCH - fetching cosi"
    repo=github.com/dedis/cosi
    go get -d $repo
    cd $GOPATH/src/$repo
    git checkout -f $BRANCH
    echo $(git rev-parse --abbrev-ref HEAD)
fi

cd $TRAVIS_BUILD_DIR
go get -t ./...
>>>>>>> master
