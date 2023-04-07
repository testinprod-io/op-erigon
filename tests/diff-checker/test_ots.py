import json
import random

import pytest

from common import RPCMethod, erigon, erigon_client, geth, geth_client
from constants import BEDROCK_START


# comparing with op-erigon relaying enabled or disabled
# therefore we only test bedrock block because ots method only
# works for bedrock when no relay
@pytest.mark.parametrize("bedrock", [True])
def test_ots_traceTransaction(bedrock):
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
        geth_trace = geth_client.send_request(
            RPCMethod.OtterscanTraceTransaction, params=[hash.hex()]
        )
        erigon_trace = erigon_client.send_request(
            RPCMethod.OtterscanTraceTransaction, params=[hash.hex()]
        )
        assert geth_trace == erigon_trace
