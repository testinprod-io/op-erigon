package app

import (
	"bytes"
	"context"
	"fmt"
	"github.com/erigontech/erigon-lib/common/datadir"
	"github.com/erigontech/erigon-lib/config3"
	"github.com/erigontech/erigon-lib/kv/order"
	"github.com/erigontech/erigon-lib/kv/rawdbv3"
	"github.com/erigontech/erigon-lib/kv/temporal"
	state3 "github.com/erigontech/erigon-lib/state"
	"github.com/erigontech/erigon/turbo/debug"
	"strconv"
	"time"

	"github.com/erigontech/erigon-lib/common"
	libcommon "github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon-lib/crypto"
	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon-lib/kv/dbutils"
	"github.com/erigontech/erigon-lib/log/v3"
	"github.com/erigontech/erigon-lib/trie"
	"github.com/erigontech/erigon-lib/types/accounts"
	"github.com/erigontech/erigon/cmd/utils"
	"github.com/erigontech/erigon/core/rawdb"
	"github.com/erigontech/erigon/core/types"
	"github.com/erigontech/erigon/eth"
	turboNode "github.com/erigontech/erigon/turbo/node"
	"github.com/urfave/cli/v2"
)

var dbSanityCheckCommand = cli.Command{
	Action:    MigrateFlags(dbSanityCheck),
	Name:      "sanity-check",
	Usage:     "sanity check blockchain database",
	ArgsUsage: "<blockNum>",
	Flags: []cli.Flag{
		&utils.DataDirFlag,
		&utils.ChainFlag,
	},
	Category: "BLOCKCHAIN COMMANDS",
	Description: `
The sanity check command checks database sanity`,
}

func dbSanityCheck(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		utils.Fatalf("This command requires an argument.")
	}

	logger, _, _, err := debug.Setup(ctx, true /* rootLogger */)
	if err != nil {
		return err
	}

	nodeCfg, err := turboNode.NewNodConfigUrfave(ctx, logger)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg, logger)

	stack := makeConfigNode(ctx.Context, nodeCfg, logger)
	defer stack.Close()

	ethereum, err := eth.New(ctx.Context, stack, ethCfg, logger)
	if err != nil {
		return err
	}
	err = ethereum.Init(stack, ethCfg, ethCfg.Genesis.Config)
	if err != nil {
		return err
	}

	blockNum, err := strconv.ParseInt(ctx.Args().First(), 10, 64)
	if err != nil {
		utils.Fatalf("Export error in parsing parameters: block number not an integer\n")
	}

	if err := DbSanityCheck(ctx.Context, ethCfg.Dirs, ethereum, uint64(blockNum), false); err != nil {
		return err
	}

	return nil
}

func DbSanityCheck(ctx context.Context, dirs datadir.Dirs, ethereum *eth.Ethereum, blockNumber uint64, checkEmpty bool) error {
	log.Info("Database sanity check for block number", "blockNumber", blockNumber, "checkEmpty", checkEmpty)

	startAddress := libcommon.Address{}

	db := ethereum.ChainDB()

	agg, err := state3.NewAggregator2(ctx, dirs, config3.DefaultStepSize, db, log.New())
	tdb, err := temporal.New(db, agg)
	if err != nil {
		return err
	}
	ttx, err := tdb.BeginTemporalRw(ctx)
	if err != nil {
		return err
	}
	defer ttx.Rollback()

	var acc accounts.Account
	var accountList []*accounts.Account
	var addrList []libcommon.Address
	var incarnationList []uint64

	idx := new(int)
	quit := StatusReporter("Walk accounts", idx)

	txNum, err := rawdbv3.TxNums.Min(ttx, blockNumber+1)
	if err != nil {
		return err
	}
	txNumForStorage, err := rawdbv3.TxNums.Min(ttx, blockNumber+1)
	if err != nil {
		return err
	}

	it, err := ttx.RangeAsOf(kv.AccountsDomain, startAddress[:], nil, txNum, order.Asc, kv.Unlim) //unlim because need skip empty vals
	if err != nil {
		return err
	}
	defer it.Close()
	for it.HasNext() {
		k, v, err := it.Next()
		if err != nil {
			return err
		}
		if len(v) == 0 {
			continue
		}

		*idx += 1
		if len(k) > 32 {
			continue
		}
		if e := acc.DecodeForStorage(v); e != nil {
			return fmt.Errorf("decoding %x for %x: %w", v, k, e)
		}
		// codehash and root will be filled at new loop
		account := accounts.Account{
			Nonce:    acc.Nonce,
			Balance:  acc.Balance,
			Root:     emptyHash,
			CodeHash: emptyCodeHash,
		}
		accountList = append(accountList, &account)
		addrList = append(addrList, libcommon.BytesToAddress(k))
		incarnationList = append(incarnationList, acc.Incarnation)
	}

	close(quit)

	worldStateTrie := trie.New(emptyHash)

	*idx = 0
	quit = StatusReporter("Iterate accounts", idx)
	for i, addr := range addrList {
		*idx += 1
		account := accountList[i]
		genesisAccount := types.GenesisAccount{
			Balance: account.Balance.ToBig(),
			Nonce:   account.Nonce,
		}
		incarnation := incarnationList[i]
		storagePrefix := dbutils.PlainGenerateStoragePrefix(addr[:], incarnation)
		if incarnation > 0 {
			codeHash, err := ttx.GetOne(kv.PlainContractCode, storagePrefix)
			if err != nil {
				return fmt.Errorf("getting code hash for %x: %w", addr, err)
			}
			if codeHash != nil {
				account.CodeHash = libcommon.BytesToHash(codeHash)
			} else {
				account.CodeHash = emptyCodeHash
			}
		} else {
			account.CodeHash = emptyCodeHash
		}
		var code []byte
		if !bytes.Equal(account.CodeHash.Bytes(), emptyCodeHash[:]) {
			if code, err = ttx.GetOne(kv.Code, account.CodeHash.Bytes()); err != nil {
				return err
			}
			genesisAccount.Code = code
		}
		tempCodeHash := crypto.Keccak256(code)
		if !bytes.Equal(tempCodeHash, account.CodeHash.Bytes()) {
			return fmt.Errorf("codehash mismatch, expected %x, got %x", account.CodeHash.Bytes(), tempCodeHash)
		}

		storageTrie := trie.New(libcommon.Hash{})

		nextAcc, _ := kv.NextSubtree(addr[:])
		r, err := ttx.RangeAsOf(kv.StorageDomain, addr[:], nextAcc, txNumForStorage, order.Asc, kv.Unlim)
		if err != nil {
			return fmt.Errorf("walking over storage for %x: %w", addr, err)
		}
		defer r.Close()
		for r.HasNext() {
			k, vs, err := r.Next()
			if err != nil {
				return fmt.Errorf("walking over storage for %x: %w", addr, err)
			}
			//if len(vs) == 0 {
			//	continue // Skip deleted entries
			//}
			loc := k[20:]
			h, _ := common.HashData(loc)
			storageTrie.Update(h.Bytes(), libcommon.Copy(vs))
		}
		r.Close()

		storageTrieRoot := storageTrie.Hash()
		// storage trie root will be eventually checked by calculating world state trie root
		account.Root = storageTrieRoot

		value := make([]byte, account.EncodingLengthForHashing())
		account.EncodeForHashing(value)

		addrHash, _ := common.HashData(addr.Bytes())
		worldStateTrie.UpdateAccount(addrHash.Bytes(), account)
	}
	close(quit)

	startTime := time.Now()
	stateRoot := worldStateTrie.Hash()
	log.Info("World State Trie Root Calculation", "elapsed", time.Since(startTime))

	var targetRoot libcommon.Hash
	if checkEmpty {
		log.Info("State trie must by empty")
		targetRoot = types.EmptyRootHash
	} else {
		blockHash, err := rawdb.ReadCanonicalHash(ttx, blockNumber)
		if err != nil {
			return err
		}

		header := rawdb.ReadHeader(ttx, blockHash, blockNumber)
		if header == nil {
			return fmt.Errorf("header for block %d not found", blockNumber)
		}

		stateRootFromHeader := header.Root
		log.Info("state root stored at blockheader", "root", stateRootFromHeader.Hex())
		targetRoot = stateRootFromHeader
	}

	if bytes.Equal(stateRoot.Bytes(), targetRoot.Bytes()) {
		log.Info("state root consistent with target root")
	} else {
		return fmt.Errorf("state trie root mismatch, expected %x, got %x", targetRoot, stateRoot)
	}

	return nil
}
