package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flashbots/suapp-examples/framework"
	"github.com/sirupsen/logrus"
)

const (
	LogLevel = "debug"
)

func main() {
	log := logrus.NewEntry(logrus.New())
	log.Logger.SetOutput(os.Stdout)

	lvl, err := logrus.ParseLevel(LogLevel)
	if err != nil {
		flag.Usage()
		log.Fatalf("invalid loglevel: %s", LogLevel)
	}
	log.Logger.SetLevel(lvl)

	fr := framework.New()

	balance, err := fr.Balance(common.HexToAddress("0xb5feafbdd752ad52afb7e1bd2e40432a485bbb7f"))
	fmt.Printf("Balance of account 0xb5feafbdd752ad52afb7e1bd2e40432a485bbb7f: %d\n", balance)

	contract := fr.DeployContract("optimism-builder.sol/OpBuilder.json")
	log.Infof("contract deployed at address: %s", contract.Address())

	evListSrv, err := NewEventListener(log, contract.Address())
	if err != nil {
		log.WithError(err).Fatal("failed creating the event listener")
	}

	go evListSrv.Listen()

	// Send transaction to contract newTx()
	log.Info("1. Create and fund test accounts")
	testAddr1 := framework.GeneratePrivKey()
	testAddr2 := framework.GeneratePrivKey()

	log.Infof("Address 1: %s", testAddr1.Address())
	log.Infof("Address 2: %s", testAddr2.Address())

	fundBalance := big.NewInt(1000000000000000000)
	fr.FundAccount(testAddr1.Address(), fundBalance)
	fr.FundAccount(testAddr2.Address(), fundBalance)

	targetAddr := testAddr1.Address()

	tx1, _ := fr.SignTx(testAddr1, &types.LegacyTx{
		To:       &targetAddr,
		Value:    big.NewInt(1000),
		Gas:      21000,
		GasPrice: big.NewInt(13),
	})

	log.Info("2. Send transaction")

	bundle := &types.SBundle{
		Txs:             types.Transactions{tx1},
		RevertingHashes: []common.Hash{},
	}
	bundleBytes, _ := json.Marshal(bundle)

	contractAddr1 := contract.Ref(testAddr1)
	receipt := contractAddr1.SendTransaction("newTx", []interface{}{}, bundleBytes)
	log.Info("Transaction sent", "receipt", receipt)

	signalCh := make(chan os.Signal, 1)
	for {
		select {
		case <-signalCh:
			log.Info("Exiting")
			return
		}
	}
}
