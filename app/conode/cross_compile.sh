#!/bin/bash

if [ ! "$1" ]; then
  echo Please give a version-number
  exit
fi
VERSION=$1

echo Cross-compiling for platforms and cpus

compile(){
BINARY=$1
echo Compiling $BINARY
rm -rf conode-bin
mkdir conode-bin
for GOOS in linux darwin; do
  for GOARCH in amd64 386; do
    echo Doing $GOOS / $GOARCH
    export GOOS GOARCH
    go build -o conode-bin/$BINARY-$GOOS-$GOARCH .
  done
done
rm conode-bin/$BINARY-darwin-386
}

compile conode
cd stamp
compile stamp
cd ..
mv stamp/conode-bin/* conode-bin
rmdir stamp/conode-bin

echo Copying scripts to the binary-directory
cp start-conode.sh conode-bin
cp real/config.toml conode-bin
TAR=conode-$VERSION.tar.gz

echo Creating $TAR
tar cf $TAR -C conode-bin .
