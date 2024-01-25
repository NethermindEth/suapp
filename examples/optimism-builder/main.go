package main

import (
	"fmt"
	"github.com/flashbots/suapp-examples/framework"
)

func main() {
	fr := framework.New()
	contract := fr.DeployContract("optimism-builder.sol/OpBuilder.json")
	fmt.Println("contract deployed at address:", contract.Address())
}
