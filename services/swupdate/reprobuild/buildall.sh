#!/bin/bash
for pkg in required essential popular random; do 
	echo Working on $pkg
	mkdir -p pkg
	cd $pkg
	../cleanup.sh
	../crawler.py $pkg > crawler.log
	cd ..
done
