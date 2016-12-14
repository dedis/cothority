#!/bin/bash

BASE=cothority:latest
NETWORK=192.168.77

main(){
  setupDocker
  setupNetwork
  bindLocalGo
  createGroupToml

  createDocker 1 cothority
  createDocker 2 follower1
  createDocker 3 follower2
  createDocker 4 device1
  createDocker 5 device2
  createDocker 6 device3
}

createGroupToml(){
  if [ ! -d config ]; then
    echo Building cothorityd-binary
    go build ../../cothorityd
    mkdir config
    for a in 0 1 2; do
      echo "Creating key-pair for server $a"
      echo -e "200$(( a * 2 ))\n127.0.0.1\nconfig/server$a\n" | \
        ./cothorityd setup > /dev/null
    done
    grep -hv "cothority roster" config/server*/group.toml > group.toml
    perl -pi -e "s/127.0.0/$NETWORK/" group.toml
    rm cothorityd
    echo "Done"
  fi
}

bindLocalGo(){
  echo "Binding local go-directory"
  MOUNT="-v $GOPATH/src:/home/dedis/go/src"
  createDocker 10 hpets

  echo "Updating go-binaries"
  docker start hpets > /dev/null
  CMD=". .profile; update_dedis.sh; ln -s bin/group.toml .; \
  echo sudo /etc/init.d/ssh start >> .bashrc"
  docker exec hpets /bin/bash -c "$CMD" > /dev/null
  docker kill hpets > /dev/null
  docker commit hpets hpets:latest > /dev/null
  BASE=hpets:latest
  MOUNT=""
}

setupDocker(){
  if ! docker images | grep -q ^cothority; then
    echo "Building docker-image for cothority"
    docker build -t $BASE ../cothority
  fi
}

setupNetwork(){
  echo "Setting up network"
  docker network rm services > /dev/null
  docker network create --subnet $NETWORK.0/24 --gateway $NETWORK.254 services > /dev/null
}

createDocker(){
  local IP=192.168.77.$1 NAME=$2
  if docker ps | grep -q $NAME; then
    docker kill $NAME > /dev/null
  fi
  if docker ps -a | grep -q $NAME; then
    docker rm $NAME > /dev/null
  fi
  echo "Creating docker for $NAME"

  MOUNT="$MOUNT -v $(pwd):/home/dedis/bin "
  docker run -d -u dedis --net services --ip $IP -h $NAME -t -i $MOUNT --name $NAME $BASE /bin/bash > /dev/null
  docker kill $NAME > /dev/null
}

main