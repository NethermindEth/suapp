package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flashbots/suapp-examples/framework"
	"log"
	"math/big"
)

var (
	hintEventABI abi.Event
	artifact     *framework.Artifact
)

func main() {
	fr := framework.New()
	contract := fr.DeployContract("mempool.sol/Mempool.json")

	var mempoolAddr common.Address = contract.Address()
	fmt.Printf("Contract Mempool deployed at %s address", mempoolAddr.Hex())

	// Sender of CCR
	sender := framework.GeneratePrivKey()
	fmt.Println("New account created: ", sender.Address().Hex())

	fundBalance := big.NewInt(100000000000000000)
	fr.FundAccount(sender.Address(), fundBalance)

	// Send a confidential request
	confidentialInp := uint8(121)
	var fn abi.Method = artifact.Abi.Methods["submitTx"]
	arg, err := fn.Inputs.Pack(confidentialInp)
	// TODO: above expression is failing with error: panic: argument count mismatch: got 1 for 0
	if err != nil {
		log.Panic("argument packing failed", err)
	}
	contract.SendTransaction("submitConfidentialTx", nil, arg)
}

type HintEvent struct {
	BidId [16]byte
	Hint  uint8
}

func (h *HintEvent) Unpack(log *types.Log) error {
	unpacked, err := hintEventABI.Inputs.Unpack(log.Data)
	if err != nil {
		return err
	}
	h.BidId = unpacked[0].([16]byte)
	h.Hint = unpacked[1].(uint8)
	return nil
}

func init() {
	artifact, _ = framework.ReadArtifact("ofa-private.sol/OFAPrivate.json")
	hintEventABI = artifact.Abi.Events["HintEvent"]
}
