#!/usr/bin/env bash

# Remove compiled docker images
docker rmi $(docker images | grep "reprod" | awk '{print $3}')
docker rmi $(docker images | grep "none" | awk '{print $3}')

# Remove log files
rm *.log