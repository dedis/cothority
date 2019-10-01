#!/usr/bin/env bash

# highest number of servers and clients
NBR=${NBR:-3}
# Per default, have $NBR servers
NBR_SERVERS=${NBR_SERVERS:-$NBR}
# Per default, keep one inactive server
NBR_SERVERS_GROUP=${NBR_SERVERS_GROUP:-$(( NBR_SERVERS - 1))}
# Show the output of the commands: 0=none, 1=test-names, 2=all
DBG_TEST=${DBG_TEST:-0}
# DBG-level for server
DBG_SRV=${DBG_SRV:-0}
# APPDIR is usually where the test.sh-script is located
APPDIR=${APPDIR:-$(pwd)}
# The app is the name of the builddir
APP=${APP:-$(basename $APPDIR)}
# Name of conode-log
COLOG=conode
# Base port when generating the configurations
BASE_PORT=2000

RUNOUT=$( mktemp )

for i in . .. ../.. ../../.. ../../../.. $( go env GOPATH )/src/go.dedis.ch/cothority
do
	if [ -f $i/conode/conode.go ]; then
		root=$( cd -P $i; pwd )
		break
	fi
done
if [ -z "$root" ]; then
	echo "Cannot find conode/connode.go"
	exit 1
fi

# cleans the test-directory and builds the CLI binary
# Globals:
#   CLEANBUILD
#   APPDIR
#   APP
startTest(){
  set +m
  if [ "$CLEANBUILD" ]; then
    rm -f conode $APP
  fi
  build $APPDIR
}

# Prints `Name`, cleans up build-directory, deletes all databases from services
# in previous run, and calls `testName`.
# Arguments:
#   `Name` - name of the testing function to run
run(){
  cleanup
  echo -e "\n* Testing $1"
  sleep .5
  $1
}

# Asserts that the exit-code of running `$@` using `dbgRun` is `0`.
# Arguments:
#   $@ - used to run the command
testOK(){
  testOut "Assert OK for '$@'"
  if ! dbgRun "$@"; then
    fail "starting $@ failed"
  fi
}

# Asserts that the exit-code of running `$@` using `dbgRun` is NOT `0`.
# Arguments:
#   $@ - used to run the command
testFail(){
  testOut "Assert FAIL for '$@'"
  if dbgRun "$@"; then
    fail "starting $@ should've failed, but succeeded"
  fi
}

# Asserts `File` exists and is a file.
# Arguments:
#   `File` - path to the file to test
testFile(){
  testOut "Assert file $1 exists"
  if [ ! -f $1 ]; then
    fail "file $1 is not here"
  fi
}

# Asserts `File` DOES NOT exist.
# Arguments:
#   `File` - path to the file to test
testNFile(){
  testOut "Assert file $1 DOESN'T exist"
  if [ -f $1 ]; then
    fail "file $1 IS here"
  fi
}

# Asserts that `String` exists in `File`.
# Arguments:
#   `String` - what to search for
#   `File` - in which file to search
testFileGrep(){
  local G="$1" F="$2"
  testFile "$F"
  testOut "Assert file $F contains --$G--"
  if ! pcregrep -M -q "$G" $F; then
    fail "Didn't find '$G' in file '$F': $(cat $F)"
  fi
}

# Asserts that `String` matches `Target`.
# Arguments:
#   `String` - element to compare
#   `Target` - what should match
matchOK(){
  testOut "Match OK between '$1' and '$2'"
  if ! [[ $1 =~ $2 ]]; then
    fail "'$1' does not match '$2'"
  fi
}

# Asserts that `String` does NOT match `Target`.
# Arguments:
#   `String` - element to compare
#   `Target` - what should not match
matchNOK(){
  testOut "Match NOT OK between '$1' and '$2'"
  if [[ $1 =~ $2 ]]; then
    fail "'$1' does match '$2'"
  fi
}

# Asserts that `String` is in the output of the command being run by `dbgRun`
# and all but the first input argument. Ignores the exit-code of the command.
# Arguments:
#   `String` - what to search for
#   `$@[1..]` - command to run
testGrep(){
  S="$1"
  shift
  testOut "Assert grepping '$S' in '$@'"
  runOutFile "$@"
  doGrep "$S"
  if [ ! "$EGREP" ]; then
    fail "Didn't find '$S' in output of '$@': $GREP"
  fi
}

# Asserts that `String` is in the output of the command being run by `dbgRun`
# and all but the first input argument. Ignores the exit-code of the command.
# Uses fgrep, which interprets pattern as a set of fixed string.
# Arguments:
#   `String` - what to search for
#   `$@[1..]` - command to run
testFGrep(){
  S="$1"
  shift
  testOut "Assert fgrepping '$S' in '$@'"
  runOutFile "$@"
  doFGrep "$S"
  if [ ! "$EGREP" ]; then
    fail "Didn't find '$S' in output of '$@': $GREP"
  fi
}

# Asserts the output of the command being run by `dbgRun` and all but the first
# input argument is N lines long. Ignores the exit-code of the command.
# Arguments:
#   `N` - how many lines should be output
#   `$@[1..]` - command to run
testCountLines(){
  N="$1"
  shift
  testOut "Assert wc -l is $N lines in '$@'"
  runOutFile "$@"
  lines=`wc -l < $RUNOUT`
  if [ $lines != $N ]; then
    fail "Found $lines lines in output of '$@'"
  fi
}

# Asserts that `String` is NOT in the output of the command being run by `dbgRun`
# and all but the first input argument. Ignores the exit-code of the command.
# Arguments:
#   `String` - what to search for
#   `$@[1..]` - command to run
testNGrep(){
  G="$1"
  shift
  testOut "Assert NOT grepping '$G' in '$@'"
  runOutFile "$@"
  doGrep "$G"
  if [ "$EGREP" ]; then
    fail "DID find '$G' in output of '$@': $(cat $RUNOUT)"
  fi
}

# Asserts `String` is part of the last command being run by `testGrep` or
# `testNGrep`.
# Arguments:
#   `String` - what to search for
testReGrep(){
  G="$1"
  testOut "Assert grepping again '$G' in same output as before"
  doGrep "$G"
  if [ ! "$EGREP" ]; then
    fail "Didn't find '$G' in last output: $(cat $RUNOUT)"
  fi
}

# Asserts `String` is NOT part of the last command being run by `testGrep` or
# `testNGrep`.
# Arguments:
#   `String` - what to search for
testReNGrep(){
  G="$1"
  testOut "Assert grepping again NOT '$G' in same output as before"
  doGrep "$G"
  if [ "$EGREP" ]; then
    fail "DID find '$G' in last output: $(cat $RUNOUT)"
  fi
}

# used in test*Grep methods.
doGrep(){
  # echo "grepping in $RUNOUT"
  # cat $RUNOUT
  WC=$( cat $RUNOUT | egrep "$1" | wc -l )
  EGREP=$( cat $RUNOUT | egrep "$1" )
}

# used in test*FGrep methods.
doFGrep(){
  # echo "grepping in $RUNOUT"
  # cat $RUNOUT
  WC=$( cat $RUNOUT | fgrep "$1" | wc -l )
  EGREP=$( cat $RUNOUT | fgrep "$1" )
}

# Asserts that `String` exists exactly `Count` times in the output of the
# command being run by `dbgRun` and all but the first two arguments.
# Arguments:
#   `Count` - number of occurences
#   `String` - what to search for
#   `$@[2..]` - command to run
testCount(){
  C="$1"
  G="$2"
  shift 2
  testOut "Assert counting '$C' of '$G' in '$@'"
  runOutFile "$@"
  doGrep "$G"
  if [ $WC -ne $C ]; then
    fail "Didn't find '$C' (but '$WC') of '$G' in output of '$@': $(cat $RUNOUT)"
  fi
}


# Outputs all arguments if `DBT_TEST -ge 1`
# Globals:
#   DBG_TEST - determines debug-level
testOut(){
  if [ "$DBG_TEST" -ge 1 ]; then
    echo -e "$@"
  fi
}

# Outputs all arguments if `DBT_TEST -ge 2`
# Globals:
#   DBG_TEST - determines debug-level
dbgOut(){
  if [ "$DBG_TEST" -ge 2 ]; then
    echo -e "$@"
  fi
}

# Runs `$@` and outputs the result of `$@` if `DBG_TEST -ge 2`. Redirects the
# output in all cases if `OUTFILE` is set.
# Globals:
#   DBG_TEST - determines debug-level
#   OUTFILE - if set, used to write output
dbgRun(){
  if [ "$DBG_TEST" -ge 2 ]; then
    OUT=/dev/stdout
  else
    OUT=/dev/null
  fi
  if [ "$OUTFILE" ]; then
    "$@" 2>&1 | tee $OUTFILE > $OUT
  else
    "$@" 2>&1 > $OUT
  fi
}

runGrepSed(){
  GREP="$1"
  SED="$2"
  shift 2
  runOutFile "$@"
  doGrep "$GREP"
  SED=$( echo $EGREP | sed -e "$SED" )
}

runOutFile(){
  OLDOUTFILE=$OUTFILE
  OUTFILE=$RUNOUT
  dbgRun "$@"
  OUTFILE=$OLDOUTFILE
}

fail(){
  echo
  echo -e "\tFAILED: $@"
  cleanup
  exit 1
}

backg(){
  ( "$@" 2>&1 & )
}

# Builds the app stored in the directory given in the first argument.
# Globals:
#   CLEANBUILD - if set, forces build of app, even if it exists.
#   TAGS - what tags to use when calling go build
# Arguments:
#   builddir - where to search for the app to build
build(){
  local builddir=$1
  local app=$( basename $builddir )
  local out=$( pwd )/$app
  if [ ! -e $app -o "$CLEANBUILD" ]; then
    testOut "Building $app"
    ( 
      cd $builddir
      if ! go build -o $out $TAGS *.go; then
        fail "Couldn't build $builddir"
      fi
    )
  else
    dbgOut "Not building $app because it's here"
  fi
}

buildDir(){
  BUILDDIR=./build
  mkdir -p $BUILDDIR
  testOut "Working in $BUILDDIR"
  cd $BUILDDIR
}

# If a directory is given as an argument, the service will be taken from that
# directory.
# Globals:
#   APPDIR - where the app is stored
# Arguments:
#   [serviceDir, ...] - if given, used as directory to be included. At least one
#                       argument must be given.
buildConode(){
  local incl="$@"
  if [ -z "$incl" ]; then
      echo "buildConode: No import paths provided."
      exit 1
  fi

  echo "Building conode"
  mkdir -p conode_
  ( echo -e "package main\nimport ("
    for i in $incl; do
      echo -e "\t_ \"$i\""
    done
  echo ")" ) > conode_/import.go

  cp "$root/conode/conode.go" conode_/conode.go
  go build -o conode ./conode_
  setupConode
}

setupConode(){
  # Don't show any setup messages
  DBG_OLD=$DBG_TEST
  DBG_TEST=0
  rm -f public.toml
  for n in $( seq $NBR_SERVERS ); do
    co=co$n
    rm -f $co/*
    mkdir -p $co
    local port="$(( $BASE_PORT + 2 * $n ))"
    echo -e "localhost:$port\nCot-$n\n$co\n" | dbgRun runCo $n setup
    if [ ! -f $co/public.toml ]; then
      echo "Setup failed: file $co/public.toml is missing."
      exit
    fi
    if [ $n -le $NBR_SERVERS_GROUP ]; then
      cat $co/public.toml >> public.toml
    fi
  done
  DBG_TEST=$DBG_OLD
}

# runCoBG: Run a conode in the background. It runs a conode under a subshell so
# that when it exits, it can make the .dead file once the conode dies. It then
# checks that they started listening on the expected port, and finally reports
# if one did not start as expected.
runCoBG(){
  for nb in "$@"; do
    dbgOut "starting conode-server #$nb"
    (
      # Always redirect output of server in log-file, but
      # only output to stdout if DBG_TEST > 1.
      rm -f "$COLOG$nb.log.dead"
      if [ $DBG_TEST -ge 2 ]; then
        ./conode -d $DBG_SRV -c co$nb/private.toml server 2>&1 | tee "$COLOG$nb.log"
      else
        ./conode -d $DBG_SRV -c co$nb/private.toml server >& "$COLOG$nb.log"
      fi
      touch "$COLOG$nb.log.dead"
    ) 2>/dev/null &
    # This makes `pkill conode` not outputting errors here
    disown
  done
  
  local allStarted=0
  # wait for conodes to start but maximum 10 seconds
  for (( k=0; k < 20 && allStarted == 0; k++ )) do
    sleep .5
    allStarted=1

    for nb in "$@"; do
      local port="$(( $BASE_PORT + $nb * 2 + 1 ))"
      
      if ! (echo >"/dev/tcp/localhost/$port") &>/dev/null; then
        allStarted=0
      fi
    done
  done

  if [ "$allStarted" -ne "1" ]; then
    echo "Servers failed to start"
    cat *.log
    cat *.log.dead
    exit 1
  fi

  dbgOut "All conodes have started"
}

runCo(){
  local nb=$1
  shift
  dbgOut "starting conode-server #$nb"
  dbgRun ./conode -d $DBG_SRV -c co$nb/private.toml "$@"
}

cleanup(){
  pkill -9 conode 2> /dev/null
  pkill -9 ^${APP}$ 2> /dev/null
  sleep .5
  rm -f co*/*bin
  rm -f cl*/*bin
  if [ -z "$KEEP_DB" ]; then
    rm -rf $CONODE_SERVICE_PATH
  fi
}

stopTest(){
  cleanup
  echo "Success"
}

if ! which pcregrep > /dev/null; then
  echo "*** WARNING ***"
  echo "Most probably you're missing pcregrep which might be used here..."
  echo "On mac you can install it with"
  echo -e "\n  brew install pcre\n"
  echo "Not aborting because it might work anyway."
  echo
fi

if ! which realpath > /dev/null; then
  echo "*** WARNING ***"
  echo "Most probably you're missing realpath which might be used here..."
  echo "On mac you can install it with"
  echo -e "\n  brew install coreutils\n"
  echo "Not aborting because it might work anyway."
  echo
  realpath() {
    [[ $1 = /* ]] && echo "$1" || echo "$PWD/${1#./}"
  }
fi

for i in "$@"; do
  case $i in
    -b|--build)
      CLEANBUILD=yes
      shift # past argument=value
      ;;
  esac
done
buildDir

export CONODE_SERVICE_PATH=service_storage
