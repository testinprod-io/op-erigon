import json

import requests
import web3
from web3.middleware import geth_poa_middleware


class RPCMethod:
    Call = "eth_call"
    EstimateGas = "eth_estimateGas"
    GetLogs = "eth_getLogs"
    GetBlockByNumber = "eth_getBlockByNumber"
    GetBlockByHash = "eth_getBlockByHash"
    GetTransactionByHash = "eth_getTransactionByHash"
    GetTransactionByBlockHashAndIndex = "eth_getTransactionByBlockHashAndIndex"
    GetTransactionByBlockNumberAndIndex = "eth_getTransactionByBlockNumberAndIndex"
    GetTransactionReceipt = "eth_getTransactionReceipt"
    GetBalance = "eth_getBalance"
    BlockNumber = "eth_blockNumber"
    GetStorageAt = "eth_getStorageAt"


class RPCError(BaseException):
    pass


class RPCClient:
    def __init__(self, url):
        self.url = url

    def send_request(self, method, params=None, token=None, allow_error=False):
        headers = {
            "Content-Type": "application/json",
        }
        if token is not None:
            headers["Authorization"] = f"Bearer {token}"
        data = {
            "jsonrpc": "2.0",
            "method": method,
            "params": [] if params is None else params,
            "id": 1,
        }
        res = requests.post(self.url, json=data, headers=headers).json()
        if "error" in res:
            if allow_error:
                return res["error"]
            else:
                raise RPCError(res["error"]["message"])
        return res["result"]


with open("endpoint.json", "r") as f:
    endpoints = json.loads(f.read())
    URL_OP_GETH = endpoints["op-geth"]
    URL_OP_ERIGON = endpoints["op-erigon"]

erigon = web3.Web3(web3.HTTPProvider(URL_OP_ERIGON))
erigon.middleware_onion.inject(geth_poa_middleware, layer=0)

geth = web3.Web3(web3.HTTPProvider(URL_OP_GETH))
geth.middleware_onion.inject(geth_poa_middleware, layer=0)

erigon_client = RPCClient(URL_OP_ERIGON)
geth_client = RPCClient(URL_OP_GETH)


def compare_txs(tx1, tx2):
    for key in set(tx1.keys()) | set(tx2.keys()):
        if key in tx1 and key in tx2:
            assert tx1[key] == tx2[key]
        elif key not in tx1:
            assert tx2[key] == None
        elif key not in tx2:
            assert tx1[key] == None
