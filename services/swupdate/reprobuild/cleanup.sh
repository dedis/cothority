#!/usr/bin/env bash

pkill -f crawler.py
/etc/init.d/docker restart
sleep 1
# Remove compiled docker images
docker kill $(docker ps -aq)
docker rm -f $(docker ps -aq)
docker rmi -f $(docker images | grep "reprod" | awk '{print $3}')
docker rmi -f $(docker images | grep "none" | awk '{print $3}')

# Remove log files
rm *.log
/etc/init.d/docker restart
