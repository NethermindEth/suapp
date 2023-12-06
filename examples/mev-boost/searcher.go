package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flashbots/suapp-examples/framework"
)

const (
	RPC_URL = "ws://127.0.0.1:8546"
)

func main() {
	client, err := ethclient.Dial(RPC_URL)
	if err != nil {
		log.Fatal("Failed to connect to RPC: ", err)
	}

	fr := framework.New()
	bidContract := fr.DeployContract("ofa-private.sol/OFAPrivate.json")
	log.Println("Bid contract deployed at ", bidContract.Address())

	go SearcherLoop(client, bidContract.Address())

	fmt.Println("1. Create and fund test accounts")
	testAddr1 := framework.GeneratePrivKey()
	testAddr2 := framework.GeneratePrivKey()

	fundBalance := big.NewInt(100000000000000000)
	fr.FundAccount(testAddr1.Address(), fundBalance)
	fr.FundAccount(testAddr2.Address(), fundBalance)

	targeAddr := testAddr1.Address()

	ethTxn1, _ := fr.SignTx(testAddr1, &types.LegacyTx{
		To:       &targeAddr,
		Value:    big.NewInt(1000),
		Gas:      21000,
		GasPrice: big.NewInt(13),
	})

	// Step 2. Send the initial transaction
	fmt.Println("2. Send bid")

	refundPercent := 10
	bundle := &types.SBundle{
		Txs:             types.Transactions{ethTxn1},
		RevertingHashes: []common.Hash{},
		RefundPercent:   &refundPercent,
	}
	bundleBytes, _ := json.Marshal(bundle)

	// new bid inputs
	contractAddr1 := bidContract.Ref(testAddr1)
	receipt := contractAddr1.SendTransaction("newOrder", []interface{}{}, bundleBytes)
	if receipt.Status != 1 {
		jsonEncodedReceipt, _ := receipt.MarshalJSON()
		fmt.Println("Sending build block request failed", string(jsonEncodedReceipt))
		return
	}

	log.Println("3. Wait for hint event - 25 seconds")
	time.Sleep(25 * time.Second)
}

func SearcherLoop(client *ethclient.Client, bidContract common.Address) {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{bidContract},
	}
	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatal("Create logs filter: ", err)
	}

	log.Println("Start listen to events")
	for {
		select {
		case err := <-sub.Err():
			log.Fatal("Subscription: ", err)
		case vLog := <-logs:
			hintEvent := &HintEvent{}
			if err := hintEvent.Unpack(&vLog); err != nil {
				log.Println("WARN: Failed to unpack hint event: ", err)
			}

			log.Println("Hint event received, id:", hintEvent.BidId)
		}
	}
}

var hintEventABI abi.Event

func init() {
	artifact, err := framework.ReadArtifact("ofa-private.sol/OFAPrivate.json")
	if err != nil {
		log.Fatal("Failed to read artifact: ", err)
	}
	hintEventABI = artifact.Abi.Events["HintEvent"]
}

type HintEvent struct {
	BidId [16]byte
	Hint  []byte
}

func (h *HintEvent) Unpack(log *types.Log) error {
	unpacked, err := hintEventABI.Inputs.Unpack(log.Data)
	if err != nil {
		return err
	}
	h.BidId = unpacked[0].([16]byte)
	h.Hint = unpacked[1].([]byte)
	return nil
}
