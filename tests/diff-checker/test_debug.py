import json
import random

import pytest

from common import RPCMethod, erigon, erigon_client, geth, geth_client
from constants import BEDROCK_START


# relayed erigon and erigon equals
# relayed erigon and geth differs: geth contains 'value':'0x0'
# ignore this diff from now
def ignore_value(data):
    # very hacky way to ignore 'value': '0x0
    # i know i know.. shitty but do not want to recurse
    msg = json.dumps(data).replace(', "value: "0x0"', "")
    msg = msg.replace('"value": "0x0", ', "")
    return json.loads(msg)


@pytest.mark.parametrize("bedrock", [False, True])
def test_debug_traceTransaction(bedrock):
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
            RPCMethod.TraceTransaction, params=[hash.hex(), {"tracer": "callTracer"}]
        )
        erigon_trace = erigon_client.send_request(
            RPCMethod.TraceTransaction, params=[hash.hex(), {"tracer": "callTracer"}]
        )
        # time will always differ
        geth_trace.pop("time", None)
        erigon_trace.pop("time", None)
        # ignore value when value is zero
        if bedrock:
            geth_trace = ignore_value(geth_trace)
            erigon_trace = ignore_value(erigon_trace)
        assert geth_trace == erigon_trace, json.dumps(geth_trace)
