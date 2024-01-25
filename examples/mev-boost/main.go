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

func main() {
	fr := framework.New()
	ofaContract := fr.DeployContract("mev-boost.sol/OFAPrivate.json")
	mevContract := fr.DeployContract("mev-boost.sol/MevBoost.json")

	log.Println("1. Create and fund test accounts")

	testAddr1 := framework.GeneratePrivKey()
	testAddr2 := framework.GeneratePrivKey()

	fundBalance := big.NewInt(100000000000000000)
	fr.FundAccount(testAddr1.Address(), fundBalance)
	fr.FundAccount(testAddr2.Address(), fundBalance)

	dstAddr := testAddr2.Address()
	ethTxn1, _ := fr.SignTx(testAddr1, &types.LegacyTx{
		To:       &dstAddr,
		Value:    big.NewInt(1000),
		Gas:      21000,
		GasPrice: big.NewInt(13),
	})

	log.Println("2. Start off-chain actors")
	ethClient, _ := ethclient.Dial("ws://127.0.0.1:8546")
	go SearcherLoop(
		ethClient,
		ofaContract,
		testAddr2,
		testAddr1.Address())
	go BuilderLoop(6*time.Second, mevContract, testAddr1)

	log.Println("3. Send a bundle bid.")
	bundle := &types.SBundle{
		Txs:             types.Transactions{ethTxn1},
		RevertingHashes: []common.Hash{},
	}

	bundleBytes, _ := json.Marshal(bundle)
	ofaFunded := ofaContract.Ref(testAddr1)
	allowedPeekers := []common.Address{mevContract.Address()}
	_ = allowedPeekers // do nothing we will see
	receipt := ofaFunded.SendTransaction(
		"newOrder", []interface{}{}, bundleBytes)

	if receipt.Status != 1 {
		jsonEncodedReceipt, _ := receipt.MarshalJSON()
		fmt.Println("Sending bundle request failed", string(jsonEncodedReceipt))
		return
	}

	// Next steps: proposer blind-sing the block header, send it to mevContract to receive block's payload
	// ...

	time.Sleep(25 * time.Second)
}

// Off-chain Actors code

func SearcherLoop(
	client *ethclient.Client,
	ofaContract *framework.Contract,
	searcher *framework.PrivKey,
	beneficiary common.Address,
) {
	fr := framework.New()
	query := ethereum.FilterQuery{
		Addresses: []common.Address{ofaContract.Address()},
	}
	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatal("[SEARCHER] Create logs filter: ", err)
	}

	ofaSearcher := ofaContract.Ref(searcher)
	knownBids := map[types.BidId]struct{}{}

	log.Println("[SEARCHER] Start listen to events")
	for {
		select {
		case err := <-sub.Err():
			log.Fatal("[SEARCHER] Subscription: ", err)
		case vLog := <-logs:
			hintEvent := &HintEvent{}
			if err := hintEvent.Unpack(&vLog); err != nil {
				log.Println("WARN[SEARCHER]: Failed to unpack hint event: ", err)
				break
			}
			if _, ok := knownBids[hintEvent.BidId]; ok {
				log.Println("[SEARCHER] Already known bid", hintEvent.BidId)
				break
			}

			log.Println("[SEARCHER] Hint event received, id:", hintEvent.BidId)
			log.Println("[SEARCHER] Send backrun")
			backRunBundleBytes := createBackrunBundle(fr, searcher, beneficiary)

			// backrun inputs
			receipt := ofaSearcher.SendTransaction(
				"newMatch", []interface{}{hintEvent.BidId}, backRunBundleBytes)

			matchEvent := &HintEvent{}
			if err := matchEvent.Unpack(receipt.Logs[0]); err != nil {
				log.Fatalf("Failed to unpack match event: %s", err)
			}
			knownBids[matchEvent.BidId] = struct{}{}
		}
	}
}

func BuilderLoop(
	blockTime time.Duration,
	mevContract *framework.Contract,
	account *framework.PrivKey) {
	log.Printf("[BUILDER] block time: %s", blockTime)
	builder := mevContract.Ref(account)

	for {
		time.Sleep(blockTime)
		log.Println("[BUILDER] Block building started")

		buildBlockArgs := &BuildBlockArgs{
			Slot:           0,
			ProposerPubkey: []byte{},
			Parent:         common.Hash{},
			Timestamp:      0,
			FeeRecipient:   common.Address{},
			GasLimit:       0,
			Random:         common.Hash{},
			Withdrawals: []struct {
				Index     uint64
				Validator uint64
				Address   common.Address
				Amount    uint64
			}{},
		}

		receipt := builder.SendTransaction("buildBlock", []interface{}{buildBlockArgs, uint64(0)}, nil)
		if receipt.Status != 1 {
			jsonEncodedReceipt, _ := receipt.MarshalJSON()
			log.Println("WARN: Sending build block request failed ", string(jsonEncodedReceipt))
		}
		log.Println("[BUILDER] Block building completed")
	}
}

// Structs and helpers

var hintEventABI abi.Event

func init() {
	artifact, err := framework.ReadArtifact("mev-boost.sol/OFAPrivate.json")
	if err != nil {
		log.Panic("Failed to read artifact: ", err)
	}
	hintEventABI = artifact.Abi.Events["HintEvent"]
}

type BuildBlockArgs struct {
	Slot           uint64
	ProposerPubkey []byte
	Parent         common.Hash
	Timestamp      uint64
	FeeRecipient   common.Address
	GasLimit       uint64
	Random         common.Hash
	Withdrawals    []struct {
		Index     uint64
		Validator uint64
		Address   common.Address
		Amount    uint64
	}
	Extra []byte
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

func createBackrunBundle(fr *framework.Framework, searcher *framework.PrivKey, beneficiary common.Address) []byte {
	ethTxnBackrun, _ := fr.SignTx(searcher, &types.LegacyTx{
		To:       &beneficiary,
		Value:    big.NewInt(1000),
		Gas:      21420,
		GasPrice: big.NewInt(13),
	})

	backRunBundle := &types.SBundle{
		Txs:             types.Transactions{ethTxnBackrun},
		RevertingHashes: []common.Hash{},
	}
	backRunBundleBytes, _ := json.Marshal(backRunBundle)

	return backRunBundleBytes
}
