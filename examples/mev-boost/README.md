# Mev-Boost build on Suave

[Here is a diagram for this example](https://app.excalidraw.com/l/AS0AxovxaIz/McxYqNvQah)

## Example description

This example consists of:

- 2 smart contracts:
    - Private Order Flow Auction
    - Block Building Auction
- 2 off-chain actors:
    - Proposer - which backruns user's orders
    - Builder - builds blocks from the orders (for simplicity it runs in fixed time intervals)
  
## How to run

1. Run suave-geth (the kettle)
    ```bash
    suave  --suave.dev
    ```
2. Compile contracts
    ```bash
    cd examples/mev-boost
    forge build
    ```
3. Run the example
    ```bash
    cd examples/mev-boost
    go run main.go
    ```

