import json
import random
from typing import List

from solcx import compile_source, install_solc
from web3 import HTTPProvider, Web3
from web3.middleware import geth_poa_middleware

with open("endpoint.json", "r") as f:
    endpoints = json.loads(f.read())
    URL_OP_ERIGON = endpoints["op-erigon"]


with open("secret.json", "r") as f:
    secret = json.loads(f.read())
    addr = secret["addr"]
    key = secret["key"]


install_solc(version="0.8.17")
web3 = Web3(HTTPProvider(URL_OP_ERIGON))
web3.middleware_onion.inject(geth_poa_middleware, layer=0)


with open("test.sol") as f:
    source = f.read()

compiled_sol = compile_source(source, output_values=["abi", "bin"])

abi = compiled_sol["<stdin>:Counter"]["abi"]
bytecode = compiled_sol["<stdin>:Counter"]["bin"]

Test = web3.eth.contract(abi=abi, bytecode=bytecode)

nonce = web3.eth.getTransactionCount(addr)
tx = {
    "from": addr,
    "gas": 1000000,
    "gasPrice": 75000000,
    "nonce": nonce,
    "chainId": 420,
    "data": bytecode,
}
signed_tx = web3.eth.account.sign_transaction(tx, private_key=key)
tx_hash = web3.eth.send_raw_transaction(signed_tx.rawTransaction)
print(f"{tx_hash = }")

tx_receipt = web3.eth.waitForTransactionReceipt(tx_hash)
print(f"{tx_receipt = }")

test_addr = tx_receipt.contractAddress
test = web3.eth.contract(
    address=test_addr,
    abi=abi,
)

prev_count = test.functions.get().call()
print(f"{prev_count = }")

data = test.functions.inc()._encode_transaction_data()
nonce = web3.eth.getTransactionCount(addr)
tx = {
    "from": addr,
    "to": test_addr,
    "gas": 1000000,
    "gasPrice": 75000000,
    "nonce": nonce,
    "chainId": 420,
    "data": data,
}
signed_tx = web3.eth.account.sign_transaction(tx, private_key=key)
tx_hash = web3.eth.send_raw_transaction(signed_tx.rawTransaction)
print(f"{tx_hash = }")

tx_receipt = web3.eth.waitForTransactionReceipt(tx_hash)
print(f"{tx_receipt = }")

after_count = test.functions.get().call()
print(f"{after_count = }")

assert after_count - prev_count == 1, (after_count, prev_count)
