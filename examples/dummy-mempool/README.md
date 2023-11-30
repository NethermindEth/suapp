# Dummy mempool example to play with suave

## Functionality

- user send his "transaction" to the mempool
- mempool stores the transaction in the confidential store
- another smart contract "builder" fetches the transaction from the mempool and builds a "block"

## Getting started

### Deployment

- start local suave chain
- run `go run deploy.go`

## Running tests

- run `forge test`
