from common import erigon, geth
from constants import BEDROCK_START
import random
import pytest


def test_eth_chainId():
    assert erigon.eth.chain_id == geth.eth.chain_id


def test_eth_getBlockByNumber_bedrock():
    max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    for full_transactions in [False, True]:
        for _ in range(16):
            target_block_number = random.randint(BEDROCK_START, max_block_number)
            erigon_block = dict(erigon.eth.get_block(target_block_number, full_transactions=full_transactions))
            geth_block = dict(geth.eth.get_block(target_block_number, full_transactions=full_transactions))
            assert erigon_block == geth_block


def test_eth_getBlockByNumber_prebedrock():
    for full_transactions in [False, True]:
        for _ in range(16):
            target_block_number = random.randint(0, BEDROCK_START - 1) 
            erigon_block = dict(erigon.eth.get_block(target_block_number, full_transactions=full_transactions))
            geth_block = dict(geth.eth.get_block(target_block_number, full_transactions=full_transactions))
            
            erigon_block.pop("totalDifficulty")
            geth_block.pop("totalDifficulty")
            
            assert erigon_block == geth_block


def get_addresses(addresses, tx):
    addresses.add(tx["from"])
    if "to" in addresses:
        addresses.add(tx["to"])


def test_eth_getBalance_bedrock():
    max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    for _ in range(16):
        target_block_number = random.randint(BEDROCK_START, max_block_number)
        erigon_block = dict(erigon.eth.get_block(target_block_number, full_transactions=True))
        geth_block = dict(geth.eth.get_block(target_block_number, full_transactions=True))
        erigon_addresses = set()
        geth_addresses = set()
        for tx in erigon_block["transactions"]:    
            get_addresses(erigon_addresses, tx)
        for tx in geth_block["transactions"]:
            get_addresses(geth_addresses, tx)
        assert erigon_addresses == geth_addresses
        for address in erigon_addresses:
            assert erigon.eth.get_balance(address) == geth.eth.get_balance(address)
        assert erigon_block == geth_block



def test_eth_getBalance_prebedrock():
    for _ in range(16):
        target_block_number = random.randint(0, BEDROCK_START - 1) 
        erigon_block = dict(erigon.eth.get_block(target_block_number, full_transactions=True))
        geth_block = dict(geth.eth.get_block(target_block_number, full_transactions=True))
        erigon_addresses = set()
        geth_addresses = set()
        for tx in erigon_block["transactions"]:    
            get_addresses(erigon_addresses, tx)
        for tx in geth_block["transactions"]:
            get_addresses(geth_addresses, tx)
        assert erigon_addresses == geth_addresses
        for address in erigon_addresses:
            assert erigon.eth.get_balance(address) == geth.eth.get_balance(address)


def test_eth_getTransactionCount_bedrock():
    max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    for _ in range(16):
        target_block_number = random.randint(BEDROCK_START, max_block_number)        
        erigon_block = dict(erigon.eth.get_block(target_block_number, full_transactions=True))
        geth_block = dict(geth.eth.get_block(target_block_number, full_transactions=True))
        erigon_addresses = set()
        geth_addresses = set()
        for tx in erigon_block["transactions"]:    
            get_addresses(erigon_addresses, tx)
        for tx in geth_block["transactions"]:
            get_addresses(geth_addresses, tx)

        assert erigon_addresses == geth_addresses
        for address in erigon_addresses:
            erigon_tx_count = erigon.eth.get_transaction_count(address)
            geth_tx_count = geth.eth.get_transaction_count(address)
            assert erigon_tx_count == geth_tx_count, (address, target_block_number)


def test_eth_getTransactionCount_prebedrock():
    for _ in range(16):
        target_block_number = random.randint(0, BEDROCK_START - 1) 
        erigon_block = dict(erigon.eth.get_block(target_block_number, full_transactions=True))
        geth_block = dict(geth.eth.get_block(target_block_number, full_transactions=True))
        erigon_addresses = set()
        geth_addresses = set()
        for tx in erigon_block["transactions"]:    
            get_addresses(erigon_addresses, tx)
        for tx in geth_block["transactions"]:
            get_addresses(geth_addresses, tx)

        assert erigon_addresses == geth_addresses
        for address in erigon_addresses:
            erigon_tx_count = erigon.eth.get_transaction_count(address)
            geth_tx_count = geth.eth.get_transaction_count(address)
            assert erigon_tx_count == geth_tx_count, (address, target_block_number)
