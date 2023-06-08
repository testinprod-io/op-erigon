#!/bin/bash
set -e

banner() {
    echo "+------------------------------------------+"
    printf "| %-40s |\n" "$(date)"
    echo "|                                          |"
    printf "|$(tput bold) %-40s $(tput sgr0)|\n" "$@"
    echo "+------------------------------------------+"
    slack_report "$1" > /dev/null
}

# never fail
slack_report() {
    if ! [ -e "slack_report.sh" ]; then
        echo "slack reporter does not exist" 1>&2
        return 0
    fi
    echo "sending slack message: $1"
    ./slack_report.sh "$1" || true
    echo
}

echo " ____           _                _      __  __ _                 _   _              "
echo " |  _ \         | |              | |    |  \/  (_)               | | (_)            "
echo " | |_) | ___  __| |_ __ ___   ___| | __ | \  / |_  __ _ _ __ __ _| |_ _  ___  _ __  "
echo " |  _ < / _ \/ _  |  __/ _ \ / __| |/ / | |\/| | |/ _  | '__/ _  | __| |/ _ \| '_ \ "
echo " | |_) |  __/ (_| | | | (_) | (__|   <  | |  | | | (_| | | | (_| | |_| | (_) | | | |"
echo " |____/ \___|\__,_|_|  \___/ \___|_|\_\ |_|  |_|_|\__, |_|  \__,_|\__|_|\___/|_| |_|"
echo "                                                   __/ |                            "
echo "                                                  |___/                             "

LOG_DIR="migration-log"

mkdir -p "$LOG_DIR"

ARTIFACT_PATH="/tmp/migration-artifact"
if [[ -n "$1" ]]; then
    ARTIFACT_PATH=$1
fi
if [ ! -d "$ARTIFACT_PATH" ]; then
    echo "artifact path $ARTIFACT_PATH does not exist"
    exit 1
fi
echo "artifact path set to $ARTIFACT_PATH"

ERIGON_DATA_DIR="erigon_db"
if [[ -n "$2" ]]; then
    ERIGON_DATA_DIR=$2
fi
echo "data dir set to $ERIGON_DATA_DIR"

EXTRA_FLAGS="--no-downloader --nodiscover --maxpeers=0 --txpool.disable"
EXTRA_FLAGS="$EXTRA_FLAGS --log.console.verbosity=3"
EXTRA_FLAGS="$EXTRA_FLAGS --chain=$CHAIN"
EXTRA_FLAGS="$EXTRA_FLAGS --metrics --metrics.addr=0.0.0.0 --metrics.port=55555"
# disable port collision between prometheus
EXTRA_FLAGS="$EXTRA_FLAGS --private.api.addr=localhost:12345"

banner "Recover Intermediate Hash"
time ./build/bin/erigon $EXTRA_FLAGS --datadir=$ERIGON_DATA_DIR --log.dir.path=$LOG_DIR/recover_intermediatehash recover-intermediatehash $ARTIFACT_PATH/intermediatehash.bin 2> /dev/null

slack_report "Recover done" > /dev/null
