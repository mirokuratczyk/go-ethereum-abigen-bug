#!/bin/bash

set -e -u -x

go test -v -fuzz=FuzzMint -run XXX -fuzztime=10s .

