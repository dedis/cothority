#!/usr/bin/env bash
set -e

# A POSIX variable
OPTIND=1         # Reset in case getopts has been used previously in the shell.

# Initialize our own variables:
verbose=0
nbr_nodes=3
base_port=7770
base_ip=localhost
data_dir=.
show_all="true"
show_time="false"
single=""

while getopts "h?v:n:p:i:d:qftsc" opt; do
    case "$opt" in
    h|\?)
        echo "Allowed arguments:

        -h help
        -v verbosity level: none (0) - full (5)
        -t show timestamps on logging
        -c show logs in color
        -n number of nodes (3)
        -p port base in case of new configuration (7000)
        -i IP in case of new configuration (localhost)
        -d data dir to store private keys, databases and logs (.)
        -q quiet all non-leader nodes
        -s don't start failing nodes again
        -f flush databases and start from scratch"
        exit 0
        ;;
    v)  verbose=$OPTARG
        ;;
    n)  nbr_nodes=$OPTARG
        ;;
    p)  base_port=$OPTARG
        ;;
    i)  base_ip=$OPTARG
        ;;
    d)  data_dir=$OPTARG
        ;;
    q)  show_all=""
        ;;
    f)  flush="yes"
        ;;
    t)  DEBUG_TIME="true"
        export DEBUG_TIME
        ;;
    s)  single="true"
        ;;
    c)  export DEBUG_COLOR=true
        ;;
    esac
done

shift $((OPTIND-1))

[ "${1:-}" = "--" ] && shift

CONODE_BIN="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"/conode
if [ ! -x $CONODE_BIN ]; then
	echo "No conode executable found. Use \"go build\" to make it."
	exit 1
fi

mkdir -p $data_dir
cd $data_dir
export DEBUG_TIME=true
if [ "$flush" ]; then
  echo "Flushing databases"
  rm -f *db
fi

rm -f public.toml
mkdir -p log
touch running
for n in $( seq $nbr_nodes -1 1 ); do
  co=co$n
  PORT=$(($base_port + 2 * n - 2))
  if [ ! -d $co ]; then
    echo -e "$base_ip:$PORT\nConode_$n\n$co" | $CONODE_BIN setup
  fi
  (
    LOG=log/conode_${co}_$PORT
    SHOW=$( [ "$n" -eq 1 -o "$show_all" ] && echo "showing" || echo "" )
    export CONODE_SERVICE_PATH=$(pwd)
    while [[ -f running ]]; do
      echo "Starting conode $LOG"
      if [[ "$SHOW" ]]; then
        $CONODE_BIN -d $verbose -c $co/private.toml server 2>&1 | tee $LOG-$(date +%y%m%d-%H%M).log
      else
        $CONODE_BIN -d $verbose -c $co/private.toml server > $LOG-$(date +%y%m%d-%H%M).log 2>&1
      fi
      if [[ "$single" ]]; then
        echo "Will not restart conode in single mode."
        exit
      fi
      sleep 1
    done
  ) &
  cat $co/public.toml >> public.toml
  # Wait for LOG to be initialized
  sleep 1
done

trap ctrl_c INT

function ctrl_c() {
  rm running
  pkill conode
}

while true; do
  sleep 1;
done
