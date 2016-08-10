#!/bin/bash

## install.sh will fetch all dependencies of the project.
## If the branch from where the PR comes from starts with
## `refactor_`, it will checkout the same branch name
## in dedis/cosi.


if [ "$TRAVIS_PULL_REQUEST" = "false" ]; then
  BRANCH=$TRAVIS_BRANCH
else
  # http://graysonkoonce.com/getting-the-current-branch-name-during-a-pull-request-in-travis-ci/
  PR=https://api.github.com/repos/$TRAVIS_REPO_SLUG/pulls/$TRAVIS_PULL_REQUEST
  BRANCH1=$(echo `curl -s $PR | jq -r .head.ref`)

  # source: https://gist.github.com/derekstavis/0526ac13cfecb5d6ffe5#file-travis-github-pull-request-integration-sh
  GITHUB_PR_URL=https://api.github.com/repos/$TRAVIS_REPO_SLUG/pulls/$TRAVIS_PULL_REQUEST
  GITHUB_PR_BODY=$(curl -s $GITHUB_PR_URL 2>/dev/null)

  if [[ $GITHUB_PR_BODY =~ \"ref\":\ *\"([a-zA-Z0-9_-]*)\" ]]; then
    BRANCH2=${BASH_REMATCH[1]}
  fi
fi
echo "Found branches $BRANCH1 -- $BRANCH2"

# If you don't believe in travis-magic:
#BRANCH=refactor_cothority_506
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

go get -t ./...
