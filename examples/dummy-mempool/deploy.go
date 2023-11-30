package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flashbots/suapp-examples/framework"
)

func main() {
	fr := framework.New()
	contract := fr.DeployContract("mempool.sol/Mempool.json")

	var mempoolAddr common.Address = contract.Address()
	fmt.Printf("Contract Mempool deployed at %s address", mempoolAddr.Hex())
}
