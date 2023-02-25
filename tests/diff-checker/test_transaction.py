import random

import pytest

from common import RPCMethod, compare_txs, erigon, erigon_client, geth, geth_client
from constants import BEDROCK_START


@pytest.mark.parametrize("bedrock", [True, False])
def test_eth_getTransactionByHash(bedrock):
    max_block_number = None
    if bedrock:
        max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    tx_hashes = []
    for _ in range(16):
        if bedrock:
            target_block_number = random.randint(BEDROCK_START, max_block_number)
        else:
            target_block_number = random.randint(0, BEDROCK_START - 1)
        geth_block = dict(
            geth.eth.get_block(target_block_number, full_transactions=False)
        )
        tx_hashes += geth_block["transactions"]

    for hash in tx_hashes:
        geth_tx = geth.eth.get_transaction(hash)
        erigon_tx = erigon.eth.get_transaction(hash)
        compare_txs(geth_tx, erigon_tx)

    # TODO: Check for each tx type


def test_eth_getTransactionByHash_invalid_hash():
    tx_hashes = ["0x" + random.randbytes(32).hex() for _ in range(16)]
    for hash in tx_hashes:
        geth_tx = geth_client.send_request(
            RPCMethod.GetTransactionByHash, params=[hash], allow_error=True
        )
        erigon_tx = erigon_client.send_request(
            RPCMethod.GetTransactionByHash, params=[hash], allow_error=True
        )
        assert geth_tx == erigon_tx


@pytest.mark.parametrize("bedrock", [True, False])
def test_eth_getTransactionByBlockHashAndIndex(bedrock):
    max_block_number = None
    if bedrock:
        max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    for _ in range(16):
        if bedrock:
            target_block_number = random.randint(BEDROCK_START, max_block_number)
        else:
            target_block_number = random.randint(0, BEDROCK_START - 1)
        geth_block = geth_client.send_request(
            RPCMethod.GetBlockByNumber, params=[hex(target_block_number), False]
        )
        tx_count = len(geth_block["transactions"])

        for i in range(tx_count):
            geth_tx = geth_client.send_request(
                RPCMethod.GetTransactionByBlockHashAndIndex,
                params=[geth_block["hash"], hex(i)],
            )
            erigon_tx = erigon_client.send_request(
                RPCMethod.GetTransactionByBlockHashAndIndex,
                params=[geth_block["hash"], hex(i)],
            )
            compare_txs(geth_tx, erigon_tx)
            assert geth_tx["hash"] == geth_block["transactions"][i]

    # TODO: Check for each tx type


@pytest.mark.parametrize("bedrock", [True, False])
def test_eth_getTransactionByBlockNumberAndIndex(bedrock):
    max_block_number = None
    if bedrock:
        max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    for _ in range(16):
        if bedrock:
            target_block_number = random.randint(BEDROCK_START, max_block_number)
        else:
            target_block_number = random.randint(0, BEDROCK_START - 1)
        geth_block = geth_client.send_request(
            RPCMethod.GetBlockByNumber, params=[hex(target_block_number), False]
        )
        tx_count = len(geth_block["transactions"])

        for i in range(tx_count):
            geth_tx = geth_client.send_request(
                RPCMethod.GetTransactionByBlockNumberAndIndex,
                params=[geth_block["number"], hex(i)],
            )
            erigon_tx = erigon_client.send_request(
                RPCMethod.GetTransactionByBlockNumberAndIndex,
                params=[geth_block["number"], hex(i)],
            )
            compare_txs(geth_tx, erigon_tx)
            assert geth_tx["hash"] == geth_block["transactions"][i]

    # TODO: Check for each tx type


@pytest.mark.parametrize("bedrock", [True, False])
def test_eth_getTransactionReceipt(bedrock):
    max_block_number = None
    if bedrock:
        max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    tx_hashes = []
    for _ in range(16):
        if bedrock:
            target_block_number = random.randint(BEDROCK_START, max_block_number)
        else:
            target_block_number = random.randint(0, BEDROCK_START - 1)
        geth_block = geth_client.send_request(
            RPCMethod.GetBlockByNumber, params=[hex(target_block_number), False]
        )
        tx_hashes += geth_block["transactions"]

    for tx_hash in tx_hashes:
        geth_receipt = geth_client.send_request(
            RPCMethod.GetTransactionReceipt, [tx_hash]
        )
        erigon_receipt = erigon_client.send_request(
            RPCMethod.GetTransactionReceipt, [tx_hash]
        )
        compare_txs(geth_receipt, erigon_receipt)

    # TODO: Check for each tx type
