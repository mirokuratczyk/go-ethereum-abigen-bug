# go-ethereum-abigen-bug
Demonstrates abigen bug fixed in https://github.com/ethereum/go-ethereum/pull/31607

## Running

First, start anvil:
```
anvil
```

Next run the tests:
```
go test -v
go test -v -fuzz=FuzzMint -run XXX -fuzztime=10s 
```
