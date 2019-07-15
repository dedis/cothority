#!/bin/bash

pkill conode
rm -f ~/.local/share/conode/*
./run_conode.sh local 3 2
rm -f *cfg
sleep 2

export BC_WAIT=true

rm -f ./*.png
cd /home/esteban/.local/share/bcadmin/
rm -f *.cfg
bcadmin create --roster ~/cothority/conode/public.toml
export BC=`ls bc-*.cfg`
echo "BC = $BC"
bcadmin qr --admin > ~/cothority/conode/qr-code-admin
