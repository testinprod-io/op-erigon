from common import erigon, geth, erigon_client, geth_client, RPCMethod, compare_txs
from constants import BEDROCK_START
import random
import pytest
import web3


@pytest.mark.parametrize('bedrock', [True, False])
def test_eth_call(bedrock):
    # L2CrossDomainMessenger
    contract_addr = '0x4200000000000000000000000000000000000007'
    selector = web3.Web3.keccak(text='messageNonce()')[:4].hex()
    max_block_number = None
    if bedrock:
        max_block_number = min(erigon.eth.block_number, geth.eth.block_number)
    for _ in range(16):
        if bedrock:
            target_block_number = random.randint(BEDROCK_START, max_block_number)
        else:
            target_block_number = random.randint(0, BEDROCK_START - 1)
        geth_res = geth_client.send_request(
            RPCMethod.Call,
            params=[
                {
                    'to': contract_addr,
                    'data': selector,
                },
                hex(target_block_number)
            ]
        )
        erigon_res = erigon_client.send_request(
            RPCMethod.Call,
            params=[
                {
                    'to': contract_addr,
                    'data': selector,
                },
                hex(target_block_number)
            ]
        )
        assert geth_res == erigon_res




