#!/bin/bash
repos="required essential popular random"
for r in $repos; do
  cp ../../../services/swupdate/reprobuild/$r/reprotest.csv repro_builds_$r.csv
done
