#!/usr/bin/env bash
set -e

# run_conode.sh wraps the conode-binary to some common usecases:
# run_conode.sh public # Launch a public conode - supposes it's already configured
# run_conode.sh local 3 # Launches 3 conodes locally.

MAILADDR=linus.gasser@epfl.ch
MAILCMD=/usr/bin/mail

# Find out which package this copy of run_conode.sh is checked into.
dir=$(dirname $(realpath $0))
all_args="$*"

# increment version sub if there's something about cothority that changes
# and requires a migration, but onet does not change.
VERSION_SUB="1"
# increment version in onet if there's something that changes that needs
# migration.
ONET_PATH="$(go env GOPATH)/src/github.com/dedis/onet"
VERSION_ONET=$( grep "const Version" $ONET_PATH/onet.go | sed -e "s/.* \"\(.*\)\"/\1/g" )
VERSION="$VERSION_ONET-$VERSION_SUB"

# TAGS should be passed in from the environment if you want to add extra
# build tags to all calls to go. For example to turn on vartime algorithms:
#   TAGS="-tags vartime" ./run_conode.sh
# Note: TAGS is also used by the integration tests.

# This will allow you to put the servers on a different range if you want.
#   $ export PORTBASE=18000
#   $ ./run_conode.sh local 3
# Results in the servers being on 18002-18007.
[ -z "$PORTBASE" ] && PORTBASE=7000

main(){
	if [ ! "$1" ]; then
		showHelp
		exit 1
	fi

	if ! go env GOPATH > /dev/null; then
		echo "Could not find GOPATH."
		echo "Please install go: https://golang.org/doc/install"
		exit 1
	fi
	gopath="$(go env GOPATH)"

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

public				# runs a public conode - supposes it's already configured
	-update			# will automatically update the repositories
	-mail			# every time the cothority restarts, the last 200 lines get sent
					# to $MAILADDR
	-debug 2		# Set the debug-level for the conode-run (default: 2)
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
	DEBUG=2
	local BUILD=true
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
		-nobuild)
			BUILD=false
			;;
		*)
			DEBUG=$1
			;;
		esac
		shift
	done

	killall -9 conode || true
	if [ "$BUILD" = "true" ]; then
		pkg=`cd $dir && go list ..`
		go install $TAGS $pkg/conode
	fi

	rm -f public.toml
	for n in $( seq $NBR ); do
		co=co$n
		if [ -f $co/public.toml ]; then
			if ! grep -q Description $co/public.toml; then
				echo "Detected old files - deleting"
				rm -rf $co
			fi
			if grep 'Public =' $co/public.toml|grep -q =\"; then
				echo "Detected base64 public key for $co: converting"
				mv $co/public.toml $co/public.toml.bak
				conode convert64 < $co/public.toml.bak > $co/public.toml
			fi
		fi

		if [ ! -d $co ]; then
			echo -e "localhost:$(($PORTBASE + 2 * $n))\nConode_$n\n$co" | conode setup
		fi
		conode -d $DEBUG -c $co/private.toml server &
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
	DEBUG=2
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
			else
				echo "$MAILCMD not found - install using"
				echo "sudo apt-get install bsd-mailx"
			fi
			;;
		-memory)
			MEMLIMIT=$2
			shift
			if [ "$MEMLIMIT" -lt 500 ]; then
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
		pkg=`cd $dir && go list ..`
		go install $TAGS $pkg/conode
	fi
	migrate
	if [ ! -f $PATH_CONODE/private.toml ]; then
		echo "Didn't find private.toml in $PATH_CONODE - setting up conode"
		if conode setup; then
			echo "Successfully setup conode."
			exit 0
		else
			echo "Something went wrong during the setup"
			exit 1
		fi
	fi

	echo "Running conode with args: $ARGS and debug: $DEBUG"
	# Thanks to Pavel Shved from http://unix.stackexchange.com/questions/44985/limit-memory-usage-for-a-single-linux-process
	if [ -n "$MEMLIMIT" ]; then
		ulimit -Sv $(( MEMLIMIT * 1024 ))
	fi

	log=/tmp/conode-$$.log
	conode -d $DEBUG $ARGS server 2>&1 | tee $log
	if [ "$MAIL" ]; then
		tail -n 200 $log | $MAILCMD -s "conode-log from $(hostname):$(date)" $MAILADDR
		echo "Waiting one minute before launching conode again"
		sleep 60
	fi
	rm $log
	echo "Conode exited at $(date) - restarting"
	sleep 5
	exec $0 $all_args
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
		1.2-1)
				co="$PATH_CONODE"
			echo "Converting base64 public key in $co"
				mv $co/public.toml $co/public.toml.bak
			conode convert64 < $co/public.toml.bak > $co/public.toml
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
  pkg=`cd $dir && go list ..`
  go get -u $pkg/...
fi
exec $0 $ACTION -update_rec $TMP
EOF
	chmod a+x $TMP
	exec $TMP
}

test(){
	. "$(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh"

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
	testGrep conode pgrep -lf conode
}

testLocal(){
	runLocal 3 &
	while pgrep go; do
		sleep 1
	done
	sleep 2
	local found=$( pgrep conode | wc -l | sed -e "s/ *//g" )
	if [ "$found" != 3 ]; then
		fail "Didn't find 3 servers, but $found"
	fi
	pkill -9 conode
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
