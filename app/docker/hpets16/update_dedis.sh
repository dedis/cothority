#!/bin/bash
for src in cosi cothority/app/cothorityd cothority/app/cisc; do
  go get -v -t github.com/dedis/$src
done
