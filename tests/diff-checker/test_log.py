import random

import pytest
import web3

from common import RPCMethod, compare_txs, erigon, erigon_client, geth, geth_client
from constants import BEDROCK_START

transfer_topic = web3.Web3.keccak(text="Transfer(address,address,uint256)").hex()
approval_topic = web3.Web3.keccak(text="Approval(address,address,uint256)").hex()
deposit_topic = web3.Web3.keccak(text="Deposit(address,uint256)").hex()
withdrawal_topic = web3.Web3.keccak(text="Withdrawal(address,uint256)").hex()
mint_topic = web3.Web3.keccak(text="Mint(address,uint256)").hex()
topics = [[transfer_topic, approval_topic, deposit_topic, withdrawal_topic, mint_topic]]


@pytest.mark.parametrize("bedrock", [True, False])
def test_eth_getLogs(bedrock):
    max_block_number = None
    if bedrock:
        max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    for _ in range(16):
        if bedrock:
            target_block_number = random.randint(BEDROCK_START, max_block_number - 100)
        else:
            target_block_number = random.randint(0, BEDROCK_START - 101)
        geth_res = geth_client.send_request(
            RPCMethod.GetLogs,
            params=[
                {
                    "topics": topics,
                    "fromBlock": hex(target_block_number),
                    "toBlock": hex(target_block_number + 100),
                },
            ],
        )
        erigon_res = erigon_client.send_request(
            RPCMethod.GetLogs,
            params=[
                {
                    "topics": topics,
                    "fromBlock": hex(target_block_number),
                    "toBlock": hex(target_block_number + 100),
                },
            ],
        )
        assert geth_res == erigon_res


target_topic = web3.Web3.keccak(
    text="SentMessage(address,address,bytes,uint256,uint256)"
).hex()
# restricting target topic by 1 to avoid timeout for geth client
target_topics = [target_topic]


def test_eth_getLogs_overlap():
    max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    for _ in range(16):
        start_block_number = random.randint(0, BEDROCK_START - 101)
        end_block_number = random.randint(BEDROCK_START, max_block_number - 100)
        geth_res = geth_client.send_request(
            RPCMethod.GetLogs,
            params=[
                {
                    "topics": target_topics,
                    "fromBlock": hex(start_block_number),
                    "toBlock": hex(end_block_number + 100),
                },
            ],
        )
        erigon_res = erigon_client.send_request(
            RPCMethod.GetLogs,
            params=[
                {
                    "topics": target_topics,
                    "fromBlock": hex(start_block_number),
                    "toBlock": hex(end_block_number + 100),
                },
            ],
        )
        assert geth_res == erigon_res
