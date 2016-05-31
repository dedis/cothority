#!/bin/bash
cd app
for sh in cosi cothorityd ssh_ks; do
	./test_$sh.sh || exit 1
done
cd ..

cd simul
./test_simul.sh || exit 1
cd ..
