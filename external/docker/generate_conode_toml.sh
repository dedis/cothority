#!/bin/bash

dir=$GOPATH/src/go.dedis.ch/cothority
cwd=$PWD

cd $dir/conode
go install
cd $cwd

echo "" > public.toml

for (( n=1; n<=7; n++ )) do
    printf "127.0.0.1:77%02d\nConode_$n\nco$n\nY\nY\n" $((70 + 2*$n - 2)) | conode setup
    cat "co$n/public.toml" >> public.toml
    echo "" >> public.toml
done

cp public.toml $dir/external/java/src/test/resources/.
cp public.toml $dir/external/js/cothority/spec/support/.
