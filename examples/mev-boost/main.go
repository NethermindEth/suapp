package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flashbots/suapp-examples/framework"
)

var builderBoostBidEventABI abi.Event
var bidEventABI abi.Event

func init() {
	artifact, _ := framework.ReadArtifact("mev-boost.sol/MevBoost.json")
	builderBoostBidEventABI = artifact.Abi.Events["BuilderBoostBidEvent"]
	bidEventABI = artifact.Abi.Events["BidEvent"]
}

type BidEvent struct {
	BidId               [16]byte
	DecryptionCondition uint64
	allowedPeekers      []common.Address
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
}

func (b *BidEvent) Unpack(log *types.Log) error {
	unpacked, err := bidEventABI.Inputs.Unpack(log.Data)
	if err != nil {
		return err
	}
	b.BidId = unpacked[0].([16]byte)
	b.DecryptionCondition = unpacked[1].(uint64)
	b.allowedPeekers = unpacked[2].([]common.Address)
	return nil
}

type relayHandlerExample struct {
}

func (rl *relayHandlerExample) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(bodyBytes))
}

func main() {
	fakeRelayer := httptest.NewServer(&relayHandlerExample{})
	defer fakeRelayer.Close()

	fr := framework.New()
	contract := fr.DeployContract("mev-boost.sol/MevBoost.json")
	bundleContract := fr.DeployContract("mev-boost.sol/BundleBidContract.json")

	fmt.Println("1. Create and fund test accounts")

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

	fmt.Println("2. Send a bundle bid.")
	bundle := &types.SBundle{
		Txs:             types.Transactions{ethTxn1},
		RevertingHashes: []common.Hash{},
	}

	bundleBytes, _ := json.Marshal(bundle)
	contractAddr1 := bundleContract.Ref(testAddr1)
	allowedPeekers := []common.Address{bundleContract.Address()}
	receipt := contractAddr1.SendTransaction(
		"newBid", []interface{}{10, allowedPeekers, allowedPeekers}, bundleBytes)

	if receipt.Status != 1 {
		jsonEncodedReceipt, _ := receipt.MarshalJSON()
		fmt.Println("Sending bundle request failed", string(jsonEncodedReceipt))
		return
	}

	bidEvent := &BidEvent{}
	if err := bidEvent.Unpack(receipt.Logs[0]); err != nil {
		panic(err)
	}

	fmt.Println("3. Build block.")

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
	contractAddr2 := contract.Ref(testAddr1)
	receipt = contractAddr2.SendTransaction("buildBlock", []interface{}{buildBlockArgs, 0}, nil)
	if receipt.Status != 1 {
		jsonEncodedReceipt, _ := receipt.MarshalJSON()
		fmt.Println("Sending build block request failed", string(jsonEncodedReceipt))
		return
	}

	blockBidEvent := &BidEvent{}
	if err := blockBidEvent.Unpack(receipt.Logs[1]); err != nil {
		panic(err)
	}

	fmt.Println("4. Get block.")
	block := contractAddr2.CallWithArgs("getBlock", []interface{}{blockBidEvent.BidId})
	fmt.Println(block)
}
