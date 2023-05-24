package app

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/cmd/utils"
	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/common/dbutils"
	"github.com/ledgerwatch/erigon/core/rawdb"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/core/types/accounts"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/eth"
	turboNode "github.com/ledgerwatch/erigon/turbo/node"
	"github.com/ledgerwatch/erigon/turbo/trie"
	"github.com/ledgerwatch/log/v3"
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

	nodeCfg := turboNode.NewNodConfigUrfave(ctx)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg)

	stack := makeConfigNode(nodeCfg)
	defer stack.Close()

	ethereum, err := eth.New(stack, ethCfg)
	if err != nil {
		return err
	}
	blockNum, err := strconv.ParseInt(ctx.Args().First(), 10, 64)
	if err != nil {
		utils.Fatalf("Export error in parsing parameters: block number not an integer\n")
	}

	if err := DbSanityCheck(ethereum, uint64(blockNum), false); err != nil {
		return err
	}

	return nil
}

func DbSanityCheck(ethereum *eth.Ethereum, blockNumber uint64, checkEmpty bool) error {
	log.Info("Database sanity check for block number", "blockNumber", blockNumber, "checkEmpty", checkEmpty)

	startAddress := libcommon.Address{}

	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var acc accounts.Account
	var accountList []*accounts.Account
	var addrList []libcommon.Address
	var incarnationList []uint64

	idx := new(int)
	quit := StatusReporter("Walk accounts", idx)
	if err := state.WalkAsOfAccounts(tx,
		startAddress,
		blockNumber+1, /* do not know why adding one up, but it just works */
		func(k, v []byte) (bool, error) {
			*idx += 1
			if len(k) > 32 {
				return true, nil
			}
			if e := acc.DecodeForStorage(v); e != nil {
				return false, fmt.Errorf("decoding %x for %x: %w", v, k, e)
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
			return true, nil
		}); err != nil {
		return err
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
			codeHash, err := tx.GetOne(kv.PlainContractCode, storagePrefix)
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
			if code, err = tx.GetOne(kv.Code, account.CodeHash.Bytes()); err != nil {
				return err
			}
			genesisAccount.Code = code
		}
		tempCodeHash := crypto.Keccak256(code)
		if !bytes.Equal(tempCodeHash, account.CodeHash.Bytes()) {
			return fmt.Errorf("codehash mismatch, expected %x, got %x", account.CodeHash.Bytes(), tempCodeHash)
		}

		storageTrie := trie.New(libcommon.Hash{})
		if err := state.WalkAsOfStorage(tx,
			addr,
			incarnation,
			libcommon.Hash{}, /* startLocation */
			blockNumber+1,    /* do not know why adding one up, but it just works */
			func(_, loc, vs []byte) (bool, error) {
				h, _ := common.HashData(loc)
				storageTrie.Update(h.Bytes(), libcommon.Copy(vs))
				return true, nil
			}); err != nil {
			return fmt.Errorf("walking over storage for %x: %w", addr, err)
		}
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
	log.Info("World State Trie Root Calculation", "elapsed", time.Duration(time.Since(startTime)))

	var targetRoot libcommon.Hash
	if checkEmpty {
		log.Info("State trie must by empty")
		targetRoot = types.EmptyRootHash
	} else {
		blockHash, err := rawdb.ReadCanonicalHash(tx, blockNumber)
		if err != nil {
			return err
		}

		header := rawdb.ReadHeader(tx, blockHash, blockNumber)
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
