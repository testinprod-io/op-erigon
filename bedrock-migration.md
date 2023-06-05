## op-migrate

This docs performs database migration for erigon from geth.

Run below steps for migration:

* [Export](#export)
* [Import](#import)
* (Optional) [Sanity Check](#sanity-check)

While migration, monitor resources:

* [Resource Monitor](#resource-monitor)

### Input

Bedrock archive for op-geth.

### Output

Bedrock archive for op-erigon.

### Export

```sh
git clone https://github.com/testinprod-io/op-geth
cd op-geth
git switch pcw109550/bedrock-db-migration
make geth
./migrate.sh [bedrock_start_block_num] [geth_bedrock_archive_location] [optional:artifact_path] 2>&1 | tee migration.log
# ex) ./migrate.sh 4061224 /home/ubuntu/geth_db  2>&1 | tee migration.log
```

[`migrate.sh`](https://github.com/testinprod-io/op-geth/blob/pcw109550/bedrock-db-migration/migrate.sh) executes below steps.

* [Export Blocks](#export-blocks)
* [Export Receipts](#export-receipts)
* [Export State](#export-state)
* [Export Total Difficulty](#export-total-difficulty)

All artifacts will be saved under `/tmp/migration-artifact`.
Example artifact sizes for optimisim goerli:
```
-rwxr-xr-x   1 changwan.park  wheel  4976571501 May 30 16:11 blocks.rlp
-rwxr-xr-x   1 changwan.park  wheel  3921257677 May 30 16:16 receipts.rlp
-rwxr-xr-x   1 changwan.park  wheel    16211937 May 30 16:12 totaldifficulty.rlp
-rw-r--r--   1 changwan.park  wheel  1485267744 May 30 16:20 world_trie_state.jsonl
```

#### Export Blocks

Export block(block header + body(transactions)) in rlp format.

```sh
./build/bin/geth --datadir=goerli-bedrock-archive --nodiscover export blocks_0_4061224.rlp 0 4061224
```

#### Export Receipts

Export transaction receipts in rlp format.

```sh
./build/bin/geth --datadir=goerli-bedrock-archive --nodiscover export-receipts receipts_0_4061224.rlp 0 4061224
```

#### Export State

Export world state trie in json or jsonl format. For data stream, you must use jsonl format. Give `--iterative` flag for jsonl output.

```
./build/bin/geth --datadir=goerli-bedrock-archive --nodiscover dump --iterative 4061224 > world_trie_state_4061224.jsonl
```
or
```
./build/bin/geth --datadir=goerli-bedrock-archive --nodiscover dump 4061224 > world_trie_state_4061224.json
```

#### Export Total Difficulty

Export total difficulty in rlp format.

```sh
./build/bin/geth --datadir=goerli-bedrock-archive --nodiscover export-totaldifficulty totaldifficulty_0_4061224.rlp 0 4061224
```

### Import

```sh
git clone https://github.com/testinprod-io/op-erigon
cd op-erigon
git switch pcw109550/bedrock-db-migration
make erigon
./migrate.sh [chain_name] [bedrock_start_block_num] [optional:artifact_path] [optional:erigon_db_path] 2>&1 | tee migration.log
# ex) ./migrate.sh optimism-goerli 4061224 2>&1 | tee migration.log
# chain name must be optimism-mainnet or optimism-goerli
```

[`migrate.sh`](./migrate.sh) executes below steps.

* [Import Genesis](#import-genesis)
* [Recover Genesis](#recover-genesis)
* [Import Block](#import-block)
* [Import Total Difficulty](#import-total-difficulty)
* [Import Receipts](#import-receipts)
* [Import State](#import-state)
* [Drop Log Index](#drop-log-index)
* [Recover Log Index](#recover-log-index)
* [Recover Senders](#recover-senders)

All artifacts will be fetched from `/tmp/migration-artifact`.
After database creation, we can sanity check state trie by below step. This sanity check step is not included in [`migrate.sh`](./migrate.sh).

* [Sanity Check](#sanity-check)

#### Import Genesis

Import genesis json and create db containing chainconfig. Due to bedrock regenesis, allocation from genesis will be set to null and cause wrong header hash and state trie root. This will be fixed using [recover genesis](#recover-genesis) command

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli init genesis.json
```

#### Recover Genesis

Correct genesis block's hash and state trie root.

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli recover-regenesis
```

#### Import Block

Import block(block header + body(transactions)) in rlp format.

```
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli import blocks_0_4061224.rlp
```

#### Import Total Difficulty

Import total difficulty in rlp format.

```
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli import-totaldifficulty totaldifficulty_0_4061224.rlp
```


#### Import Receipts

Import transaction receipts in rlp format.

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli import-receipts receipts_0_4061223.rlp
```

#### Import State

Import world state trie in json or jsonl format. For data stream, you must use jsonl format. Default is to use stream. Use `--import.stream` flag to disable streaming.

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli import-state world_trie_state_4061224.jsonl 4061224
```
or 
```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli --import.stream=false import-state world_trie_state_4061224.json 4061224
```

### Recovery

#### Drop Log Index

Drop log index table for recovery. In theory table will be already empty, but just to be safe, explicity drop the table. Empty table is required for [recovering log index](#recover-log-index).

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli drop-log-index
```

#### Recover Log Index

Recover log index table. The logic assumes that log index table is empty.

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli recover-log-index 0 4061224
```

#### Recover Senders

Recover senders table.

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli recover-senders 0 4061224
```

### Sanity Check

Sanity check state trie root of specific block.

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli sanity-check 4061224
```

### Resource Monitor

```sh
docker-compose up prometheus grafana
```

For monitoring resource comsumption.
