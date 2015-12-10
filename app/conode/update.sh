#!/usr/bin/env bash
VERSION=000600

# When called without arguments, downloads the latest version, updates and
# calls ./start-conode.sh
# When called with "update_only", it will do the same, but not run conode
# afterwards

main(){
    case "$1" in
    update_only)
        # In case we have new instructions, execute these
        update
        ./update.sh update_version
        ;;
    update_version)
        update_version
        ;;
    *)
        # Perhaps we have new instructions, so exec the perhaps new ./update.sh
        update
        ./update.sh update_version
        exec ./start-conode.sh run
        ;;
    esac
}

# Fetches the latest version and untars it here
update(){
  if [ ! -e NO_UPDATE ]; then
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
  fi
}

# Looks if there is an update-path to do
update_version(){
    # Last unsupported version
    VERSION_LAST=000507
    if [ -e version ]; then
        VERSION_LAST=$( cat version )
    fi
    if [ $VERSION_LAST != $VERSION ]; then
        for v in $( seq $VERSION_LAST $VERSION ); do
            echo -n $(( v + 1 )) > version
            case $v in
            507)
                if [ -e key.pub ]; then
                    mv key.pub key_old.pub
                    mv key.priv key_old.priv
                    addr=$( sed -e "s/ .*//" key_old.pub )
                    exec ./start-conode.sh setup $addr
                else
                    echo "Didn't find any key-file, please setup new keys"
                    exit
                fi
                ;;
            esac
        done
        echo -n $VERSION > version
    fi
}
main $1