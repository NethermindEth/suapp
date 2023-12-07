package main

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flashbots/suapp-examples/framework"
	"log"
	"math/big"
)

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

	log.Println("2. Send a bundle bid.")
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

	hintEvent := &HintEvent{}
	if err := hintEvent.Unpack(receipt.Logs[0]); err != nil {
		log.Fatalf("Failed to unpack hint event: %s", err)
	}
	log.Println("Received hint event ", hintEvent.BidId)

	// Step 3. Send the backrun transaction
	targetAddr := testAddr1.Address()
	ethTxnBackrun, _ := fr.SignTx(testAddr2, &types.LegacyTx{
		To:       &targetAddr,
		Value:    big.NewInt(1000),
		Gas:      21420,
		GasPrice: big.NewInt(13),
	})

	log.Println("3. Send backrun")

	backRunBundle := &types.SBundle{
		Txs:             types.Transactions{ethTxnBackrun},
		RevertingHashes: []common.Hash{},
	}
	backRunBundleBytes, _ := json.Marshal(backRunBundle)

	// backrun inputs
	ofaSearcher := ofaContract.Ref(testAddr2)
	receipt = ofaSearcher.SendTransaction(
		"newMatch", []interface{}{hintEvent.BidId}, backRunBundleBytes)

	matchEvent := &HintEvent{}
	if err := matchEvent.Unpack(receipt.Logs[0]); err != nil {
		panic(err)
	}

	log.Println("Match event id", matchEvent.BidId)

	log.Println("3. Build block.")

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
	mevFunded := mevContract.Ref(testAddr1)
	receipt = mevFunded.SendTransaction("buildBlock", []interface{}{buildBlockArgs, uint64(0)}, nil)
	if receipt.Status != 1 {
		jsonEncodedReceipt, _ := receipt.MarshalJSON()
		log.Println("Sending build block request failed", string(jsonEncodedReceipt))
		return
	}

	// Next steps: proposer blind-sing the block header, send it to mevContract to receive block's payload
	// ...
}
