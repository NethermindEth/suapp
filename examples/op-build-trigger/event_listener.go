package main

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flashbots/suapp-examples/framework"
	"github.com/sirupsen/logrus"
)

const (
	SuaveNodeRpc             = "ws://127.0.0.1:11546"
	ContractAddrEnv          = "CONTRACT_ADDR"
	NewBundleEventName       = "NewBundleEvent"
	NewBuilderBidEventName   = "NewBuilderBidEvent"
	ContractAbiJsonPath      = "optimism-builder.sol/OpBuilder.json"
	ContractBuildBlockMethod = "buildBlock"
	ContractPostBlockMethod  = "submitBlock"

	// BuilderPrivKey Builder address "0xDceef22333b11aD2CAb54Be2A8ECe08EE64D919C" needs to be funded
	BuilderPrivKey = "91ab9a7e53c220e6210460b65a7a3bb2ca181412a8a7b43ff336b3df1737ce12"
)

var (
	errMissingContractAddr = errors.New("missing contract address to listen events for")
	errArtifactRead        = errors.New("failed to read artifact from " + ContractAbiJsonPath)
	errMissingMethod       = errors.New("missing method " + ContractBuildBlockMethod + " in abi" + ContractAbiJsonPath)
	errUnsuccessfulTx      = errors.New("unsuccessful transaction to " + ContractBuildBlockMethod)
	errSubmitBlock         = errors.New("failed to submit block")
)

type EventListener struct {
	ethclient          *ethclient.Client
	opEthClient        *ethclient.Client
	log                *logrus.Entry
	contractAddr       common.Address
	artifact           *framework.Artifact
	bundleEventAbi     abi.Event
	builderBidEventAbi abi.Event
}

func NewEventListener(log *logrus.Entry, contractAddress common.Address) (*EventListener, error) {
	ethClient, err := ethclient.Dial(SuaveNodeRpc)
	if err != nil {
		return nil, err
	}

	opEthClient, err := ethclient.Dial("http://localhost:9545")
	if err != nil {
		return nil, err
	}

	artifact, err := framework.ReadArtifact(ContractAbiJsonPath)
	if err != nil {
		return nil, errArtifactRead
	}
	bundleEventAbi := artifact.Abi.Events[NewBundleEventName]
	builderBidEventAbi := artifact.Abi.Events[NewBuilderBidEventName]

	return &EventListener{
		log:                log,
		ethclient:          ethClient,
		opEthClient:        opEthClient,
		contractAddr:       contractAddress,
		artifact:           artifact,
		bundleEventAbi:     bundleEventAbi,
		builderBidEventAbi: builderBidEventAbi,
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

	bundleEventSig := []byte("NewBundleEvent(bytes16,uint64,address[])")
	bundleEventSigHash := crypto.Keccak256Hash(bundleEventSig)
	builderBidEventSig := []byte("NewBuilderBidEvent(bytes16,uint64,address[],bytes)")
	builderBidEventSigHash := crypto.Keccak256Hash(builderBidEventSig)

	for {
		select {
		case err := <-sub.Err():
			el.log.Fatal("Subscription: ", err)
		case vLog := <-logs:
			if vLog.Topics[0] == bundleEventSigHash {
				el.log.Printf("Got bid event")
				bidEvent := &BundleEvent{}
				if err := bidEvent.Unpack(&vLog, el.bundleEventAbi); err != nil {
					el.log.Warn("Failed to unpack hint event: ", err)
					break
				}
				el.log.Printf("%+v\n", bidEvent)
				el.TriggerBlockBuild(bidEvent.BidId, bidEvent.decryptCond)
			} else if vLog.Topics[0] == builderBidEventSigHash {
				el.log.Printf("Got builder bid event")
				builderBidEvent := &BuilderBidEvent{}
				if err := builderBidEvent.Unpack(&vLog, el.builderBidEventAbi); err != nil {
					el.log.Warn("Failed to unpack hint event: ", err)
					break
				}
				el.log.Printf("%+v\n", builderBidEvent)
				el.SubmitBlock(builderBidEvent.BidId, "http://host.docker.internal:18550")
			} else {
				el.log.Warn("Unknown event: ", vLog.Topics[0].Hex())
			}
		}
	}
}

// Assumes Builder account is funded, see `BuilderAddr` in constants
func (el *EventListener) TriggerBlockBuild(bidId types.BidId, decryptCond uint64) error {
	fr := framework.New()
	builder := framework.NewPrivKeyFromHex(BuilderPrivKey)

	ctrct := fr.ContractAt(el.contractAddr, el.artifact.Abi)
	builderCtrct := ctrct.Ref(builder)

	el.log.Info("Trigger block build for block ", decryptCond, " with bid ", bidId)
	receipt := builderCtrct.SendTransaction(ContractBuildBlockMethod, []interface{}{decryptCond}, []byte("hello"))

	if receipt.Status == 0 {
		return errUnsuccessfulTx
	}

	return nil
}

func (el *EventListener) SubmitBlock(bidId types.BidId, url string) error {
	fr := framework.New()
	builder := framework.NewPrivKeyFromHex(BuilderPrivKey)

	ctrct := fr.ContractAt(el.contractAddr, el.artifact.Abi)
	builderCtrct := ctrct.Ref(builder)

	el.log.Info("Submit block ", url, " with bid ", bidId)
	receipt := builderCtrct.SendTransaction(ContractPostBlockMethod, []interface{}{bidId, url}, []byte("hello"))
	if receipt.Status == 0 {
		return errSubmitBlock
	}
	return nil
}

type BundleEvent struct {
	BidId       [16]byte
	decryptCond uint64
	peekers     []common.Address
}

func (h *BundleEvent) Unpack(log *types.Log, eventAbi abi.Event) error {
	unpacked, err := eventAbi.Inputs.Unpack(log.Data)
	if err != nil {
		return err
	}
	h.BidId = unpacked[0].([16]byte)
	h.decryptCond = unpacked[1].(uint64)
	h.peekers = unpacked[2].([]common.Address)
	return nil
}

type BuilderBidEvent struct {
	BidId       [16]byte
	decryptCond uint64
	peekers     []common.Address
	envelope    []byte
}

func (h *BuilderBidEvent) Unpack(log *types.Log, eventAbi abi.Event) error {
	unpacked, err := eventAbi.Inputs.Unpack(log.Data)
	if err != nil {
		return err
	}
	h.BidId = unpacked[0].([16]byte)
	h.decryptCond = unpacked[1].(uint64)
	h.peekers = unpacked[2].([]common.Address)
	h.envelope = unpacked[3].([]byte)
	return nil
}
