#!/bin/bash
cd app
for sh in test_*.sh; do
    echo Launching $sh
	./$sh || exit 1
done
cd ..

cd simul
echo Launching simulations
./test_simul.sh || exit 1
cd ..
