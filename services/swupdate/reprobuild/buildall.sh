#!/bin/bash
repos="required essential popular random"
./cleanup.sh
for pkg in $repos; do
	echo Launching $pkg
	rm -rf $pkg
	mkdir -p $pkg
	(
	cd $pkg
	../crawler.py $pkg &
	)
done
