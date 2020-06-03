# Releasing a new version

To be able to do `docker push` you need to have your Docker Hub account added to the dedis organisation.
Here are the steps we use to make a conode binary release:
```
git checkout master
git diff -> be sure you have no outstanding changes
git log
```
review to be sure this is where the tag should be
```
git tag v3.4.6
git push origin v3.4.6
cd conode
make bindist # makes a file called conode-v3.4.6.tar.gz file with the binaries in it
make tooldist # makes a file called conode-tools-v3.4.6.tar.gz with the tools binaries in it
```

Note: requires a linux environment to call the make functions. You can run those in a docker container, ie with 
```
docker run -it -v /path/to/cothority:/home golang:1.14 make bindist tooldist
```

use GitHub web interface to convert tag v3.4.6 into a release, and add the conode-v3.0.4.tar.gz file onto it (click on the tag and then click on “Edit Tag”)
```
docker login --username=${DOCKERHUB_USERNAME}
make docker_push BUILD_TAG=v3.4.6
# move the :latest tag over to the new Docker container
docker tag dedis/conode:v3.4.6 dedis/conode:latest
docker push dedis/conode:latest
```
