#!/bin/bash

main(){
  echo Starting conode from the correct cpu and arch
  if [ ! -x conode ]; then
    search_arch
  fi
  case "$1" in
  setup)
    if [ -f key.pub ]; then
      echo "Key.pub already exists - if you want to re-create, please delete it first"
    else
      ./conode keygen $2
    fi
    cat key.pub
    ./conode validate
    ;;
  run)
    if [ ! -f config.toml ]; then
      echo "Didn't find 'config.toml' - searching in update"
      update
      if [ ! -f config.toml ]; then
        echo "Still didn't find config.toml - please copy it first here"
        echo
        exit 1
      fi
    fi
    echo Running conode
    ./conode run
    echo Updating
    update
    echo Sleeping a bit
    sleep 10
    exec ./start-conode run
    ;;
  *)
    echo Usage:
    echo $0 setup address
    echo or
    echo $0 run
    echo
    ;;
  esac
}

update(){
  RELEASE=$( wget -q -O- https://github.com/dedis/cothority/releases/latest | grep DeDiS/cothority/releases/download | sed -e "s/.*href=.\(.*\). rel.*/\1/" )
  TGZ=$( basename $RELEASE )
  if [ -e $TGZ ]; then
    echo $RELEASE already here
  else
    echo Getting $RELEASE
    wget -q https://github.com/$RELEASE
    echo Untarring
    tar xf $TGZ
  fi
}

run_loop(){
  pkill -f conode
  if [ $( which screen ) ]; then
    screen -S conode -dm ./conode $@ &
  else
    nohup ./conode $@ &
    rm nohup.out
  fi
}

search_arch(){
  echo searching for correct conode
  for GOOS in linux darwin windows netbsd; do
    for GOARCH in amd64 386 arm; do
      CONODE=conode-$GOOS-$GOARCH
      if ./$CONODE 2&>/dev/null; then
        cat - > conode <<EOF
#!/bin/bash
./$CONODE \$@
EOF
	    sed -e "s/conode/stamp/" conode > stamp
        chmod a+x conode stamp
        echo Found $CONODE to run here
        return
      fi
    done
  done
}

main $@
