#!/bin/bash

## install.sh will fetch all dependencies of the project.
## If the branch from where the PR comes from starts with
## `refactor_`, it will checkout the same branch name
## in dedis/cosi.

# Temporarily overwrite the branch
BRANCH=protocols_conode_632

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

#pattern="refactor_*"
pattern="protocols_conode_632"
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
