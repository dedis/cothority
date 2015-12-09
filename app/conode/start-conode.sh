#!/bin/bash

main(){
  echo Starting conode from the correct cpu and arch
  if [ ! -x conode ]; then
    search_arch
  fi
  case "$1" in
  setup)
    if [ -f key.pub ]; then
      echo -e "\n*** Key.pub already exists - if you want to re-create, please delete it first\n"
      exit
    else
      if ./conode keygen $2; then
        echo Sending public-key to linus.gasser@epfl.ch
        cat key.pub | mail linus.gasser@epfl.ch
      else
        echo Couldnt create key-pair
        exit
      fi
    fi
    echo If you want to be added, make sure the following is known by the dedis-group
    cat key.pub
    ./conode validate
    if [ "$?" = "1" ]; then
      echo Received exit-command - will update and run
      exec ./update.sh
    fi
    ;;
  run)
    if [ ! -f config.toml ]; then
      echo "Didn't find 'config.toml' - searching in update"
      ./update.sh update_only
      if [ ! -f config.toml ]; then
        echo "Still didn't find config.toml - please copy it first here"
        echo
        exit 1
      fi
    fi
    echo Running conode
    ./conode run
    echo Sleeping a bit, then updating
    sleep 10
    exec ./update.sh
    ;;
  update|"")
    exec ./update.sh
    ;;
  *)
    echo Usage:
    echo $0 setup address
    echo "or to update and run it:"
    echo $0
    echo "or only run it (no update):"
    echo $0 run
    echo "or if you want to manually update:"
    echo $0 update
    echo
    ;;
  esac
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
