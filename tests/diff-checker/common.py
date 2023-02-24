import web3
from web3.middleware import geth_poa_middleware
import json

with open("endpoint.json", "r") as f:
    endpoints = json.loads(f.read())
    URL_OP_GETH = endpoints["op-geth"]
    URL_OP_ERIGON = endpoints["op-erigon"]

erigon = web3.Web3(web3.HTTPProvider(URL_OP_ERIGON))
erigon.middleware_onion.inject(geth_poa_middleware, layer=0)

geth = web3.Web3(web3.HTTPProvider(URL_OP_GETH))
geth.middleware_onion.inject(geth_poa_middleware, layer=0)
