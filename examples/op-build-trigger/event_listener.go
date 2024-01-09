package main

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flashbots/suapp-examples/framework"
	"github.com/sirupsen/logrus"
)

const (
	SuaveNodeRpc             = "ws://127.0.0.1:8546"
	ContractAddrEnv          = "CONTRACT_ADDR"
	EventTypeName            = "BidEvent"
	ContractAbiJsonPath      = "optimism-builder.sol/OpBuilder.json"
	ContractBuildBlockMethod = "buildBlock"

	// BuilderPrivKey Builder address "0xDceef22333b11aD2CAb54Be2A8ECe08EE64D919C" needs to be funded
	BuilderPrivKey = "9b6fa7074578db9ce7752ac85bf5c0acd071c7115f8fc02abdd435918edd4b62"
)

var (
	errMissingContractAddr = errors.New("missing contract address to listen events for")
	errArtifactRead        = errors.New("failed to read artifact from " + ContractAbiJsonPath)
	errMissingMethod       = errors.New("missing method " + ContractBuildBlockMethod + " in abi" + ContractAbiJsonPath)
	errUnsuccessfulTx      = errors.New("unsuccessful transaction to " + ContractBuildBlockMethod)
)

type EventListener struct {
	ethclient    *ethclient.Client
	log          *logrus.Entry
	contractAddr common.Address
	artifact     *framework.Artifact
	eventAbi     abi.Event
}

func NewEventListener(log *logrus.Entry, contractAddress common.Address) (*EventListener, error) {
	ethClient, err := ethclient.Dial(SuaveNodeRpc)
	if err != nil {
		return nil, err
	}

	artifact, err := framework.ReadArtifact(ContractAbiJsonPath)
	if err != nil {
		return nil, errArtifactRead
	}
	eventAbi := artifact.Abi.Events[EventTypeName]

	return &EventListener{
		log:          log,
		ethclient:    ethClient,
		contractAddr: contractAddress,
		artifact:     artifact,
		eventAbi:     eventAbi,
	}, nil
}

func (el *EventListener) Listen() {
	el.log.Println("Start listen to events", "RPC", SuaveNodeRpc, "contract", el.contractAddr.Hex())
	query := ethereum.FilterQuery{
		Addresses: []common.Address{el.contractAddr},
	}
	logs := make(chan types.Log)
	sub, err := el.ethclient.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		el.log.Fatal("Create logs filter: ", err)
	}

	for {
		select {
		case err := <-sub.Err():
			el.log.Fatal("Subscription: ", err)
		case vLog := <-logs:
			event := &BidEvent{}
			if err := event.Unpack(&vLog, el.eventAbi); err != nil {
				el.log.Warn("Failed to unpack hint event: ", err)
				break
			}

			el.log.WithField("Event", event).Println("Event received, id:", event.BidId)
			el.TriggerBlockBuild(event.BidId)
		}
	}
}

// Assumes Builder account is funded, see `BuilderAddr` in constants
func (el *EventListener) TriggerBlockBuild(builderBid types.BidId) error {
	fr := framework.New()
	builder := framework.NewPrivKeyFromHex(BuilderPrivKey)

	fundBalance := big.NewInt(1000000000000000000)
	fr.FundAccount(builder.Address(), fundBalance)

	ctrct := fr.ContractAt(el.contractAddr, el.artifact.Abi)
	builderCtrct := ctrct.Ref(builder)

	blkNo := uint64(10)
	receipt := builderCtrct.SendTransaction(ContractBuildBlockMethod, []interface{}{blkNo}, nil)

	if receipt.Status == 0 {
		return errUnsuccessfulTx
	}

	return nil
}

type BidEvent struct {
	BidId       [16]byte
	decryptCond uint64
	peekers     []common.Address
}

func (h *BidEvent) Unpack(log *types.Log, eventAbi abi.Event) error {
	unpacked, err := eventAbi.Inputs.Unpack(log.Data)
	if err != nil {
		return err
	}
	h.BidId = unpacked[0].([16]byte)
	h.decryptCond = unpacked[1].(uint64)
	h.peekers = unpacked[2].([]common.Address)
	return nil
}
