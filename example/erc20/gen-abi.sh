#!/bin/bash

set -e -u -x

abigen --abi ERC20.abi --bin ERC20.bin --pkg erc20 --type ERC20 --out ERC20.go

