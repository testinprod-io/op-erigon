## op-migrate

This docs performs database migration for erigon from geth.

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
```

#### Export Blocks

```sh
./build/bin/geth --datadir=goerli-bedrock-archive --nodiscover export blocks_0_4061224.rlp 0 4061224
```

#### Export Receipts

```sh
./build/bin/geth --datadir=goerli-bedrock-archive --nodiscover export-receipts receipts_0_4061223.rlp 0 4061223
```

#### Export State

```
./build/bin/geth --datadir=goerli-bedrock-archive --nodiscover dump --iterative 4061224 > world_trie_state_4061224.jsonl
```
or
```
./build/bin/geth --datadir=goerli-bedrock-archive --nodiscover dump 4061224 > world_trie_state_4061224.json
```

#### Export Total Difficulty

```sh
./build/bin/geth --datadir=goerli-bedrock-archive --nodiscover export-totaldifficulty totaldifficulty_0_4061224.rlp 0 4061224
```

### Import

```sh
git clone https://github.com/testinprod-io/op-erigon
cd op-erigon
git switch pcw109550/bedrock-db-migration
make erigon
```

#### Import Genesis

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli init genesis.json
```

#### Recover Genesis

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover  --chain=optimism-goerli recover-regenesis
```

#### Import Block

```
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli import blocks_0_4061224.rlp
```

#### Import Total Difficulty

```
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli import-totaldifficulty totaldifficulty_0_4061224.rlp
```


#### Import Receipts

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli import-receipts receipts_0_4061223.rlp
```

#### Import State

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli import-state world_trie_state_4061224.jsonl 4061224
```
or 
```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli --import.stream=false import-state world_trie_state_4061224.json 4061224
```

### Recovery

#### Drop Log Index

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli drop-log-index
```

#### Recover Log Index

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli recover-log-index 0 4061224
```

#### Recover Senders

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli recover-senders 0 4061224
```

### Sanity Check

```sh
./build/bin/erigon --datadir=goerli-bedrock-archive-erigon --nodiscover --chain=optimism-goerli sanity-check 4061224
```
