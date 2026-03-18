# go-ethereum-abigen-bug
Demonstrates abigen bug now fixed in go-ethereum

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
