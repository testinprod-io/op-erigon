#!/bin/bash
set -e

banner() {
    echo "+------------------------------------------+"
    printf "| %-40s |\n" "$(date)"
    echo "|                                          |"
    printf "|$(tput bold) %-40s $(tput sgr0)|\n" "$@"
    echo "+------------------------------------------+"
}

echo " ____           _                _      __  __ _                 _   _              "
echo " |  _ \         | |              | |    |  \/  (_)               | | (_)            "
echo " | |_) | ___  __| |_ __ ___   ___| | __ | \  / |_  __ _ _ __ __ _| |_ _  ___  _ __  "
echo " |  _ < / _ \/ _  |  __/ _ \ / __| |/ / | |\/| | |/ _  | '__/ _  | __| |/ _ \| '_ \ "
echo " | |_) |  __/ (_| | | | (_) | (__|   <  | |  | | | (_| | | | (_| | |_| | (_) | | | |"
echo " |____/ \___|\__,_|_|  \___/ \___|_|\_\ |_|  |_|_|\__, |_|  \__,_|\__|_|\___/|_| |_|"
echo "                                                   __/ |                            "
echo "                                                  |___/                             "

ERIGON_DATA_DIR="erigon_db"
CHAIN="optimism-goerli"
LOG_DIR="migration-log"

if [[ -n "$1" ]]; then
    if [[ "$1" == "optimism-goerli" || "$1" == "optimism-mainnet" ]]; then
        CHAIN=$1
    else
        echo "invalid value for chain name. Must be either 'optimism-goerli' or 'optimism-mainnet'"
        exit 1
    fi
else
    echo "chain name not set."
    exit 1
fi

if [[ -n "$2" ]]; then
    BEDROCK_START_BLOCK_NUM=$2
else
    echo "bedrock start block number not set."
    exit 1
fi

if [ ! -d "$LOG_DIR" ]; then
    mkdir "$LOG_DIR"
fi

ARTIFACT_PATH="/tmp/migration-artifact"
if [ ! -d "$ARTIFACT_PATH" ]; then
    mkdir "$ARTIFACT_PATH"
fi

EXTRA_FLAGS="--no-downloader --nodiscover --maxpeers=0 --txpool.disable"
EXTRA_FLAGS="$EXTRA_FLAGS --log.console.verbosity=3"
EXTRA_FLAGS="$EXTRA_FLAGS --chain=$CHAIN"
EXTRA_FLAGS="$EXTRA_FLAGS --metrics --metrics.addr=0.0.0.0 --metrics.port=55555"
# disable port collision between prometheus
EXTRA_FLAGS="$EXTRA_FLAGS --private.api.addr=localhost:12345"

banner "Import Genesis"
time ./build/bin/erigon $EXTRA_FLAGS --datadir=$ERIGON_DATA_DIR --log.dir.path=$LOG_DIR/init_genesis init init/"$CHAIN".json 2> /dev/null

banner "Recover Genesis"
time ./build/bin/erigon $EXTRA_FLAGS --datadir=$ERIGON_DATA_DIR --log.dir.path=$LOG_DIR/recover_genesis recover-regenesis 2> /dev/null

banner "Import Blocks"
time ./build/bin/erigon $EXTRA_FLAGS --datadir=$ERIGON_DATA_DIR --log.dir.path=$LOG_DIR/import_block import $ARTIFACT_PATH/blocks.rlp 2> /dev/null

banner "Import Total Difficulty"
time ./build/bin/erigon $EXTRA_FLAGS --datadir=$ERIGON_DATA_DIR --log.dir.path=$LOG_DIR/import_totaldifficulty import-totaldifficulty $ARTIFACT_PATH/totaldifficulty.rlp 2> /dev/null

banner "Import Receipts"
time ./build/bin/erigon $EXTRA_FLAGS --datadir=$ERIGON_DATA_DIR --log.dir.path=$LOG_DIR/import_receipts import-receipts $ARTIFACT_PATH/receipts.rlp 2> /dev/null

banner "Import State"
time ./build/bin/erigon $EXTRA_FLAGS --datadir=$ERIGON_DATA_DIR --log.dir.path=$LOG_DIR/import_state import-state $ARTIFACT_PATH/world_trie_state.jsonl "$BEDROCK_START_BLOCK_NUM" 2> /dev/null

banner "Drop Log Index"
time ./build/bin/erigon $EXTRA_FLAGS --datadir=$ERIGON_DATA_DIR --log.dir.path=$LOG_DIR/drop_log_index drop-log-index 2> /dev/null

banner "Recover Log Index"
time ./build/bin/erigon $EXTRA_FLAGS --datadir=$ERIGON_DATA_DIR --log.dir.path=$LOG_DIR/recover_log_index recover-log-index 0 "$BEDROCK_START_BLOCK_NUM" 2> /dev/null

banner "Recover Senders"
time ./build/bin/erigon $EXTRA_FLAGS --datadir=$ERIGON_DATA_DIR --log.dir.path=$LOG_DIR/recover_senders recover-senders 0 "$BEDROCK_START_BLOCK_NUM" 2> /dev/null
