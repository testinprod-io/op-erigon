package app

import (
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/cmd/utils"
	"github.com/ledgerwatch/erigon/eth"
	turboNode "github.com/ledgerwatch/erigon/turbo/node"
	"github.com/ledgerwatch/log/v3"
	"github.com/urfave/cli/v2"
)

var dropLogIndexCommand = cli.Command{
	Action: MigrateFlags(dropLogIndex),
	Name:   "drop-log-index",
	Usage:  "drop log index",
	Flags: []cli.Flag{
		&utils.DataDirFlag,
		&utils.ChainFlag,
	},
	Category: "BLOCKCHAIN COMMANDS",
	Description: `
The drop command drops LogTopicIndex table and LogAddressIndex table.`,
}

func dropLogIndex(ctx *cli.Context) error {
	logger := log.New(ctx)

	nodeCfg := turboNode.NewNodConfigUrfave(ctx)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg)

	stack := makeConfigNode(nodeCfg)
	defer stack.Close()

	ethereum, err := eth.New(stack, ethCfg, logger)
	if err != nil {
		return err
	}
	err = ethereum.Init(stack, ethCfg)
	if err != nil {
		return err
	}

	if err := DropLogIndex(ethereum); err != nil {
		return err
	}

	return nil
}

func DropLogIndex(ethereum *eth.Ethereum) error {
	log.Info("Dropping Log Index")
	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.ClearBucket(kv.LogTopicIndex)
	tx.ClearBucket(kv.LogAddressIndex)

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
