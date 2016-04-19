#!/usr/bin/env bash
# Compiles binaries for MacOS and Linux 64-bit versions
# also copies the .sh-scripts from the Apps-directories
# Syntax:
# ./cross-compile.sh version [nocompile]
# if nocompile is given, only the tar is done.

if [ ! "$1" ]; then
  echo Please give a version-number
  exit
fi
VERSION=$1
APPS="cosi cosid"
BUILD=conode-bin

compile(){
    rm -rf $BUILD
    mkdir $BUILD
    for APP in $@; do
        echo "Compiling $APP"
        cd $APP
        for GOOS in linux darwin; do
          for GOARCH in amd64; do
            echo "Doing $GOOS / $GOARCH"
            export GOOS GOARCH
            go build -o ../$BUILD/$APP-$GOOS-$GOARCH .
          done
        done
        cd ..
    done
}

if [ ! "$2" ]; then
  go build
  echo "Cross-compiling for platforms and cpus"
  compile $APPS
fi

for a in $APPS; do
    cp -v chose_version.sh $BUILD/$a
done
cp dedis-servers.toml $BUILD
cp ../README.md $BUILD
TAR=conode-$VERSION.tar.gz

echo "Creating $TAR"
tar cf $TAR -C $BUILD .

git tag $VERSION
git push origin $VERSION
