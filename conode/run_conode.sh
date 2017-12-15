#!/usr/bin/env bash
set -e

# run_conode.sh wraps the conode-binary to some common usecases:
# run_conode.sh public # Launch a public conode - supposes it's already configured
# run_conode.sh local 3 # Launches 3 conodes locally.

MAILADDR=linus.gasser@epfl.ch
MAILCMD=/usr/bin/mail
CONODE_BIN=conode
DEDIS_PATH=$(go env GOPATH)/src/github.com/dedis
COTHORITY_PATH=$DEDIS_PATH/cothority
ONET_PATH=$(go env GOPATH)/src/github.com/dedis/onet
CONODE_PATH=$COTHORITY_PATH/conode
CONODE_GO=github.com/dedis/cothority/conode
# increment version sub if there's something about cothority that changes
# and requires a migration, but onet does not change.
VERSION_SUB="1"
# increment version in onet if there's something that changes that needs
# migration.
VERSION_ONET=$( grep "const Version" $ONET_PATH/onet.go | sed -e "s/.* \"\(.*\)\"/\1/g" )
VERSION="$VERSION_ONET-$VERSION_SUB"
RUN_CONODE=$0
ALL_ARGS="$*"
LOG=/tmp/conode-$$.log
MEMLIMIT=""

main(){
	if [ ! "$1" ]; then
		showHelp
		exit 1
	fi

	go env GOPATH > /dev/null
	if [ $? != 0 ]; then
		echo "Could not find GOPATH."
		echo "Please install go: https://golang.org/doc/install"
		exit 1
	fi
	gopath=`go env GOPATH`
	
	if ! echo $PATH | grep -q $gopath/bin; then
		echo "Please add '$gopath/bin' to your '$PATH'"
		PATH=$PATH:$gopath/bin
		export PATH
	fi
	case $( uname ) in
	Darwin)
		PATH_CO=~/Library
		;;
	*)
		PATH_CO=~/.config
		;;
	esac

	ACTION=$1
	shift
	case $ACTION in
	public)
		runPublic $@
		;;
	local)
		runLocal $@
		;;
	test)
		test $@
		;;
	*)
		showHelp
		;;
	esac
}

showHelp(){
		cat - <<EOF
Syntax is $0: (public|local)

public			  	# runs a public conode - supposes it's already configured
	-update			# will automatically update the repositories
	-mail			# every time the cothority restarts, the last 200 lines get sent
					# to $MAILADDR
	-debug 3 		# Set the debug-level for the conode-run
	-memory 500		# Restarts the process if it exceeds 500MBytes

local nbr [dbg_lvl]	# runs nbr local conodes - you can give a debug-level as second
			      	# argument: 1-sparse..5-flood.
EOF
}

runLocal(){
	if [ ! "$1" ]; then
		echo "Please give the number of nodes to start"
		exit 1
	fi
	NBR=$1
	shift
	WAIT=""
	DEBUG=1
	while [ "$1" ]; do
		case $1 in
		-update)
			UPDATE=yes
			;;
		-debug|-d)
			DEBUG=$2
			shift
			;;
		-wait_for_apocalypse|-wait)
			WAIT=true
			;;
		*)
			DEBUG=$1
			;;
		esac
		shift
	done

	killall -9 $CONODE_BIN || true
	go install $CONODE_GO

	rm -f public.toml
	for n in $( seq $NBR ); do
		co=co$n
		if [ -f $co/public.toml ]; then
		    if ! grep -q Description $co/public.toml; then
			echo "Detected old files - deleting"
			rm -rf $co
		    fi
		    grep 'Public =' $co/public.toml|grep -q =\"
		    if [ "$?" = 0 ]; then
			echo "Detected base64 public key for $co: converting"
			mv $co/public.toml $co/public.toml.bak
			$CONODE_BIN convert64 < $co/public.toml.bak > $co/public.toml
		    fi
		fi

		if [ ! -d $co ]; then
			echo -e "127.0.0.1:$((7000 + 2 * $n))\nConode_$n\n$co" | $CONODE_BIN setup
		fi
		$CONODE_BIN server -c $co/private.toml -d $DEBUG &
		cat $co/public.toml >> public.toml
	done
	sleep 1

	cat - <<EOF

*********

Now you can use public.toml as the group-toml file to interact with your
local cothority.
EOF

	if [ "$WAIT" ]; then
		echo -e "\nWaiting for <ctrl-c>"
		while sleep 3600; do
			date
		done
	fi
}

runPublic(){
	# Get all arguments
	ARGS=""
	DEBUG=0
	while [ "$1" ]; do
		case $1 in
		-update)
			UPDATE=yes
			;;
		-update_rec)
			rm $2
			shift
			UPDATE_REC=yes
			;;
		-debug|-d)
			DEBUG=$2
			shift
			;;
		-mail)
			if [ -x $MAILCMD ]; then
				MAIL=yes
				DEBUG=3
			else
				echo "$MAILCMD not found - install using"
				echo "sudo apt-get install bsd-mailx"
			fi
			;;
		-memory)
			MEMORY=$2
			shift
			if [ "$MEMORY" -lt 500 ]; then
				echo "It will not run with less than 500 MBytes of RAM."
				exit 1
			fi
			;;
		*)
			ARGS="$ARGS $1"
			;;
		esac
		shift
	done
	if [ "$UPDATE" ]; then
		update
	else
		go install $CONODE_GO
	fi
	migrate
	if [ ! -f $PATH_CONODE/private.toml ]; then
		echo "Didn't find private.toml in $PATH_CONODE - setting up conode"
		if $CONODE_BIN setup; then
		    echo "Successfully setup conode."
		    exit 0
		else
		    echo "Something went wrong during the setup"
		    exit 1
		fi
	fi

	echo "Running conode with args: $ARGS and debug: $DEBUG"
	# Thanks to Pavel Shved from http://unix.stackexchange.com/questions/44985/limit-memory-usage-for-a-single-linux-process
	if [ "$MEMLIMIT" ]; then
		ulimit -Sv $(( MEMLIMIT * 1024 ))
	fi
	$CONODE_BIN server -d $DEBUG $ARGS | tee $LOG
	if [ "$MAIL" ]; then
		tail -n 200 $LOG | $MAILCMD -s "conode-log from $(hostname):$(date)" $MAILADDR
		echo "Waiting one minute before launching conode again"
		sleep 60
	fi
	rm $LOG
	echo "Conode exited at $(date) - restarting"
	exec $RUN_CONODE "$ALL_ARGS"
}

migrate(){
	PATH_VERSION=$PATH_CO/conode/version
	if [ -d $PATH_CO/cothorityd ]; then
		echo "Moving cothorityd-directory to conode"
		mv $PATH_CO/cothorityd $PATH_CO/conode
		echo 0.9 > $PATH_VERSION
	elif [ -d $PATH_CO/cothority ]; then
		echo "Moving cothority-directory to conode"
		mv $PATH_CO/cothority $PATH_CO/conode
		echo 0.9 > $PATH_VERSION
	fi
	PATH_CONODE=$PATH_CO/conode
	if [ ! -f $PATH_VERSION ]; then
	    mkdir -p $PATH_CONODE
	    echo $VERSION > $PATH_VERSION
	    return
	fi

	while [ "$( cat $PATH_VERSION )" != $VERSION ]; do
		case $( cat $PATH_VERSION ) in
		0.9)
			if [ -f $PATH_CONODE/config.toml ]; then
				echo "Renaming existing toml-files"
				mv $PATH_CONODE/config.toml $PATH_CONODE/private.toml
				mv $PATH_CONODE/group.toml $PATH_CONODE/public.toml
			fi
			echo 1.0 > $PATH_VERSION
			;;
		1.0)
			if ! grep -q Description $PATH_CONODE/private.toml; then
				echo "Adding description"
				grep Description $PATH_CONODE/public.toml >> $PATH_CONODE/private.toml
			fi
			echo $VERSION > $PATH_VERSION
			;;
		2.0-1)
		        co="$PATH_CONODE"
			echo "Converting base64 public key in $co"
		        mv $co/public.toml $co/public.toml.bak
			$CONODE_BIN convert64 < $co/public.toml.bak > $co/public.toml
			echo $VERSION > $PATH_VERSION
			echo "Migration to $VERSION complete"
			;;
		$VERSION)
			echo No migration necessary
			;;
        *)
            echo Found wrong version $PATH_VERSION - trying to fix
            if [ -d $PATH_CO/conode ]; then
            	echo $VERSION > $PATH_CO/conode/version
            fi
            echo "Check $PATH_CO to verify configuration is OK and re-run $0"
            exit 1
            ;;
        esac
	done
}

update(){
	# As this script might also be updated, run the update in the /tmp-directory
	echo Updating to latest version
	TMP=$( mktemp )
	TEST=$1
	cat - > $TMP << EOF
if [ ! "$TEST" ]; then
  go get -u $COTHORITY_PATH/...
fi
exec $RUN_CONODE $ACTION -update_rec $TMP
EOF
	chmod a+x $TMP
	exec $TMP
}

test(){
	. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh

	if [ "$1" != "-update_rec" ]; then
		testUpdate
	fi
	testMigrate
	testLocal
	testPublic
}

testPublic(){
	runPublic &
	sleep 5
	testGrep $CONODE_BIN pgrep -lf $CONODE_BIN
}

testLocal(){
	runLocal 3 &
	while pgrep go; do
		sleep 1
	done
	sleep 2
	local found=$( pgrep $CONODE_BIN | wc -l | sed -e "s/ *//g" )
	if [ "$found" != 3 ]; then
		fail "Didn't find 3 servers, but $found"
	fi
	pkill -9 $CONODE_BIN
}

testMigrate(){
	testOK date
	PATH_CO=$( mktemp -d )
	for subdir in cothority cothorityd; do
		P=$PATH_CO/$subdir
		mkdir $P
		echo config > $P/config.toml
		echo group > $P/group.toml
		migrate
		testFileGrep group $PATH_CO/conode/public.toml
		testFileGrep config $PATH_CO/conode/private.toml
		testFileGrep $VERSION $PATH_CO/conode/version

		rm -rf $P/*
	done
	testFileGrep Description $PATH_CO/conode/private.toml
}

testUpdate(){
	update test
}

main $@
