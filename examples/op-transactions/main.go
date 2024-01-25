package main

// NOTE: Run it with contract address passed as an env variable:
// CONTRACT_ADDR=0xd594760B2A36467ec7F0267382564772D7b0b73c go run .

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/suave/sdk"
	"github.com/flashbots/suapp-examples/framework"
	"github.com/sirupsen/logrus"
)

const (
	LogLevel = "debug"

	// GethNodeRpc RPC endpoint is the pointing to op-geth node
	GethNodeRpc = "http://localhost:8545"

	// PrivateKeyHex [OP chain] of the 1st of prefunded accounts from pbs-on-optimism, address 0x1023e8DbDebAd480C43f6e19b3381c465c74E933
	PrivateKeyHex = "f513f6c8a938fb611fa6589400fba7f1d00cad4ec0972b5bfe38657f26d1b0fc"
	// BuilderPrivKey [Suave chain] Builder address "0xDceef22333b11aD2CAb54Be2A8ECe08EE64D919C" needs to be funded
	BuilderPrivKey = "9b6fa7074578db9ce7752ac85bf5c0acd071c7115f8fc02abdd435918edd4b62"

	ContractAddrEnv     = "CONTRACT_ADDR"
	ContractAbiJsonPath = "optimism-builder.sol/OpBuilder.json"
	ContractNewTxMethod = "newTx"
)

var (
	BundlePushIntervalSeconds = time.Duration(5 * time.Second)

	errMissingContractAddr = errors.New("missing contract address to listen events for")
	errArtifactRead        = errors.New("failed to read artifact from " + ContractAbiJsonPath)
	errUnsuccessfulTx      = errors.New("unsuccessful transaction to " + ContractNewTxMethod)
	errOpGethConnection    = errors.New("failed to connect to op-geth node")
	errNonceFetch          = errors.New("failed getting nonce from op-geth node")
)

type AccountTransfer struct {
	Address common.Address
	key     *framework.PrivKey
	client  *sdk.Client
	log     *logrus.Entry

	// Tx creation params
	txInBundle       uint8
	amount, gasPrice *big.Int
	nonce, gasLimit  uint64
}

func main() {
	log := logrus.NewEntry(logrus.New())
	log.Logger.SetOutput(os.Stdout)

	lvl, err := logrus.ParseLevel(LogLevel)
	if err != nil {
		flag.Usage()
		log.Fatalf("invalid loglevel: %s", LogLevel)
	}
	log.Logger.SetLevel(lvl)

	at, err := NewAccountTransfer(log, GethNodeRpc, PrivateKeyHex)
	if err != nil {
		log.Fatal("failed creating the account transfer")
	}
	bb, err := NewBuilderRef(log)
	if err != nil {
		log.Fatal("failed creating the builder reference")
	}

	index := 1
	for {
		bundle, err := at.createBundle()
		if err != nil {
			log.WithError(err).Fatal("could not create bundle")
		}

		bundleBytes, err := json.Marshal(bundle)
		if err != nil {
			log.WithError(err).Fatal("could not marshal a bundle")
		}

		log.WithField("index", index).Info("Bundle created")
		bb.SendBundle(index, bundleBytes)

		time.Sleep(BundlePushIntervalSeconds)
		index++
	}
}

func NewAccountTransfer(log *logrus.Entry, rpcUrl, key string) (*AccountTransfer, error) {
	rpcClient, err := rpc.Dial(rpcUrl)
	if err != nil {
		log.WithError(err).Error("failed to connect to op-geth node")
		return nil, errOpGethConnection
	}

	privKey := framework.NewPrivKeyFromHex(key)
	addr := crypto.PubkeyToAddress(privKey.Priv.PublicKey)
	sdkClient := sdk.NewClient(rpcClient, privKey.Priv, common.Address{})

	rpc := sdkClient.RPC()
	nonce, err := rpc.PendingNonceAt(context.Background(), addr)
	if err != nil {
		log.WithError(err).Error("failed getting nonce")
		return nil, errNonceFetch
	}
	log.WithField("nonce", nonce).WithField("address", addr.Hex()).Info("Initialized")

	return &AccountTransfer{
		Address: addr,
		key:     privKey,
		client:  sdkClient,
		log:     log,

		nonce: nonce,

		// some artificial numbers to start with
		txInBundle: 10,

		amount:   big.NewInt(100000000000), // 100 Gwei
		gasPrice: big.NewInt(100000000000),
		gasLimit: 21000,
	}, nil
}

func (at *AccountTransfer) createTx() (*types.Transaction, error) {
	txn := &types.LegacyTx{
		To:       &at.Address,
		Value:    at.amount,
		GasPrice: at.gasPrice,
		Gas:      at.gasLimit,
		Nonce:    at.nonce,
	}

	tx, err := at.client.SignTxn(txn)
	if err != nil {
		at.log.WithError(err).Error("failed to sign transaction")
		return nil, err
	}

	// remember to increment nonce
	at.nonce++

	return tx, nil
}

func (at *AccountTransfer) createBundle() (*types.SBundle, error) {
	// TODO: Is it the exact bundle type we have in op-builder?
	bundle := &types.SBundle{
		Txs:             make([]*types.Transaction, at.txInBundle),
		RevertingHashes: []common.Hash{},
	}

	for i := uint8(0); i < at.txInBundle; i++ {
		tx, err := at.createTx()
		if err != nil {
			return nil, err
		}
		bundle.Txs[i] = tx
	}

	return bundle, nil
}

type BuilderRef struct {
	log          *logrus.Entry
	contractAddr common.Address
	artifact     *framework.Artifact
}

func NewBuilderRef(log *logrus.Entry) (*BuilderRef, error) {
	artifact, err := framework.ReadArtifact(ContractAbiJsonPath)
	if err != nil {
		return nil, errArtifactRead
	}

	addrHex := os.Getenv(ContractAddrEnv)
	if addrHex == "" {
		return nil, errMissingContractAddr
	}
	contractAddr := common.HexToAddress(addrHex)

	return &BuilderRef{
		log:          log,
		contractAddr: contractAddr,
		artifact:     artifact,
	}, nil
}

func (bb *BuilderRef) SendBundle(blkHeight int, bundle []byte) error {
	fr := framework.New()

	builder := framework.NewPrivKeyFromHex(BuilderPrivKey)

	ctrct := fr.ContractAt(bb.contractAddr, bb.artifact.Abi)
	builderCtrct := ctrct.Ref(builder)

	bb.log.WithField("bundle", blkHeight).Info("Sending bundle")
	receipt := builderCtrct.SendTransaction(ContractNewTxMethod, []interface{}{uint64(blkHeight)}, bundle)
	// TODO: I've got the following error: `failed to send transaction err argument count mismatch: got 1 for 3`
	if receipt.Status == 0 {
		return errUnsuccessfulTx
	}

	return nil
}
