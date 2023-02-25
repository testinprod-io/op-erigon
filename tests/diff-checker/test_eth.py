import random

import pytest
import web3

from common import RPCMethod, compare_txs, erigon, erigon_client, geth, geth_client
from constants import BEDROCK_START


def test_eth_chainId():
    assert erigon.eth.chain_id == geth.eth.chain_id


def test_eth_gasPrice():
    for _ in range(16):
        erigon_gas_price = erigon.eth.gas_price
        geth_gas_price = geth.eth.gas_price
        assert erigon_gas_price == geth_gas_price, (erigon_gas_price, geth_gas_price)


def test_eth_blockNumber():
    for _ in range(16):
        erigon_block_number = erigon.eth.block_number
        geth_block_number = geth.eth.block_number
        assert abs(erigon_block_number - geth_block_number) < 5, (
            erigon_block_number,
            geth_block_number,
        )


def test_eth_getBlockByNumber_bedrock():
    max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    for full_transactions in [False, True]:
        for _ in range(16):
            target_block_number = random.randint(BEDROCK_START, max_block_number)
            erigon_block = erigon_client.send_request(
                RPCMethod.GetBlockByNumber,
                params=[hex(target_block_number), full_transactions],
            )
            geth_block = geth_client.send_request(
                RPCMethod.GetBlockByNumber,
                params=[hex(target_block_number), full_transactions],
            )
            assert erigon_block == geth_block, (erigon_block, geth_block)


def test_eth_getBlockByNumber_prebedrock():
    for full_transactions in [False, True]:
        for _ in range(16):
            target_block_number = random.randint(0, BEDROCK_START - 1)
            erigon_block = dict(
                erigon.eth.get_block(
                    target_block_number, full_transactions=full_transactions
                )
            )
            geth_block = dict(
                geth.eth.get_block(
                    target_block_number, full_transactions=full_transactions
                )
            )

            # TODO
            erigon_block.pop("totalDifficulty")
            geth_block.pop("totalDifficulty")

            assert erigon_block == geth_block, (erigon_block, geth_block)


def test_eth_getBlockByHash_bedrock():
    max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    for full_transactions in [False, True]:
        for _ in range(16):
            target_block_number = random.randint(BEDROCK_START, max_block_number)
            erigon_block = erigon_client.send_request(
                RPCMethod.GetBlockByNumber,
                params=[hex(target_block_number), full_transactions],
            )
            geth_block = geth_client.send_request(
                RPCMethod.GetBlockByNumber,
                params=[hex(target_block_number), full_transactions],
            )
            assert erigon_block == geth_block, (erigon_block, geth_block)

            erigon_block_hash = erigon_block["hash"]
            geth_block_hash = geth_block["hash"]
            assert erigon_block_hash == geth_block_hash

            erigon_block_by_hash = erigon_client.send_request(
                RPCMethod.GetBlockByHash,
                params=[erigon_block_hash, full_transactions],
            )
            geth_block_by_hash = geth_client.send_request(
                RPCMethod.GetBlockByHash,
                params=[geth_block_hash, full_transactions],
            )

            assert erigon_block_by_hash == geth_block_by_hash, (
                erigon_block_by_hash,
                geth_block_by_hash,
            )


def test_eth_getBlockByHash_prebedrock():
    for full_transactions in [False, True]:
        for _ in range(16):
            target_block_number = random.randint(0, BEDROCK_START - 1)
            erigon_block = dict(
                erigon.eth.get_block(
                    target_block_number, full_transactions=full_transactions
                )
            )
            geth_block = dict(
                geth.eth.get_block(
                    target_block_number, full_transactions=full_transactions
                )
            )

            erigon_block_hash = erigon_block["hash"]
            geth_block_hash = geth_block["hash"]
            assert erigon_block_hash == geth_block_hash

            erigon_block_by_hash = erigon_client.send_request(
                RPCMethod.GetBlockByHash,
                params=[erigon_block_hash, full_transactions],
            )
            geth_block_by_hash = geth_client.send_request(
                RPCMethod.GetBlockByHash,
                params=[geth_block_hash, full_transactions],
            )

            # TODO
            erigon_block_by_hash.pop("totalDifficulty")
            geth_block_by_hash.pop("totalDifficulty")

            assert erigon_block_by_hash == geth_block_by_hash, (
                erigon_block_by_hash,
                geth_block_by_hash,
            )


def get_addresses(addresses, tx):
    addresses.add(tx["from"])
    if "to" in addresses:
        addresses.add(tx["to"])


def test_eth_getBalance_bedrock():
    max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    for _ in range(16):
        target_block_number = random.randint(BEDROCK_START, max_block_number)
        erigon_block = dict(
            erigon.eth.get_block(target_block_number, full_transactions=True)
        )
        geth_block = dict(
            geth.eth.get_block(target_block_number, full_transactions=True)
        )
        erigon_addresses = set()
        geth_addresses = set()
        for tx in erigon_block["transactions"]:
            get_addresses(erigon_addresses, tx)
        for tx in geth_block["transactions"]:
            get_addresses(geth_addresses, tx)
        assert erigon_addresses == geth_addresses
        for address in erigon_addresses:
            assert erigon.eth.get_balance(
                address, block_identifier=target_block_number
            ) == geth.eth.get_balance(address, block_identifier=target_block_number)


def test_eth_getBalance_prebedrock():
    for _ in range(16):
        target_block_number = random.randint(0, BEDROCK_START - 1)
        erigon_block = dict(
            erigon.eth.get_block(target_block_number, full_transactions=True)
        )
        geth_block = dict(
            geth.eth.get_block(target_block_number, full_transactions=True)
        )
        erigon_addresses = set()
        geth_addresses = set()
        for tx in erigon_block["transactions"]:
            get_addresses(erigon_addresses, tx)
        for tx in geth_block["transactions"]:
            get_addresses(geth_addresses, tx)
        assert erigon_addresses == geth_addresses
        for address in erigon_addresses:
            assert erigon.eth.get_balance(
                address, block_identifier=target_block_number
            ) == geth.eth.get_balance(address, block_identifier=target_block_number)


def test_eth_getTransactionCount_bedrock():
    max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    for _ in range(16):
        target_block_number = random.randint(BEDROCK_START, max_block_number)
        erigon_block = dict(
            erigon.eth.get_block(target_block_number, full_transactions=True)
        )
        geth_block = dict(
            geth.eth.get_block(target_block_number, full_transactions=True)
        )
        erigon_addresses = set()
        geth_addresses = set()
        for tx in erigon_block["transactions"]:
            get_addresses(erigon_addresses, tx)
        for tx in geth_block["transactions"]:
            get_addresses(geth_addresses, tx)

        assert erigon_addresses == geth_addresses
        for address in erigon_addresses:
            erigon_tx_count = erigon.eth.get_transaction_count(
                address, block_identifier=target_block_number
            )
            geth_tx_count = geth.eth.get_transaction_count(
                address, block_identifier=target_block_number
            )
            assert erigon_tx_count == geth_tx_count, (address, target_block_number)


def test_eth_getTransactionCount_prebedrock():
    for _ in range(16):
        target_block_number = random.randint(0, BEDROCK_START - 1)
        erigon_block = dict(
            erigon.eth.get_block(target_block_number, full_transactions=True)
        )
        geth_block = dict(
            geth.eth.get_block(target_block_number, full_transactions=True)
        )
        erigon_addresses = set()
        geth_addresses = set()
        for tx in erigon_block["transactions"]:
            get_addresses(erigon_addresses, tx)
        for tx in geth_block["transactions"]:
            get_addresses(geth_addresses, tx)

        assert erigon_addresses == geth_addresses
        for address in erigon_addresses:
            erigon_tx_count = erigon.eth.get_transaction_count(
                address, block_identifier=target_block_number
            )
            geth_tx_count = geth.eth.get_transaction_count(
                address, block_identifier=target_block_number
            )
            assert erigon_tx_count == geth_tx_count, (address, target_block_number)


def test_eth_getCode_bedrock():
    max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    for _ in range(16):
        target_block_number = random.randint(BEDROCK_START, max_block_number)
        erigon_block = dict(
            erigon.eth.get_block(target_block_number, full_transactions=True)
        )
        geth_block = dict(
            geth.eth.get_block(target_block_number, full_transactions=True)
        )
        erigon_addresses = set()
        geth_addresses = set()
        for tx in erigon_block["transactions"]:
            get_addresses(erigon_addresses, tx)
        for tx in geth_block["transactions"]:
            get_addresses(geth_addresses, tx)

        assert erigon_addresses == geth_addresses
        for address in erigon_addresses:
            erigon_code = erigon.eth.get_code(
                address, block_identifier=target_block_number
            )
            geth_code = geth.eth.get_code(address, block_identifier=target_block_number)
            assert erigon_code == geth_code, (address, target_block_number)


def test_eth_getCode_prebedrock():
    for _ in range(16):
        target_block_number = random.randint(0, BEDROCK_START - 1)
        erigon_block = dict(
            erigon.eth.get_block(target_block_number, full_transactions=True)
        )
        geth_block = dict(
            geth.eth.get_block(target_block_number, full_transactions=True)
        )
        erigon_addresses = set()
        geth_addresses = set()
        for tx in erigon_block["transactions"]:
            get_addresses(erigon_addresses, tx)
        for tx in geth_block["transactions"]:
            get_addresses(geth_addresses, tx)

        assert erigon_addresses == geth_addresses
        for address in erigon_addresses:
            erigon_code = erigon.eth.get_code(
                address, block_identifier=target_block_number
            )
            geth_code = geth.eth.get_code(address, block_identifier=target_block_number)
            assert erigon_code == geth_code, (address, target_block_number)


def test_eth_estimateGas():
    # L2CrossDomainMessenger
    to_addr = "0x4200000000000000000000000000000000000007"
    selector = web3.Web3.keccak(text="messageNonce()")[:4].hex()
    for _ in range(16):
        erigon_estimate = erigon_client.send_request(
            RPCMethod.EstimateGas,
            params=[
                {
                    "to": to_addr,
                    "gasPrice": 10000000000,
                    "data": selector,
                    "gas": 1000000,
                }
            ],
        )
        geth_estimate = geth_client.send_request(
            RPCMethod.EstimateGas,
            params=[
                {
                    "to": to_addr,
                    "gasPrice": "0x09184e72a000",
                    "data": selector,
                    "gas": "0xde0b6b3a7640000",
                }
            ],
        )
        assert erigon_estimate == geth_estimate, (erigon_estimate, geth_estimate)
