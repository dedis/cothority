#!/bin/bash
docker kill $( docker ps -q )
docker rm $( docker ps -a | grep cothority | sed -e "s/.* //" )
docker rmi cothority