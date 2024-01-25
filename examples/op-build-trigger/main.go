package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flashbots/suapp-examples/framework"
	"github.com/sirupsen/logrus"
)

const (
	LogLevel               = "debug"
	OpDevAccountPrivKeyHex = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	OpChainId              = 901
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

	log.Info("2. Send transaction")

	client, err := ethclient.Dial("http://localhost:9545")
	OpDevAccountPrivKey := framework.NewPrivKeyFromHex(OpDevAccountPrivKeyHex)
	ephemeralAddr := framework.GeneratePrivKey().Address()

	blkNo, err := client.BlockNumber(context.Background())

	if err != nil {
		log.Fatal(err)
	}

	nonce, err := client.NonceAt(context.Background(), OpDevAccountPrivKey.Address(), big.NewInt(int64(blkNo)))

	opTxn1 := types.NewTransaction(nonce, ephemeralAddr, big.NewInt(1000), 21000, big.NewInt(13), nil)

	opTxn1Signed, err := types.SignTx(opTxn1, types.NewEIP155Signer(big.NewInt(OpChainId)), OpDevAccountPrivKey.Priv)

	if err != nil {
		log.Fatal(err)
	}

	bundle := &types.SBundle{
		Txs:             types.Transactions{opTxn1Signed},
		RevertingHashes: []common.Hash{},
	}
	bundleBytes, _ := json.Marshal(bundle)

	contractAddr1 := contract.Ref(testAddr1)

	blockNo, err := client.BlockNumber(context.Background())
	receipt := contractAddr1.SendTransaction("newTx", []interface{}{blockNo}, bundleBytes)
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
