#!/bin/bash

# cross_compile.sh calls the go-compiler to create 32- and 64-bit binaries for
# Linux and MacOSX.

main(){
    echo Cross-compiling for platforms and cpus
    go build
    compile conode
    cd stamp
    compile stamp
    cd ..
    mv stamp/conode-bin/* conode-bin
    rmdir stamp/conode-bin
}

compile(){
    BINARY=$1
    echo Compiling $BINARY
    rm -rf conode-bin
    mkdir conode-bin
    #for GOOS in linux darwin; do
    #    for GOARCH in amd64 386; do
    for GOOS in linux; do
        for GOARCH in amd64; do
            echo Doing $GOOS / $GOARCH
            export GOOS GOARCH
            go build -o conode-bin/$BINARY-$GOOS-$GOARCH .
        done
    done
    rm conode-bin/$BINARY-darwin-386
}

main
