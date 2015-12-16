#!/usr/bin/env bash

# create_release.sh compiles, creates the tree and puts everything into a
# binary .tgz that can be uploaded to github or other sites.

if [ ! "$1" ]; then
  echo Please give a version-number and a second argument to skip building
  exit
fi
VERSION=$1
NOCOMPILE=$2

if [ ! "$NOCOMPILE" ]; then
    ./cross_compile.sh
fi
./build_tree.sh

echo Copying scripts to the binary-directory
BIN=conode-bin
cp start-conode.sh update.sh build_tree.sh exit_conodes.sh stamp/check_stampers.sh $BIN
cp keys/config.toml $BIN
cp -a keys $BIN
cp README.binary.md $BIN
TAR=conode-$VERSION.tar.gz

echo Creating $TAR
tar cf $TAR -C $BIN .
