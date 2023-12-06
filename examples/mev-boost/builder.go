package main

import (
	"log"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flashbots/suapp-examples/framework"
)

const (
	ENV_BLOCK_TIME_SECONDS = "BLOCK_TIME"
	DEFAUKT_BLOCK_TIME     = "12"
)

func main() {
	blockTime := getBlockTime()

	fr := framework.New()
	contract := fr.DeployContract("mev-boost.sol/MevBoost.json")
	log.Println("Builder contract deployed at ", contract.Address())

	testAddr1 := framework.GeneratePrivKey()

	log.Println("Fund test account")
	fundBalance := big.NewInt(100000000000000000)
	fr.FundAccount(testAddr1.Address(), fundBalance)

	builder := contract.Ref(testAddr1)
	BuilderLoop(blockTime, builder)
}

func BuilderLoop(blockTime time.Duration, builder *framework.Contract) {
	log.Printf("block time: %s", blockTime)
	for {
		time.Sleep(blockTime)
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
	}
}

func getBlockTime() time.Duration {
	blockTime, err := strconv.Atoi(getEnv(ENV_BLOCK_TIME_SECONDS, DEFAUKT_BLOCK_TIME))
	if err != nil {
		log.Printf("ERROR: failed to parse block time, using %s seconds: %s", DEFAUKT_BLOCK_TIME, err)
		blockTime, _ = strconv.Atoi(DEFAUKT_BLOCK_TIME)
	}
	return time.Duration(blockTime) * time.Second
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

/// -------------------

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

//struct BuildBlockArgs {
//	uint64 slot;
//	bytes proposerPubkey;
//	bytes32 parent;
//	uint64 timestamp;
//	address feeRecipient;
//	uint64 gasLimit;
//	bytes32 random;
//	Withdrawal[] withdrawals;
//	bytes extra;
//}
//struct Withdrawal {
//	uint64 index;
//	uint64 validator;
//	address Address;
//	uint64 amount;
//}
